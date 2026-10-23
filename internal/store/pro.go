package store

import (
	"database/sql"
	"strings"
)

// The "Pro" section: payments received from clients for a freelance activity, split by the user's own
// categories (tax, contributions…). It is independent from accounts and transactions.

const proSchema = `
CREATE TABLE IF NOT EXISTS pro_clients (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  archived INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS pro_categories (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  archived INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS pro_payments (
  id INTEGER PRIMARY KEY,
  date TEXT NOT NULL,
  client_id INTEGER NOT NULL REFERENCES pro_clients(id),
  gross_cents INTEGER NOT NULL CHECK (gross_cents > 0),
  note TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS pro_lines (
  id INTEGER PRIMARY KEY,
  payment_id INTEGER NOT NULL REFERENCES pro_payments(id) ON DELETE CASCADE,
  category_id INTEGER NOT NULL REFERENCES pro_categories(id),
  amount_cents INTEGER NOT NULL CHECK (amount_cents > 0)
);
CREATE INDEX IF NOT EXISTS pro_payment_date ON pro_payments(date);
CREATE INDEX IF NOT EXISTS pro_line_payment ON pro_lines(payment_id);
`

// ProName is a client or a category: archived ones are hidden from the pickers but stay in old payments.
type ProName struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}

type ProLine struct {
	CategoryID  int64  `json:"category_id"`
	Name        string `json:"name"`
	AmountCents int64  `json:"amount_cents"`
}

type ProPayment struct {
	ID         int64     `json:"id"`
	Date       string    `json:"date"`
	ClientID   int64     `json:"client_id"`
	ClientName string    `json:"client_name"`
	GrossCents int64     `json:"gross_cents"`
	Note       string    `json:"note"`
	Lines      []ProLine `json:"lines"`
	NetCents   int64     `json:"net_cents"`
}

// proTable maps the two kinds of names to their table; the value is never taken from user input.
func proTable(kind string) string {
	if kind == "clients" {
		return "pro_clients"
	}
	return "pro_categories"
}

func (s *Store) ListProNames(kind string) ([]ProName, error) {
	rows, err := s.db.Query("SELECT id, name, archived FROM " + proTable(kind) + " WHERE archived = 0 ORDER BY name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProName{}
	for rows.Next() {
		var n ProName
		if err := rows.Scan(&n.ID, &n.Name, &n.Archived); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) CreateProName(kind string, n *ProName) error {
	n.Name = strings.TrimSpace(n.Name)
	if n.Name == "" {
		return invalid("name is required")
	}
	res, err := s.db.Exec("INSERT INTO "+proTable(kind)+"(name) VALUES(?)", n.Name)
	if err != nil {
		return err
	}
	n.ID, _ = res.LastInsertId()
	n.Archived = false
	return nil
}

func (s *Store) UpdateProName(kind string, n *ProName) error {
	n.Name = strings.TrimSpace(n.Name)
	if n.Name == "" {
		return invalid("name is required")
	}
	res, err := s.db.Exec("UPDATE "+proTable(kind)+" SET name=? WHERE id=? AND archived=0", n.Name, n.ID)
	if err != nil {
		return err
	}
	if c, _ := res.RowsAffected(); c == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteProName removes a client or category. If payments still refer to it, it is only archived,
// so the old payments keep its name and their totals.
func (s *Store) DeleteProName(kind string, id int64) error {
	table := proTable(kind)
	var used int
	q := "SELECT COUNT(*) FROM pro_payments WHERE client_id=?"
	if kind != "clients" {
		q = "SELECT COUNT(*) FROM pro_lines WHERE category_id=?"
	}
	if err := s.db.QueryRow(q, id).Scan(&used); err != nil {
		return err
	}
	var res sql.Result
	var err error
	if used > 0 {
		res, err = s.db.Exec("UPDATE "+table+" SET archived=1 WHERE id=? AND archived=0", id)
	} else {
		res, err = s.db.Exec("DELETE FROM "+table+" WHERE id=? AND archived=0", id)
	}
	if err != nil {
		return err
	}
	if c, _ := res.RowsAffected(); c == 0 {
		return ErrNotFound
	}
	return nil
}

// ListProPayments returns the payments with date in [from, to] (either may be empty), newest first.
func (s *Store) ListProPayments(from, to string) ([]ProPayment, error) {
	q := `SELECT p.id, p.date, p.client_id, c.name, p.gross_cents, p.note
	  FROM pro_payments p JOIN pro_clients c ON c.id = p.client_id WHERE 1=1`
	var args []any
	if from != "" {
		q += " AND p.date >= ?"
		args = append(args, from)
	}
	if to != "" {
		q += " AND p.date <= ?"
		args = append(args, to)
	}
	rows, err := s.db.Query(q+" ORDER BY p.date DESC, p.id DESC", args...)
	if err != nil {
		return nil, err
	}
	out := []ProPayment{}
	byID := map[int64]int{}
	for rows.Next() {
		var p ProPayment
		if err := rows.Scan(&p.ID, &p.Date, &p.ClientID, &p.ClientName, &p.GrossCents, &p.Note); err != nil {
			rows.Close()
			return nil, err
		}
		p.Lines = []ProLine{}
		p.NetCents = p.GrossCents
		byID[p.ID] = len(out)
		out = append(out, p)
	}
	rows.Close()
	if len(out) == 0 {
		return out, nil
	}

	lq := `SELECT l.payment_id, l.category_id, c.name, l.amount_cents
	  FROM pro_lines l JOIN pro_categories c ON c.id = l.category_id
	  JOIN pro_payments p ON p.id = l.payment_id WHERE 1=1`
	if from != "" {
		lq += " AND p.date >= ?"
	}
	if to != "" {
		lq += " AND p.date <= ?"
	}
	lrows, err := s.db.Query(lq+" ORDER BY l.id", args...)
	if err != nil {
		return nil, err
	}
	defer lrows.Close()
	for lrows.Next() {
		var pid int64
		var l ProLine
		if err := lrows.Scan(&pid, &l.CategoryID, &l.Name, &l.AmountCents); err != nil {
			return nil, err
		}
		if i, ok := byID[pid]; ok {
			out[i].Lines = append(out[i].Lines, l)
			out[i].NetCents -= l.AmountCents
		}
	}
	return out, lrows.Err()
}

func (s *Store) GetProPayment(id int64) (*ProPayment, error) {
	var date string
	if err := s.db.QueryRow("SELECT date FROM pro_payments WHERE id=?", id).Scan(&date); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	list, err := s.ListProPayments(date, date)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, ErrNotFound
}

// proCheck validates a payment. A client or category already in this payment may be archived (editing an old
// payment); a new one has to be active.
func (s *Store) proCheck(tx *sql.Tx, p *ProPayment, existing int64) error {
	if _, err := parseDate(p.Date); err != nil {
		return err
	}
	if p.GrossCents <= 0 {
		return invalid("amount must be greater than zero")
	}
	p.Note = strings.TrimSpace(p.Note)
	var ok int
	err := tx.QueryRow(`SELECT COUNT(*) FROM pro_clients WHERE id=? AND (archived=0 OR id=(SELECT client_id FROM pro_payments WHERE id=?))`,
		p.ClientID, existing).Scan(&ok)
	if err != nil {
		return err
	}
	if ok == 0 {
		return invalid("choose a client")
	}
	seen := map[int64]bool{}
	for _, l := range p.Lines {
		if l.AmountCents <= 0 {
			return invalid("amount must be greater than zero")
		}
		if seen[l.CategoryID] {
			return invalid("a category is used twice in the same payment")
		}
		seen[l.CategoryID] = true
		err := tx.QueryRow(`SELECT COUNT(*) FROM pro_categories WHERE id=? AND (archived=0 OR id IN (SELECT category_id FROM pro_lines WHERE payment_id=?))`,
			l.CategoryID, existing).Scan(&ok)
		if err != nil {
			return err
		}
		if ok == 0 {
			return invalid("category does not exist")
		}
	}
	return nil
}

func (s *Store) writeProPayment(p *ProPayment, update bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	existing := int64(0)
	if update {
		existing = p.ID
	}
	if err := s.proCheck(tx, p, existing); err != nil {
		return err
	}
	if update {
		res, err := tx.Exec("UPDATE pro_payments SET date=?, client_id=?, gross_cents=?, note=? WHERE id=?", p.Date, p.ClientID, p.GrossCents, p.Note, p.ID)
		if err != nil {
			return err
		}
		if c, _ := res.RowsAffected(); c == 0 {
			return ErrNotFound
		}
		if _, err := tx.Exec("DELETE FROM pro_lines WHERE payment_id=?", p.ID); err != nil {
			return err
		}
	} else {
		res, err := tx.Exec("INSERT INTO pro_payments(date, client_id, gross_cents, note) VALUES(?,?,?,?)", p.Date, p.ClientID, p.GrossCents, p.Note)
		if err != nil {
			return err
		}
		p.ID, _ = res.LastInsertId()
	}
	for _, l := range p.Lines {
		if _, err := tx.Exec("INSERT INTO pro_lines(payment_id, category_id, amount_cents) VALUES(?,?,?)", p.ID, l.CategoryID, l.AmountCents); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) CreateProPayment(p *ProPayment) error { return s.writeProPayment(p, false) }
func (s *Store) UpdateProPayment(p *ProPayment) error { return s.writeProPayment(p, true) }

func (s *Store) DeleteProPayment(id int64) error {
	res, err := s.db.Exec("DELETE FROM pro_payments WHERE id=?", id)
	if err != nil {
		return err
	}
	if c, _ := res.RowsAffected(); c == 0 {
		return ErrNotFound
	}
	return nil
}
