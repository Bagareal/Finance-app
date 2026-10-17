package store

import "strings"

type Account struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	InitialCents int64  `json:"initial_cents"`
	Archived     bool   `json:"archived"`
	BalanceCents int64  `json:"balance_cents"`
}

var accountTypes = map[string]bool{"corrente": true, "contanti": true, "risparmio": true, "carta": true, "investimenti": true}

func (a *Account) normalize() error {
	a.Name = strings.TrimSpace(a.Name)
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

const accountSelect = `
SELECT a.id, a.name, a.type, a.initial_cents, a.archived,
  a.initial_cents
  + COALESCE((SELECT SUM(CASE kind WHEN 'income' THEN amount_cents ELSE -amount_cents END)
              FROM transactions WHERE account_id = a.id), 0)
  + COALESCE((SELECT SUM(amount_cents) FROM transactions
              WHERE kind = 'transfer' AND to_account_id = a.id), 0)
FROM accounts a`

func scanAccounts(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]Account, error) {
	out := []Account{}
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.InitialCents, &a.Archived, &a.BalanceCents); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) ListAccounts() ([]Account, error) {
	rows, err := s.db.Query(accountSelect + " ORDER BY a.archived, a.name COLLATE NOCASE")
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
	res, err := s.db.Exec("INSERT INTO accounts(name,type,initial_cents,archived) VALUES(?,?,?,?)", a.Name, a.Type, a.InitialCents, a.Archived)
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
	res, err := s.db.Exec("UPDATE accounts SET name=?, type=?, initial_cents=?, archived=? WHERE id=?", a.Name, a.Type, a.InitialCents, a.Archived, a.ID)
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
