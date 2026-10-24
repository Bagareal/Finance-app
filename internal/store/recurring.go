package store

import (
	"strings"
	"time"
)

type Recurring struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	AmountCents int64  `json:"amount_cents"`
	AccountID   int64  `json:"account_id"`
	ToAccountID *int64 `json:"to_account_id"`
	CategoryID  *int64 `json:"category_id"`
	Note        string `json:"note"`
	Freq        string `json:"freq"` // weekly | monthly | yearly
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"` // vuoto = nessuna fine
	NextDate    string `json:"next_date"`
	Active      bool   `json:"active"`
}

func (s *Store) validateRecurring(r *Recurring) error {
	if r.Freq != "weekly" && r.Freq != "monthly" && r.Freq != "yearly" {
		return invalid("invalid frequency")
	}
	start, err := parseDate(r.StartDate)
	if err != nil {
		return err
	}
	if r.EndDate != "" {
		end, err := parseDate(r.EndDate)
		if err != nil {
			return err
		}
		if end.Before(start) {
			return invalid("the end date is before the start date")
		}
	}
	r.Note = strings.TrimSpace(r.Note)
	return s.validateFlow(r.Kind, r.AmountCents, r.AccountID, &r.ToAccountID, &r.CategoryID)
}

func (s *Store) ListRecurring() ([]Recurring, error) {
	rows, err := s.db.Query(`SELECT id,kind,amount_cents,account_id,to_account_id,category_id,note,freq,start_date,end_date,next_date,active
	  FROM recurring ORDER BY active DESC, next_date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Recurring{}
	for rows.Next() {
		var r Recurring
		if err := rows.Scan(&r.ID, &r.Kind, &r.AmountCents, &r.AccountID, &r.ToAccountID, &r.CategoryID, &r.Note, &r.Freq, &r.StartDate, &r.EndDate, &r.NextDate, &r.Active); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateRecurring(r *Recurring) error {
	if err := s.validateRecurring(r); err != nil {
		return err
	}
	r.NextDate = r.StartDate
	res, err := s.db.Exec(`INSERT INTO recurring(kind,amount_cents,account_id,to_account_id,category_id,note,freq,start_date,end_date,next_date,active)
	  VALUES(?,?,?,?,?,?,?,?,?,?,?)`, r.Kind, r.AmountCents, r.AccountID, r.ToAccountID, r.CategoryID, r.Note, r.Freq, r.StartDate, r.EndDate, r.NextDate, r.Active)
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) UpdateRecurring(r *Recurring) error {
	if err := s.validateRecurring(r); err != nil {
		return err
	}
	var next string
	if err := s.db.QueryRow("SELECT next_date FROM recurring WHERE id=?", r.ID).Scan(&next); err != nil {
		return ErrNotFound
	}
	// non rigenera il passato già generato: next_date avanza solo se l'inizio viene spostato in avanti
	if r.StartDate > next {
		next = r.StartDate
	}
	r.NextDate = next
	_, err := s.db.Exec(`UPDATE recurring SET kind=?,amount_cents=?,account_id=?,to_account_id=?,category_id=?,note=?,freq=?,start_date=?,end_date=?,next_date=?,active=? WHERE id=?`,
		r.Kind, r.AmountCents, r.AccountID, r.ToAccountID, r.CategoryID, r.Note, r.Freq, r.StartDate, r.EndDate, r.NextDate, r.Active, r.ID)
	return err
}

// UpdateRecurringHistory aggiorna la ricorrenza e allinea ai nuovi valori anche i movimenti già generati
// (tipo, importo, conti, categoria e nota; le date restano quelle di ogni movimento).
func (s *Store) UpdateRecurringHistory(r *Recurring) error {
	if err := s.UpdateRecurring(r); err != nil {
		return err
	}
	_, err := s.db.Exec(`UPDATE transactions SET kind=?,amount_cents=?,account_id=?,to_account_id=?,category_id=?,note=? WHERE recurring_id=?`,
		r.Kind, r.AmountCents, r.AccountID, r.ToAccountID, r.CategoryID, r.Note, r.ID)
	return err
}

// DeleteRecurringHistory elimina la ricorrenza insieme ai movimenti già generati.
func (s *Store) DeleteRecurringHistory(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM transactions WHERE recurring_id=?", id); err != nil {
		return err
	}
	res, err := tx.Exec("DELETE FROM recurring WHERE id=?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// DeleteRecurring elimina la ricorrenza; i movimenti già generati restano, ma senza più il legame con essa
// (altrimenti una nuova ricorrenza che riusa lo stesso id li adotterebbe).
func (s *Store) DeleteRecurring(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec("DELETE FROM recurring WHERE id=?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec("UPDATE transactions SET recurring_id=NULL WHERE recurring_id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

func daysIn(y int, m time.Month) int { return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day() }

// addMonths somma n mesi a t mantenendo il giorno anchorDay (ridotto a fine mese se serve).
func addMonths(t time.Time, n, anchorDay int) time.Time {
	total := int(t.Month()) - 1 + n
	y := t.Year() + total/12
	m := time.Month(total%12 + 1)
	d := anchorDay
	if dim := daysIn(y, m); d > dim {
		d = dim
	}
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// NextOccurrence calcola l'occorrenza successiva a cur per una ricorrenza iniziata in start.
func NextOccurrence(freq string, start, cur time.Time) time.Time {
	switch freq {
	case "weekly":
		return cur.AddDate(0, 0, 7)
	case "yearly":
		return addMonths(cur, 12, start.Day())
	default:
		return addMonths(cur, 1, start.Day())
	}
}

// RunRecurring genera i movimenti dovuti fino a asOf (AAAA-MM-GG, vuoto = oggi), compresi gli arretrati.
// Restituisce quanti movimenti ha creato.
func (s *Store) RunRecurring(asOf string) (int, error) {
	if asOf == "" {
		asOf = today()
	}
	rows, err := s.db.Query(`SELECT id,kind,amount_cents,account_id,to_account_id,category_id,note,freq,start_date,end_date,next_date
	  FROM recurring WHERE active=1 AND next_date <= ?`, asOf)
	if err != nil {
		return 0, err
	}
	var due []Recurring
	for rows.Next() {
		var r Recurring
		if err := rows.Scan(&r.ID, &r.Kind, &r.AmountCents, &r.AccountID, &r.ToAccountID, &r.CategoryID, &r.Note, &r.Freq, &r.StartDate, &r.EndDate, &r.NextDate); err != nil {
			rows.Close()
			return 0, err
		}
		due = append(due, r)
	}
	rows.Close()

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	created := 0
	for _, r := range due {
		start, _ := time.Parse("2006-01-02", r.StartDate)
		cur, err := time.Parse("2006-01-02", r.NextDate)
		if err != nil {
			continue
		}
		active := true
		for i := 0; i < 2000 && cur.Format("2006-01-02") <= asOf; i++ {
			d := cur.Format("2006-01-02")
			if r.EndDate != "" && d > r.EndDate {
				active = false
				break
			}
			if _, err := tx.Exec(`INSERT INTO transactions(date,kind,amount_cents,account_id,to_account_id,category_id,note,recurring_id)
			  VALUES(?,?,?,?,?,?,?,?)`, d, r.Kind, r.AmountCents, r.AccountID, r.ToAccountID, r.CategoryID, r.Note, r.ID); err != nil {
				return 0, err
			}
			created++
			cur = NextOccurrence(r.Freq, start, cur)
		}
		next := cur.Format("2006-01-02")
		if r.EndDate != "" && next > r.EndDate {
			active = false
		}
		if _, err := tx.Exec("UPDATE recurring SET next_date=?, active=? WHERE id=?", next, active, r.ID); err != nil {
			return 0, err
		}
	}
	return created, tx.Commit()
}
