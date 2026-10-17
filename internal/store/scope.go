package store

import (
	"database/sql"
	"time"
)

// Ambito di una modifica o eliminazione su un movimento generato da una ricorrenza.
const (
	ScopeOne       = "one"       // solo questo movimento
	ScopeFollowing = "following" // questo e i successivi
	ScopeAll       = "all"       // tutti i movimenti della ricorrenza
)

func validScope(scope string) (string, error) {
	switch scope {
	case "", ScopeOne:
		return ScopeOne, nil
	case ScopeFollowing, ScopeAll:
		return scope, nil
	}
	return "", invalid("invalid scope")
}

// txOrigin restituisce data e ricorrenza (0 = nessuna) del movimento.
func (s *Store) txOrigin(id int64) (date string, recurringID int64, err error) {
	var rid sql.NullInt64
	err = s.db.QueryRow("SELECT date, recurring_id FROM transactions WHERE id=?", id).Scan(&date, &rid)
	if err == sql.ErrNoRows {
		return "", 0, ErrNotFound
	}
	return date, rid.Int64, err
}

// UpdateTxScope modifica un movimento. Per i movimenti nati da una ricorrenza, con ambito "following" o "all"
// propaga tipo, importo, conti, categoria e nota anche agli altri movimenti e alla ricorrenza stessa
// (così quelli futuri sono coerenti). La data cambia sempre solo per il movimento modificato.
func (s *Store) UpdateTxScope(t *Tx, scope string) error {
	scope, err := validScope(scope)
	if err != nil {
		return err
	}
	date, rid, err := s.txOrigin(t.ID)
	if err != nil {
		return err
	}
	if scope == ScopeOne || rid == 0 {
		return s.UpdateTx(t)
	}
	if err := s.validateTx(t); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE transactions SET date=?,kind=?,amount_cents=?,account_id=?,to_account_id=?,category_id=?,note=? WHERE id=?`,
		t.Date, t.Kind, t.AmountCents, t.AccountID, t.ToAccountID, t.CategoryID, t.Note, t.ID); err != nil {
		return err
	}
	q := `UPDATE transactions SET kind=?,amount_cents=?,account_id=?,to_account_id=?,category_id=?,note=? WHERE recurring_id=? AND id<>?`
	args := []any{t.Kind, t.AmountCents, t.AccountID, t.ToAccountID, t.CategoryID, t.Note, rid, t.ID}
	if scope == ScopeFollowing {
		q += " AND date >= ?"
		args = append(args, date)
	}
	if _, err := tx.Exec(q, args...); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE recurring SET kind=?,amount_cents=?,account_id=?,to_account_id=?,category_id=?,note=? WHERE id=?`,
		t.Kind, t.AmountCents, t.AccountID, t.ToAccountID, t.CategoryID, t.Note, rid); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteTxScope elimina un movimento. Per quelli nati da una ricorrenza:
//   - "following": elimina questo e i successivi e ferma la ricorrenza al giorno prima;
//   - "all": elimina tutti i movimenti della ricorrenza e la ricorrenza stessa.
func (s *Store) DeleteTxScope(id int64, scope string) error {
	scope, err := validScope(scope)
	if err != nil {
		return err
	}
	date, rid, err := s.txOrigin(id)
	if err != nil {
		return err
	}
	if scope == ScopeOne || rid == 0 {
		return s.DeleteTx(id)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if scope == ScopeAll {
		if _, err := tx.Exec("DELETE FROM transactions WHERE recurring_id=?", rid); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM recurring WHERE id=?", rid); err != nil {
			return err
		}
		return tx.Commit()
	}

	d, perr := time.Parse("2006-01-02", date)
	if perr != nil {
		return perr
	}
	prev := d.AddDate(0, 0, -1).Format("2006-01-02")
	if _, err := tx.Exec("DELETE FROM transactions WHERE recurring_id=? AND date >= ?", rid, date); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE recurring SET active=0,
	  end_date = CASE WHEN end_date <> '' AND end_date < ? THEN end_date ELSE ? END WHERE id=?`, prev, prev, rid); err != nil {
		return err
	}
	return tx.Commit()
}
