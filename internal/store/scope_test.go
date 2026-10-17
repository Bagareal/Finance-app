package store

import "testing"

// setupRecurring crea una ricorrenza mensile con 4 movimenti generati (gen-apr) e restituisce
// l'id della ricorrenza, il conto e i movimenti in ordine di data.
func setupRecurring(t *testing.T, s *Store) (int64, *Account, []Tx) {
	t.Helper()
	a := mustAccount(t, s, "Banca", 0)
	cat := mustCat(t, s, "Casa", "expense", nil)
	r := &Recurring{Kind: "expense", AmountCents: 50000, AccountID: a.ID, CategoryID: &cat.ID, Note: "affitto", Freq: "monthly", StartDate: "2026-01-10", Active: true}
	if err := s.CreateRecurring(r); err != nil {
		t.Fatal(err)
	}
	if n, err := s.RunRecurring("2026-04-30"); err != nil || n != 4 {
		t.Fatalf("generati %d (%v), attesi 4", n, err)
	}
	txs, _ := s.ListTx(TxFilter{})
	// ListTx è in ordine decrescente di data: inverti
	for i, j := 0, len(txs)-1; i < j; i, j = i+1, j-1 {
		txs[i], txs[j] = txs[j], txs[i]
	}
	return r.ID, a, txs
}

func amounts(t *testing.T, s *Store) []int64 {
	t.Helper()
	txs, _ := s.ListTx(TxFilter{})
	var out []int64
	for i := len(txs) - 1; i >= 0; i-- {
		out = append(out, txs[i].AmountCents)
	}
	return out
}

func eq(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func edited(tx Tx, cents int64) *Tx {
	tx.AmountCents = cents
	return &tx
}

func TestUpdateScopeOneKeepsRecurrence(t *testing.T) {
	s := newStore(t)
	rid, _, txs := setupRecurring(t, s)
	if err := s.UpdateTxScope(edited(txs[1], 60000), "one"); err != nil {
		t.Fatal(err)
	}
	if got := amounts(t, s); !eq(got, []int64{50000, 60000, 50000, 50000}) {
		t.Errorf("importi: %v", got)
	}
	list, _ := s.ListRecurring()
	if list[0].ID != rid || list[0].AmountCents != 50000 {
		t.Errorf("la ricorrenza non doveva cambiare: %+v", list[0])
	}
}

func TestUpdateScopeFollowingChangesFutureToo(t *testing.T) {
	s := newStore(t)
	_, _, txs := setupRecurring(t, s)
	if err := s.UpdateTxScope(edited(txs[2], 70000), "following"); err != nil { // marzo
		t.Fatal(err)
	}
	if got := amounts(t, s); !eq(got, []int64{50000, 50000, 70000, 70000}) {
		t.Errorf("importi: %v", got)
	}
	list, _ := s.ListRecurring()
	if list[0].AmountCents != 70000 {
		t.Errorf("la ricorrenza deve usare il nuovo importo per i movimenti futuri: %+v", list[0])
	}
	// il prossimo movimento generato ha il nuovo importo
	s.RunRecurring("2026-05-30")
	if got := amounts(t, s); len(got) != 5 || got[4] != 70000 {
		t.Errorf("movimento futuro: %v", got)
	}
}

func TestUpdateScopeAll(t *testing.T) {
	s := newStore(t)
	_, _, txs := setupRecurring(t, s)
	e := edited(txs[3], 45000)
	e.Note = "affitto nuovo"
	e.Date = "2026-04-12" // la data cambia solo per questo movimento
	if err := s.UpdateTxScope(e, "all"); err != nil {
		t.Fatal(err)
	}
	if got := amounts(t, s); !eq(got, []int64{45000, 45000, 45000, 45000}) {
		t.Errorf("importi: %v", got)
	}
	all, _ := s.ListTx(TxFilter{})
	dates := map[string]bool{}
	for _, x := range all {
		dates[x.Date] = true
		if x.Note != "affitto nuovo" {
			t.Errorf("nota non propagata: %+v", x)
		}
	}
	if !dates["2026-04-12"] || !dates["2026-01-10"] || !dates["2026-03-10"] {
		t.Errorf("la data doveva cambiare solo per il movimento modificato: %v", dates)
	}
}

func TestUpdateScopeValidatesAndIgnoresPlainTx(t *testing.T) {
	s := newStore(t)
	_, a, txs := setupRecurring(t, s)
	if err := s.UpdateTxScope(edited(txs[0], 1), "boh"); err == nil {
		t.Error("ambito sconosciuto: atteso errore")
	}
	if err := s.UpdateTxScope(edited(txs[0], 0), "all"); err == nil {
		t.Error("importo zero: atteso errore")
	}
	if got := amounts(t, s); !eq(got, []int64{50000, 50000, 50000, 50000}) {
		t.Errorf("un errore non deve modificare nulla: %v", got)
	}
	plain := &Tx{Date: "2026-05-01", Kind: "expense", AmountCents: 100, AccountID: a.ID}
	s.CreateTx(plain)
	plain.AmountCents = 200
	if err := s.UpdateTxScope(plain, "all"); err != nil { // non ricorrente: vale solo per sé
		t.Fatal(err)
	}
	if got := amounts(t, s); got[len(got)-1] != 200 || got[0] != 50000 {
		t.Errorf("importi: %v", got)
	}
	if err := s.UpdateTxScope(&Tx{ID: 9999, Date: "2026-01-01", Kind: "expense", AmountCents: 1, AccountID: a.ID}, "all"); err != ErrNotFound {
		t.Errorf("atteso ErrNotFound, ottenuto %v", err)
	}
}

func TestDeleteScopeFollowingStopsRecurrence(t *testing.T) {
	s := newStore(t)
	rid, _, txs := setupRecurring(t, s)
	if err := s.DeleteTxScope(txs[2].ID, "following"); err != nil { // marzo
		t.Fatal(err)
	}
	if got := amounts(t, s); len(got) != 2 {
		t.Errorf("restano %d movimenti, attesi 2", len(got))
	}
	list, _ := s.ListRecurring()
	if list[0].ID != rid || list[0].Active || list[0].EndDate != "2026-03-09" {
		t.Errorf("la ricorrenza doveva fermarsi: %+v", list[0])
	}
	s.RunRecurring("2026-12-31")
	if got := amounts(t, s); len(got) != 2 {
		t.Errorf("non devono comparire nuovi movimenti: %d", len(got))
	}
}

func TestDeleteScopeAllRemovesRecurrence(t *testing.T) {
	s := newStore(t)
	_, _, txs := setupRecurring(t, s)
	if err := s.DeleteTxScope(txs[1].ID, "all"); err != nil {
		t.Fatal(err)
	}
	if got := amounts(t, s); len(got) != 0 {
		t.Errorf("restano %d movimenti", len(got))
	}
	if list, _ := s.ListRecurring(); len(list) != 0 {
		t.Errorf("la ricorrenza doveva sparire: %+v", list)
	}
}

func TestDeleteScopeOneLeavesTheRest(t *testing.T) {
	s := newStore(t)
	rid, _, txs := setupRecurring(t, s)
	if err := s.DeleteTxScope(txs[0].ID, "one"); err != nil {
		t.Fatal(err)
	}
	if got := amounts(t, s); len(got) != 3 {
		t.Errorf("restano %d movimenti, attesi 3", len(got))
	}
	list, _ := s.ListRecurring()
	if len(list) != 1 || list[0].ID != rid || !list[0].Active {
		t.Errorf("la ricorrenza deve continuare: %+v", list)
	}
	// il movimento eliminato non viene rigenerato
	s.RunRecurring("2026-04-30")
	if got := amounts(t, s); len(got) != 3 {
		t.Errorf("movimento rigenerato: %d", len(got))
	}
}
