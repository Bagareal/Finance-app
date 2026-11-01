package store

import "testing"

func mustPro(t *testing.T, s *Store, kind, name string) *ProName {
	t.Helper()
	n := &ProName{Name: name}
	if err := s.CreateProName(kind, n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestProPaymentNetAndEdit(t *testing.T) {
	s := newStore(t)
	cl := mustPro(t, s, "clients", "Acme")
	tva := mustPro(t, s, "categories", "VAT")
	soc := mustPro(t, s, "categories", "Social")

	p := &ProPayment{Date: "2026-10-14", ClientID: cl.ID, GrossCents: 100000,
		Lines: []ProLine{{CategoryID: tva.ID, AmountCents: 20000}, {CategoryID: soc.ID, AmountCents: 22000}}}
	if err := s.CreateProPayment(p); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetProPayment(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.NetCents != 58000 || len(got.Lines) != 2 || got.ClientName != "Acme" {
		t.Fatalf("payment: %+v", got)
	}

	// a past payment can be edited: new date, fewer lines
	got.Date, got.Lines = "2026-09-30", got.Lines[:1]
	if err := s.UpdateProPayment(got); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListProPayments("2026-10-01", "2026-10-31"); len(list) != 0 {
		t.Fatalf("october should be empty: %+v", list)
	}
	list, _ := s.ListProPayments("2026-09-01", "2026-09-30")
	if len(list) != 1 || list[0].NetCents != 80000 {
		t.Fatalf("september: %+v", list)
	}

	// more than the gross is allowed: the net just goes negative
	list[0].Lines = []ProLine{{CategoryID: tva.ID, AmountCents: 150000}}
	if err := s.UpdateProPayment(&list[0]); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.GetProPayment(p.ID); again.NetCents != -50000 {
		t.Fatalf("net: %+v", again)
	}
}

func TestProValidation(t *testing.T) {
	s := newStore(t)
	cl := mustPro(t, s, "clients", "Acme")
	tva := mustPro(t, s, "categories", "VAT")
	bad := []*ProPayment{
		{Date: "oggi", ClientID: cl.ID, GrossCents: 100},
		{Date: "2026-01-01", ClientID: cl.ID, GrossCents: 0},
		{Date: "2026-01-01", ClientID: 999, GrossCents: 100},
		{Date: "2026-01-01", ClientID: cl.ID, GrossCents: 100, Lines: []ProLine{{CategoryID: 999, AmountCents: 1}}},
		{Date: "2026-01-01", ClientID: cl.ID, GrossCents: 100, Lines: []ProLine{{CategoryID: tva.ID, AmountCents: 0}}},
		{Date: "2026-01-01", ClientID: cl.ID, GrossCents: 100, Lines: []ProLine{{CategoryID: tva.ID, AmountCents: 1}, {CategoryID: tva.ID, AmountCents: 2}}},
	}
	for i, p := range bad {
		if err := s.CreateProPayment(p); err == nil {
			t.Errorf("payment %d should be rejected", i)
		}
	}
	if err := s.CreateProName("clients", &ProName{Name: "  "}); err == nil {
		t.Error("empty name should be rejected")
	}
}

func TestProDeleteKeepsHistory(t *testing.T) {
	s := newStore(t)
	cl := mustPro(t, s, "clients", "Acme")
	unused := mustPro(t, s, "clients", "Nobody")
	tva := mustPro(t, s, "categories", "VAT")
	p := &ProPayment{Date: "2026-03-01", ClientID: cl.ID, GrossCents: 1000, Lines: []ProLine{{CategoryID: tva.ID, AmountCents: 200}}}
	if err := s.CreateProPayment(p); err != nil {
		t.Fatal(err)
	}

	// used client and category: archived, hidden from the lists, still shown in the payment
	if err := s.DeleteProName("clients", cl.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProName("categories", tva.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProName("clients", unused.ID); err != nil { // never used: really deleted
		t.Fatal(err)
	}
	if l, _ := s.ListProNames("clients"); len(l) != 0 {
		t.Fatalf("clients: %+v", l)
	}
	if l, _ := s.ListProNames("categories"); len(l) != 0 {
		t.Fatalf("categories: %+v", l)
	}
	got, err := s.GetProPayment(p.ID)
	if err != nil || got.ClientName != "Acme" || got.Lines[0].Name != "VAT" || got.NetCents != 800 {
		t.Fatalf("history lost: %+v %v", got, err)
	}

	// the old payment can still be edited, but the archived client cannot go on a new one
	got.GrossCents = 2000
	if err := s.UpdateProPayment(got); err != nil {
		t.Fatalf("editing an old payment: %v", err)
	}
	if err := s.CreateProPayment(&ProPayment{Date: "2026-03-02", ClientID: cl.ID, GrossCents: 5}); err == nil {
		t.Error("an archived client must not be usable for a new payment")
	}

	if err := s.DeleteProPayment(p.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListProPayments("", ""); len(list) != 0 {
		t.Fatalf("payments: %+v", list)
	}
	if err := s.DeleteProPayment(p.ID); err != ErrNotFound {
		t.Fatalf("second delete: %v", err)
	}
}
