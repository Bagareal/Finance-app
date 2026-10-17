package store

import "strings"

type Tx struct {
	ID          int64  `json:"id"`
	Date        string `json:"date"`
	Kind        string `json:"kind"` // income | expense | transfer
	AmountCents int64  `json:"amount_cents"`
	AccountID   int64  `json:"account_id"`
	ToAccountID *int64 `json:"to_account_id"`
	CategoryID  *int64 `json:"category_id"`
	Note        string `json:"note"`
	RecurringID *int64 `json:"recurring_id"`
}

// validateFlow controlla coerenza di tipo, importo, conti e categoria (usa s.db: da chiamare fuori da transazioni).
func (s *Store) validateFlow(kind string, amount, account int64, to, category **int64) error {
	if amount <= 0 {
		return invalid("l'importo deve essere maggiore di zero")
	}
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM accounts WHERE id=?", account).Scan(&n); err != nil || n == 0 {
		return invalid("conto inesistente")
	}
	switch kind {
	case "transfer":
		if *to == nil {
			return invalid("scegli il conto di destinazione")
		}
		if **to == account {
			return invalid("il conto di destinazione deve essere diverso da quello di origine")
		}
		if err := s.db.QueryRow("SELECT COUNT(*) FROM accounts WHERE id=?", **to).Scan(&n); err != nil || n == 0 {
			return invalid("conto di destinazione inesistente")
		}
		*category = nil // un trasferimento non ha categoria
	case "income", "expense":
		*to = nil
		if *category != nil {
			c, err := s.getCategory(**category)
			if err != nil {
				return invalid("categoria inesistente")
			}
			if c.Kind != kind {
				return invalid("la categoria non è del tipo giusto (entrata/uscita)")
			}
		}
	default:
		return invalid("tipo di movimento non valido")
	}
	return nil
}

func (s *Store) validateTx(t *Tx) error {
	if _, err := parseDate(t.Date); err != nil {
		return err
	}
	t.Note = strings.TrimSpace(t.Note)
	return s.validateFlow(t.Kind, t.AmountCents, t.AccountID, &t.ToAccountID, &t.CategoryID)
}

func (s *Store) CreateTx(t *Tx) error {
	if err := s.validateTx(t); err != nil {
		return err
	}
	res, err := s.db.Exec(`INSERT INTO transactions(date,kind,amount_cents,account_id,to_account_id,category_id,note,recurring_id)
	  VALUES(?,?,?,?,?,?,?,?)`, t.Date, t.Kind, t.AmountCents, t.AccountID, t.ToAccountID, t.CategoryID, t.Note, t.RecurringID)
	if err != nil {
		return err
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) UpdateTx(t *Tx) error {
	if err := s.validateTx(t); err != nil {
		return err
	}
	res, err := s.db.Exec(`UPDATE transactions SET date=?,kind=?,amount_cents=?,account_id=?,to_account_id=?,category_id=?,note=? WHERE id=?`,
		t.Date, t.Kind, t.AmountCents, t.AccountID, t.ToAccountID, t.CategoryID, t.Note, t.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTx(id int64) error {
	res, err := s.db.Exec("DELETE FROM transactions WHERE id=?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

type TxFilter struct {
	From, To   string
	AccountID  int64
	CategoryID int64 // include le sottocategorie
	Kind       string
	Limit      int
	Offset     int
}

func (s *Store) ListTx(f TxFilter) ([]Tx, error) {
	q := `SELECT id,date,kind,amount_cents,account_id,to_account_id,category_id,note,recurring_id FROM transactions WHERE 1=1`
	var args []any
	if f.From != "" {
		q += " AND date >= ?"
		args = append(args, f.From)
	}
	if f.To != "" {
		q += " AND date <= ?"
		args = append(args, f.To)
	}
	if f.AccountID > 0 {
		q += " AND (account_id = ? OR to_account_id = ?)"
		args = append(args, f.AccountID, f.AccountID)
	}
	if f.CategoryID > 0 {
		q += " AND category_id IN (SELECT id FROM categories WHERE id = ? OR parent_id = ?)"
		args = append(args, f.CategoryID, f.CategoryID)
	}
	if f.Kind != "" {
		q += " AND kind = ?"
		args = append(args, f.Kind)
	}
	q += " ORDER BY date DESC, id DESC"
	if f.Limit > 0 {
		q += " LIMIT ? OFFSET ?"
		args = append(args, f.Limit, f.Offset)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tx{}
	for rows.Next() {
		var t Tx
		if err := rows.Scan(&t.ID, &t.Date, &t.Kind, &t.AmountCents, &t.AccountID, &t.ToAccountID, &t.CategoryID, &t.Note, &t.RecurringID); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
