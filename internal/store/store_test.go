package store

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustAccount(t *testing.T, s *Store, name string, initial int64) *Account {
	t.Helper()
	a := &Account{Name: name, InitialCents: initial}
	if err := s.CreateAccount(a); err != nil {
		t.Fatal(err)
	}
	return a
}

func mustCat(t *testing.T, s *Store, name, kind string, parent *int64) *Category {
	t.Helper()
	c := &Category{Name: name, Kind: kind, ParentID: parent}
	if err := s.CreateCategory(c); err != nil {
		t.Fatal(err)
	}
	return c
}

func balance(t *testing.T, s *Store, id int64) int64 {
	t.Helper()
	a, err := s.GetAccount(id)
	if err != nil {
		t.Fatal(err)
	}
	return a.BalanceCents
}

func TestBalancesAndTransfers(t *testing.T) {
	s := newStore(t)
	a := mustAccount(t, s, "Banca", 100000)
	b := mustAccount(t, s, "Contanti", 0)
	food := mustCat(t, s, "Cibo", "expense", nil)
	sal := mustCat(t, s, "Stipendio", "income", nil)

	for _, tx := range []*Tx{
		{Date: "2026-01-05", Kind: "income", AmountCents: 200000, AccountID: a.ID, CategoryID: &sal.ID},
		{Date: "2026-01-06", Kind: "expense", AmountCents: 2550, AccountID: a.ID, CategoryID: &food.ID},
		{Date: "2026-01-07", Kind: "transfer", AmountCents: 5000, AccountID: a.ID, ToAccountID: &b.ID},
	} {
		if err := s.CreateTx(tx); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := balance(t, s, a.ID), int64(100000+200000-2550-5000); got != want {
		t.Errorf("saldo banca = %d, atteso %d", got, want)
	}
	if got := balance(t, s, b.ID); got != 5000 {
		t.Errorf("saldo contanti = %d, atteso 5000", got)
	}

	// i trasferimenti non contano nel riepilogo
	sum, err := s.Summary(2026, 1)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Income != 200000 || sum.Expense != 2550 || sum.Saved != 197450 {
		t.Errorf("riepilogo errato: %+v", sum)
	}
	if len(sum.Expenses) != 1 || sum.Expenses[0].Name != "Cibo" {
		t.Errorf("categorie spese errate: %+v", sum.Expenses)
	}
	if sum.Months[0].Income != 200000 || sum.Months[1].Income != 0 {
		t.Errorf("mesi errati: %+v", sum.Months[:2])
	}
}

func TestTxValidation(t *testing.T) {
	s := newStore(t)
	a := mustAccount(t, s, "A", 0)
	inc := mustCat(t, s, "Stipendio", "income", nil)
	cases := map[string]*Tx{
		"importo zero":        {Date: "2026-01-01", Kind: "expense", AmountCents: 0, AccountID: a.ID},
		"data errata":         {Date: "01/01/2026", Kind: "expense", AmountCents: 1, AccountID: a.ID},
		"trasferimento a sé":  {Date: "2026-01-01", Kind: "transfer", AmountCents: 1, AccountID: a.ID, ToAccountID: &a.ID},
		"trasferimento senza": {Date: "2026-01-01", Kind: "transfer", AmountCents: 1, AccountID: a.ID},
		"categoria sbagliata": {Date: "2026-01-01", Kind: "expense", AmountCents: 1, AccountID: a.ID, CategoryID: &inc.ID},
		"conto inesistente":   {Date: "2026-01-01", Kind: "expense", AmountCents: 1, AccountID: 999},
		"tipo sconosciuto":    {Date: "2026-01-01", Kind: "boh", AmountCents: 1, AccountID: a.ID},
	}
	for name, tx := range cases {
		if err := s.CreateTx(tx); err == nil {
			t.Errorf("%s: atteso errore", name)
		} else if _, ok := err.(ErrInvalid); !ok {
			t.Errorf("%s: errore non ErrInvalid: %v", name, err)
		}
	}
}

func TestDeleteCategoryMovesTransactions(t *testing.T) {
	s := newStore(t)
	a := mustAccount(t, s, "A", 0)
	casa := mustCat(t, s, "Casa", "expense", nil)
	bollette := mustCat(t, s, "Bollette", "expense", &casa.ID)
	altro := mustCat(t, s, "Altro", "expense", nil)
	if err := s.CreateCategory(&Category{Name: "x", ParentID: &bollette.ID}); err == nil {
		t.Error("atteso errore: terzo livello")
	}
	if sub := mustCat(t, s, "Sub", "income", &casa.ID); sub.Kind != "expense" {
		t.Error("la sottocategoria deve ereditare il tipo")
	}
	tx := &Tx{Date: "2026-02-01", Kind: "expense", AmountCents: 1000, AccountID: a.ID, CategoryID: &bollette.ID}
	if err := s.CreateTx(tx); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteCategory(casa.ID, nil); err == nil {
		t.Fatal("atteso errore: movimenti presenti e nessuna destinazione")
	}
	if err := s.DeleteCategory(casa.ID, &casa.ID); err == nil {
		t.Fatal("atteso errore: destinazione nell'albero eliminato")
	}
	if err := s.DeleteCategory(casa.ID, &altro.ID); err != nil {
		t.Fatal(err)
	}
	txs, _ := s.ListTx(TxFilter{})
	if len(txs) != 1 || txs[0].CategoryID == nil || *txs[0].CategoryID != altro.ID {
		t.Errorf("movimento non spostato: %+v", txs)
	}
	cats, _ := s.ListCategories()
	if len(cats) != 1 || cats[0].ID != altro.ID {
		t.Errorf("categorie rimaste: %+v", cats)
	}
	// categoria senza movimenti: si elimina liberamente
	if err := s.DeleteCategory(altro.ID, nil); err == nil {
		t.Error("Altro ha ancora un movimento: serve una destinazione")
	}
}

func TestCategoryFilterIncludesSubs(t *testing.T) {
	s := newStore(t)
	a := mustAccount(t, s, "A", 0)
	casa := mustCat(t, s, "Casa", "expense", nil)
	sub := mustCat(t, s, "Affitto", "expense", &casa.ID)
	s.CreateTx(&Tx{Date: "2026-03-01", Kind: "expense", AmountCents: 500, AccountID: a.ID, CategoryID: &sub.ID})
	txs, _ := s.ListTx(TxFilter{CategoryID: casa.ID})
	if len(txs) != 1 {
		t.Errorf("il filtro sulla macro deve includere le sottocategorie, trovati %d", len(txs))
	}
	sum, _ := s.Summary(2026, 3)
	if len(sum.Expenses) != 1 || sum.Expenses[0].Cents != 500 || len(sum.Expenses[0].Subs) != 1 {
		t.Errorf("aggregazione per macro errata: %+v", sum.Expenses)
	}
}

func TestDeleteAccountRules(t *testing.T) {
	s := newStore(t)
	a := mustAccount(t, s, "A", 0)
	b := mustAccount(t, s, "B", 0)
	s.CreateTx(&Tx{Date: "2026-03-01", Kind: "income", AmountCents: 1, AccountID: a.ID})
	if err := s.DeleteAccount(a.ID); err == nil {
		t.Error("un conto con movimenti non si elimina")
	}
	if err := s.DeleteAccount(b.ID); err != nil {
		t.Error(err)
	}
}

func TestRecurringBackfillAndEnd(t *testing.T) {
	s := newStore(t)
	a := mustAccount(t, s, "A", 0)
	r := &Recurring{Kind: "expense", AmountCents: 1000, AccountID: a.ID, Freq: "monthly", StartDate: "2026-01-31", Active: true}
	if err := s.CreateRecurring(r); err != nil {
		t.Fatal(err)
	}
	n, err := s.RunRecurring("2026-04-15")
	if err != nil || n != 3 { // 31 gen, 28 feb, 31 mar
		t.Fatalf("generati %d (err %v), attesi 3", n, err)
	}
	txs, _ := s.ListTx(TxFilter{})
	var dates []string
	for i := len(txs) - 1; i >= 0; i-- {
		dates = append(dates, txs[i].Date)
	}
	if strings.Join(dates, ",") != "2026-01-31,2026-02-28,2026-03-31" {
		t.Errorf("date errate: %v", dates)
	}
	// idempotente
	if n, _ := s.RunRecurring("2026-04-15"); n != 0 {
		t.Errorf("seconda esecuzione ha generato %d movimenti", n)
	}
	if n, _ := s.RunRecurring("2026-04-30"); n != 1 {
		t.Errorf("attesa l'occorrenza del 30 aprile, generati %d", n)
	}

	// con data di fine
	r2 := &Recurring{Kind: "income", AmountCents: 500, AccountID: a.ID, Freq: "weekly", StartDate: "2026-01-01", EndDate: "2026-01-20", Active: true}
	s.CreateRecurring(r2)
	if n, _ := s.RunRecurring("2026-12-31"); n < 3 {
		t.Fatal("la ricorrenza settimanale doveva generare movimenti")
	}
	list, _ := s.ListRecurring()
	for _, x := range list {
		if x.ID == r2.ID && x.Active {
			t.Error("la ricorrenza finita deve disattivarsi")
		}
	}
	got, _ := s.ListTx(TxFilter{To: "2026-01-31", Kind: "income"})
	if len(got) != 3 { // 1, 8, 15 gen (22 > fine)
		t.Errorf("movimenti settimanali = %d, attesi 3", len(got))
	}

	// ricorrenza in pausa non genera
	r3 := &Recurring{Kind: "expense", AmountCents: 1, AccountID: a.ID, Freq: "yearly", StartDate: "2020-01-01", Active: false}
	s.CreateRecurring(r3)
	before, _ := s.ListTx(TxFilter{})
	s.RunRecurring("2026-12-31")
	after, _ := s.ListTx(TxFilter{})
	if len(after) != len(before) {
		t.Error("una ricorrenza in pausa non deve generare movimenti")
	}
}

func TestNextOccurrenceYearlyLeap(t *testing.T) {
	start := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)
	n1 := NextOccurrence("yearly", start, start)
	if n1.Format("2006-01-02") != "2025-02-28" {
		t.Errorf("atteso 2025-02-28, ottenuto %s", n1.Format("2006-01-02"))
	}
	n2 := NextOccurrence("yearly", start, NextOccurrence("yearly", start, n1))
	if n2.Format("2006-01-02") != "2027-02-28" {
		t.Errorf("atteso 2027-02-28, ottenuto %s", n2.Format("2006-01-02"))
	}
}

func TestCSVRoundTrip(t *testing.T) {
	s := newStore(t)
	csvIn := "data;tipo;importo;conto;conto_destinazione;categoria;sottocategoria;nota\n" +
		"2026-01-02;uscita;12,50;Banca;;Casa;Bollette;luce\n" +
		"2026-01-03;entrata;1.500,00;Banca;;Stipendio;;\n" +
		"2026-01-04;trasferimento;100;Banca;Contanti;;;\n" +
		"2026-01-05;;-3.20;Contanti;;Cibo;;caffè\n"
	n, err := s.ImportCSV([]byte(csvIn))
	if err != nil || n != 4 {
		t.Fatalf("import: %d righe, errore %v", n, err)
	}
	accs, _ := s.ListAccounts()
	bal := map[string]int64{}
	for _, a := range accs {
		bal[a.Name] = a.BalanceCents
	}
	if bal["Banca"] != -1250+150000-10000 || bal["Contanti"] != 10000-320 {
		t.Errorf("saldi dopo import: %v", bal)
	}

	var buf bytes.Buffer
	if err := s.ExportCSV(&buf); err != nil {
		t.Fatal(err)
	}
	s2 := newStore(t)
	if n, err := s2.ImportCSV(buf.Bytes()); err != nil || n != 4 {
		t.Fatalf("reimport: %d righe, errore %v\n%s", n, err, buf.String())
	}
	accs2, _ := s2.ListAccounts()
	for _, a := range accs2 {
		if bal[a.Name] != a.BalanceCents {
			t.Errorf("saldo %s dopo round-trip = %d, atteso %d", a.Name, a.BalanceCents, bal[a.Name])
		}
	}

	if _, err := s.ImportCSV([]byte("data,importo,conto\n2026-01-01,abc,X\n")); err == nil {
		t.Error("atteso errore su importo non valido")
	}
	before, _ := s.ListTx(TxFilter{})
	s.ImportCSV([]byte("data,importo,conto\n2026-01-01,5,X\n2026-13-45,5,X\n"))
	after, _ := s.ListTx(TxFilter{})
	if len(before) != len(after) {
		t.Error("un CSV con righe non valide non deve importare nulla")
	}
}

func TestParseCents(t *testing.T) {
	for in, want := range map[string]int64{"12,50": 1250, "12.5": 1250, "1.234,56": 123456, "-3": -300, "€ 7": 700, "0,07": 7} {
		if got, err := ParseCents(in); err != nil || got != want {
			t.Errorf("ParseCents(%q) = %d, %v; atteso %d", in, got, err, want)
		}
	}
	if _, err := ParseCents("abc"); err == nil {
		t.Error("atteso errore")
	}
}

func TestBackup(t *testing.T) {
	s := newStore(t)
	mustAccount(t, s, "A", 123)
	dest := filepath.Join(t.TempDir(), "b.db")
	if err := s.BackupTo(dest); err != nil {
		t.Fatal(err)
	}
	b, err := Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if accs, _ := b.ListAccounts(); len(accs) != 1 || accs[0].InitialCents != 123 {
		t.Errorf("backup non valido: %+v", accs)
	}
}
