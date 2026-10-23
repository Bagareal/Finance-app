package store

import "strings"

type Account struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	InitialCents int64  `json:"initial_cents"`
	Archived     bool   `json:"archived"`
	Group        string `json:"group"`  // accounts sharing a group name are shown together under it
	Manual       bool   `json:"manual"` // the balance is a value updated by hand (investments); transfers after it still count
	ValueDate    string `json:"value_date"`
	BalanceCents int64  `json:"balance_cents"`
}

var accountTypes = map[string]bool{"corrente": true, "contanti": true, "risparmio": true, "carta": true, "investimenti": true}

func (a *Account) normalize() error {
	a.Name = strings.TrimSpace(a.Name)
	a.Group = strings.TrimSpace(a.Group)
	if a.Name == "" {
		return invalid("account name is required")
	}
	if a.Type == "" {
		a.Type = "corrente"
	}
	if !accountTypes[a.Type] {
		return invalid("invalid account type")
	}
	return nil
}

// For an account with manual values the balance starts from the latest value and only counts the
// movements dated after it; the others start from the opening balance and count everything.
const accountSelect = `
SELECT a.id, a.name, a.type, a.initial_cents, a.archived, a.group_name, a.manual, COALESCE(lv.date, ''),
  CASE WHEN a.manual = 1 AND lv.date IS NOT NULL THEN lv.value_cents ELSE a.initial_cents END
  + COALESCE((SELECT SUM(CASE kind WHEN 'income' THEN amount_cents ELSE -amount_cents END)
              FROM transactions WHERE account_id = a.id AND (a.manual = 0 OR lv.date IS NULL OR date > lv.date)), 0)
  + COALESCE((SELECT SUM(amount_cents) FROM transactions
              WHERE kind = 'transfer' AND to_account_id = a.id AND (a.manual = 0 OR lv.date IS NULL OR date > lv.date)), 0)
FROM accounts a
LEFT JOIN account_values lv ON lv.account_id = a.id
  AND lv.date = (SELECT MAX(date) FROM account_values WHERE account_id = a.id)`

func scanAccounts(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]Account, error) {
	out := []Account{}
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.InitialCents, &a.Archived, &a.Group, &a.Manual, &a.ValueDate, &a.BalanceCents); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DisplayName is the name shown in lists and pickers: "Group · Name" for a grouped account.
func (a Account) DisplayName() string {
	if a.Group == "" {
		return a.Name
	}
	return a.Group + " · " + a.Name
}

func (s *Store) ListAccounts() ([]Account, error) {
	rows, err := s.db.Query(accountSelect + " ORDER BY a.archived, COALESCE(NULLIF(a.group_name, ''), a.name) COLLATE NOCASE, a.name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAccounts(rows)
}

func (s *Store) GetAccount(id int64) (*Account, error) {
	rows, err := s.db.Query(accountSelect+" WHERE a.id = ?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanAccounts(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

func (s *Store) CreateAccount(a *Account) error {
	if err := a.normalize(); err != nil {
		return err
	}
	res, err := s.db.Exec("INSERT INTO accounts(name,type,initial_cents,archived,group_name,manual) VALUES(?,?,?,?,?,?)",
		a.Name, a.Type, a.InitialCents, a.Archived, a.Group, a.Manual)
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) UpdateAccount(a *Account) error {
	if err := a.normalize(); err != nil {
		return err
	}
	res, err := s.db.Exec("UPDATE accounts SET name=?, type=?, initial_cents=?, archived=?, group_name=?, manual=? WHERE id=?",
		a.Name, a.Type, a.InitialCents, a.Archived, a.Group, a.Manual, a.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAccount elimina un conto senza movimenti né ricorrenze; altrimenti suggerisce l'archiviazione.
func (s *Store) DeleteAccount(id int64) error {
	var n int
	s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM transactions WHERE account_id=?1 OR to_account_id=?1)
	  + (SELECT COUNT(*) FROM recurring WHERE account_id=?1 OR to_account_id=?1)`, id).Scan(&n)
	if n > 0 {
		return invalid("the account has transactions or recurring items: archive it instead of deleting it")
	}
	res, err := s.db.Exec("DELETE FROM accounts WHERE id=?", id)
	if err != nil {
		return err
	}
	if c, _ := res.RowsAffected(); c == 0 {
		return ErrNotFound
	}
	return nil
}

// --- values of accounts updated by hand ---

type AccountValue struct {
	ID          int64  `json:"id"`
	Date        string `json:"date"`
	ValueCents  int64  `json:"value_cents"`
	ChangeCents int64  `json:"change_cents"` // gain or loss since the previous value, after the transfers made in between
}

// netFlow is the effect of the movements dated in (after, upTo] on the account balance.
func (s *Store) netFlow(accountID int64, after, upTo string) (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT
	  COALESCE((SELECT SUM(CASE kind WHEN 'income' THEN amount_cents ELSE -amount_cents END)
	            FROM transactions WHERE account_id = ?1 AND date > ?2 AND date <= ?3), 0)
	  + COALESCE((SELECT SUM(amount_cents) FROM transactions
	            WHERE kind = 'transfer' AND to_account_id = ?1 AND date > ?2 AND date <= ?3), 0)`, accountID, after, upTo).Scan(&n)
	return n, err
}

// ListValues returns the history of an account's values, newest first, with the gain or loss of each step.
func (s *Store) ListValues(accountID int64) ([]AccountValue, error) {
	acc, err := s.GetAccount(accountID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query("SELECT id, date, value_cents FROM account_values WHERE account_id = ? ORDER BY date", accountID)
	if err != nil {
		return nil, err
	}
	var asc []AccountValue
	for rows.Next() {
		var v AccountValue
		if err := rows.Scan(&v.ID, &v.Date, &v.ValueCents); err != nil {
			rows.Close()
			return nil, err
		}
		asc = append(asc, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	prev, prevDate := acc.InitialCents, ""
	for i := range asc {
		flow, err := s.netFlow(accountID, prevDate, asc[i].Date)
		if err != nil {
			return nil, err
		}
		asc[i].ChangeCents = asc[i].ValueCents - (prev + flow)
		prev, prevDate = asc[i].ValueCents, asc[i].Date
	}
	out := make([]AccountValue, 0, len(asc))
	for i := len(asc) - 1; i >= 0; i-- {
		out = append(out, asc[i])
	}
	return out, nil
}

// SetValue records the value of an account on a date; a second value on the same date replaces the first.
func (s *Store) SetValue(accountID int64, date string, cents int64) error {
	if _, err := parseDate(date); err != nil {
		return err
	}
	acc, err := s.GetAccount(accountID)
	if err != nil {
		return err
	}
	if !acc.Manual {
		return invalid("this account does not use manual values")
	}
	_, err = s.db.Exec(`INSERT INTO account_values(account_id,date,value_cents) VALUES(?,?,?)
	  ON CONFLICT(account_id,date) DO UPDATE SET value_cents=excluded.value_cents`, accountID, date, cents)
	return err
}

func (s *Store) DeleteValue(id int64) error {
	res, err := s.db.Exec("DELETE FROM account_values WHERE id=?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
