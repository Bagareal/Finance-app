package store

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

var csvHeader = []string{"data", "tipo", "importo", "conto", "conto_destinazione", "categoria", "sottocategoria", "nota"}

var kindToIT = map[string]string{"income": "entrata", "expense": "uscita", "transfer": "trasferimento"}

func FormatCents(c int64) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

// ParseCents converte "12,50", "12.50" o "-3" in centesimi.
func ParseCents(s string) (int64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, " ", ""))
	s = strings.TrimPrefix(s, "€")
	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ".", "") // 1.234,56
	}
	s = strings.ReplaceAll(s, ",", ".")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, invalid("importo non valido: %q", s)
	}
	return int64(math.Round(f * 100)), nil
}

// ExportCSV scrive tutti i movimenti in CSV (UTF-8, separatore virgola).
func (s *Store) ExportCSV(w io.Writer) error {
	accs, err := s.ListAccounts()
	if err != nil {
		return err
	}
	cats, err := s.ListCategories()
	if err != nil {
		return err
	}
	txs, err := s.ListTx(TxFilter{})
	if err != nil {
		return err
	}
	accName := map[int64]string{}
	for _, a := range accs {
		accName[a.ID] = a.Name
	}
	catByID := map[int64]Category{}
	for _, c := range cats {
		catByID[c.ID] = c
	}
	cw := csv.NewWriter(w)
	cw.Write(csvHeader)
	for i := len(txs) - 1; i >= 0; i-- { // dal più vecchio
		t := txs[i]
		to, cat, sub := "", "", ""
		if t.ToAccountID != nil {
			to = accName[*t.ToAccountID]
		}
		if t.CategoryID != nil {
			c := catByID[*t.CategoryID]
			if c.ParentID != nil {
				cat, sub = catByID[*c.ParentID].Name, c.Name
			} else {
				cat = c.Name
			}
		}
		cw.Write([]string{t.Date, kindToIT[t.Kind], FormatCents(t.AmountCents), accName[t.AccountID], to, cat, sub, t.Note})
	}
	cw.Flush()
	return cw.Error()
}

// ImportCSV importa movimenti; crea al volo conti e categorie mancanti. Prima valida tutto le righe.
// Colonne: data, tipo, importo, conto, conto_destinazione, categoria, sottocategoria, nota.
// Se "tipo" è vuoto, un importo negativo è un'uscita e uno positivo un'entrata.
func (s *Store) ImportCSV(data []byte) (int, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	if i := bytes.IndexByte(data, '\n'); i > 0 && bytes.Count(data[:i], []byte(";")) > bytes.Count(data[:i], []byte(",")) {
		r.Comma = ';'
	}
	records, err := r.ReadAll()
	if err != nil {
		return 0, invalid("CSV non valido: %v", err)
	}
	if len(records) < 2 {
		return 0, invalid("il CSV non contiene righe")
	}
	col := map[string]int{}
	for i, h := range records[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, need := range []string{"data", "importo", "conto"} {
		if _, ok := col[need]; !ok {
			return 0, invalid("manca la colonna %q", need)
		}
	}
	get := func(rec []string, name string) string {
		if i, ok := col[name]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}

	type row struct {
		date, kind, acc, to, cat, sub, note string
		cents                               int64
	}
	var rows []row
	for n, rec := range records[1:] {
		line := n + 2
		if len(rec) == 1 && strings.TrimSpace(rec[0]) == "" {
			continue
		}
		cents, err := ParseCents(get(rec, "importo"))
		if err != nil {
			return 0, invalid("riga %d: %v", line, err)
		}
		kind := ""
		switch strings.ToLower(get(rec, "tipo")) {
		case "entrata", "income":
			kind = "income"
		case "uscita", "spesa", "expense":
			kind = "expense"
		case "trasferimento", "transfer":
			kind = "transfer"
		case "":
			kind = "income"
			if cents < 0 {
				kind = "expense"
			}
		default:
			return 0, invalid("riga %d: tipo sconosciuto %q", line, get(rec, "tipo"))
		}
		if cents < 0 {
			cents = -cents
		}
		d := get(rec, "data")
		if _, err := parseDate(d); err != nil {
			return 0, invalid("riga %d: %v", line, err)
		}
		if get(rec, "conto") == "" {
			return 0, invalid("riga %d: conto mancante", line)
		}
		if kind == "transfer" && get(rec, "conto_destinazione") == "" {
			return 0, invalid("riga %d: trasferimento senza conto di destinazione", line)
		}
		rows = append(rows, row{d, kind, get(rec, "conto"), get(rec, "conto_destinazione"), get(rec, "categoria"), get(rec, "sottocategoria"), get(rec, "nota"), cents})
	}

	accs, err := s.ListAccounts()
	if err != nil {
		return 0, err
	}
	accID := map[string]int64{}
	for _, a := range accs {
		accID[strings.ToLower(a.Name)] = a.ID
	}
	cats, err := s.ListCategories()
	if err != nil {
		return 0, err
	}
	catID := map[string]int64{} // kind|parent|name
	for _, c := range cats {
		p := ""
		if c.ParentID != nil {
			for _, pc := range cats {
				if pc.ID == *c.ParentID {
					p = strings.ToLower(pc.Name)
				}
			}
		}
		catID[c.Kind+"|"+p+"|"+strings.ToLower(c.Name)] = c.ID
	}
	account := func(name string) (int64, error) {
		if id, ok := accID[strings.ToLower(name)]; ok {
			return id, nil
		}
		a := &Account{Name: name}
		if err := s.CreateAccount(a); err != nil {
			return 0, err
		}
		accID[strings.ToLower(name)] = a.ID
		return a.ID, nil
	}
	category := func(kind, cat, sub string) (*int64, error) {
		if cat == "" {
			return nil, nil
		}
		k := kind + "||" + strings.ToLower(cat)
		id, ok := catID[k]
		if !ok {
			c := &Category{Name: cat, Kind: kind}
			if err := s.CreateCategory(c); err != nil {
				return nil, err
			}
			id, catID[k] = c.ID, c.ID
		}
		if sub == "" {
			return &id, nil
		}
		sk := kind + "|" + strings.ToLower(cat) + "|" + strings.ToLower(sub)
		sid, ok := catID[sk]
		if !ok {
			c := &Category{Name: sub, Kind: kind, ParentID: &id}
			if err := s.CreateCategory(c); err != nil {
				return nil, err
			}
			sid, catID[sk] = c.ID, c.ID
		}
		return &sid, nil
	}

	for i, rw := range rows {
		from, err := account(rw.acc)
		if err != nil {
			return i, err
		}
		t := &Tx{Date: rw.date, Kind: rw.kind, AmountCents: rw.cents, AccountID: from, Note: rw.note}
		if rw.kind == "transfer" {
			to, err := account(rw.to)
			if err != nil {
				return i, err
			}
			t.ToAccountID = &to
		} else if t.CategoryID, err = category(rw.kind, rw.cat, rw.sub); err != nil {
			return i, err
		}
		if err := s.CreateTx(t); err != nil {
			return i, invalid("riga %d: %v", i+2, err)
		}
	}
	return len(rows), nil
}
