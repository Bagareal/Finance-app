package store

import (
	"fmt"
	"sort"
)

type CatTotal struct {
	CategoryID int64      `json:"category_id"` // 0 = senza categoria
	Name       string     `json:"name"`
	Color      string     `json:"color"`
	Cents      int64      `json:"cents"`
	Subs       []CatTotal `json:"subs,omitempty"`
}

type MonthTotal struct {
	Month   int   `json:"month"`
	Income  int64 `json:"income"`
	Expense int64 `json:"expense"`
}

type Summary struct {
	From     string       `json:"from"`
	To       string       `json:"to"`
	Income   int64        `json:"income"`
	Expense  int64        `json:"expense"`
	Saved    int64        `json:"saved"`
	Expenses []CatTotal   `json:"expenses"`
	Incomes  []CatTotal   `json:"incomes"`
	Months   []MonthTotal `json:"months"` // 12 mesi dell'anno richiesto
}

// Summary calcola guadagnato, speso e risparmiato. month=0 significa tutto l'anno.
// I trasferimenti tra conti non contano.
func (s *Store) Summary(year, month int) (*Summary, error) {
	if year < 1970 || year > 9999 || month < 0 || month > 12 {
		return nil, invalid("periodo non valido")
	}
	sum := &Summary{Expenses: []CatTotal{}, Incomes: []CatTotal{}, Months: make([]MonthTotal, 12)}
	for i := range sum.Months {
		sum.Months[i].Month = i + 1
	}
	if month == 0 {
		sum.From, sum.To = fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-12-31", year)
	} else {
		sum.From, sum.To = fmt.Sprintf("%04d-%02d-01", year, month), fmt.Sprintf("%04d-%02d-31", year, month)
	}

	cats, err := s.ListCategories()
	if err != nil {
		return nil, err
	}
	byID := map[int64]Category{}
	for _, c := range cats {
		byID[c.ID] = c
	}

	rows, err := s.db.Query(`SELECT COALESCE(category_id,0), kind, SUM(amount_cents) FROM transactions
	  WHERE kind IN ('income','expense') AND date BETWEEN ? AND ? GROUP BY category_id, kind`, sum.From, sum.To)
	if err != nil {
		return nil, err
	}
	type key struct {
		kind  string
		macro int64
	}
	macros := map[key]*CatTotal{}
	for rows.Next() {
		var cid, cents int64
		var kind string
		if err := rows.Scan(&cid, &kind, &cents); err != nil {
			rows.Close()
			return nil, err
		}
		if kind == "income" {
			sum.Income += cents
		} else {
			sum.Expense += cents
		}
		c, known := byID[cid]
		macroID := cid
		if known && c.ParentID != nil {
			macroID = *c.ParentID
		}
		k := key{kind, macroID}
		m := macros[k]
		if m == nil {
			m = &CatTotal{CategoryID: macroID, Name: "Senza categoria"}
			if mc, ok := byID[macroID]; ok {
				m.Name, m.Color = mc.Name, mc.Color
			}
			macros[k] = m
		}
		m.Cents += cents
		if known && c.ParentID != nil {
			m.Subs = append(m.Subs, CatTotal{CategoryID: cid, Name: c.Name, Color: c.Color, Cents: cents})
		}
	}
	rows.Close()
	sum.Saved = sum.Income - sum.Expense
	for k, m := range macros {
		sort.Slice(m.Subs, func(i, j int) bool { return m.Subs[i].Cents > m.Subs[j].Cents })
		if k.kind == "income" {
			sum.Incomes = append(sum.Incomes, *m)
		} else {
			sum.Expenses = append(sum.Expenses, *m)
		}
	}
	for _, l := range [][]CatTotal{sum.Incomes, sum.Expenses} {
		sort.Slice(l, func(i, j int) bool { return l[i].Cents > l[j].Cents })
	}

	mrows, err := s.db.Query(`SELECT CAST(substr(date,6,2) AS INTEGER), kind, SUM(amount_cents) FROM transactions
	  WHERE kind IN ('income','expense') AND date BETWEEN ? AND ? GROUP BY 1, 2`,
		fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-12-31", year))
	if err != nil {
		return nil, err
	}
	defer mrows.Close()
	for mrows.Next() {
		var m int
		var kind string
		var cents int64
		if err := mrows.Scan(&m, &kind, &cents); err != nil {
			return nil, err
		}
		if m < 1 || m > 12 {
			continue
		}
		if kind == "income" {
			sum.Months[m-1].Income = cents
		} else {
			sum.Months[m-1].Expense = cents
		}
	}
	return sum, mrows.Err()
}
