package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLanguageAndCurrencySettings(t *testing.T) {
	c := newServer(t)

	// i valori predefiniti sono visibili anche prima del login (servono alla schermata di accesso)
	st := c.must("GET", "/api/status", nil, 200)
	if st["lang"] != "en" || st["currency"] != "EUR" {
		t.Fatalf("predefiniti: %v", st)
	}

	c.must("POST", "/api/settings", map[string]string{"language": "it"}, 401) // serve il login
	c.must("POST", "/api/setup", map[string]string{"password": "segreta1"}, 204)

	c.must("POST", "/api/settings", map[string]string{"language": "fr"}, 400)
	c.must("POST", "/api/settings", map[string]string{"currency": "JPY"}, 400)
	if st := c.must("GET", "/api/status", nil, 200); st["lang"] != "en" || st["currency"] != "EUR" {
		t.Fatalf("un valore non valido non deve cambiare nulla: %v", st)
	}

	out := c.must("POST", "/api/settings", map[string]string{"language": "it", "currency": "AUD"}, 200)
	if out["lang"] != "it" || out["currency"] != "AUD" {
		t.Fatalf("risposta: %v", out)
	}
	// un solo campo alla volta: l'altro resta com'è
	c.must("POST", "/api/settings", map[string]string{"currency": "NZD"}, 200)
	if st := c.must("GET", "/api/status", nil, 200); st["lang"] != "it" || st["currency"] != "NZD" {
		t.Fatalf("dopo l'aggiornamento: %v", st)
	}
	for _, cur := range []string{"EUR", "USD", "AUD", "NZD", "GBP"} {
		c.must("POST", "/api/settings", map[string]string{"currency": cur}, 200)
	}
}

func TestExportFollowsAppLanguage(t *testing.T) {
	c := newServer(t)
	c.must("POST", "/api/setup", map[string]string{"password": "segreta1"}, 204)
	header := func(path string) string {
		req, rec := c.rawGet(path)
		_ = req
		return strings.SplitN(rec, "\n", 2)[0]
	}
	if h := header("/api/export.csv"); !strings.HasPrefix(h, "date,type,amount") {
		t.Errorf("inglese di default: %q", h)
	}
	c.must("POST", "/api/settings", map[string]string{"language": "it"}, 200)
	if h := header("/api/export.csv"); !strings.HasPrefix(h, "data,tipo,importo") {
		t.Errorf("italiano dopo il cambio: %q", h)
	}
	if h := header("/api/export.csv?lang=en"); !strings.HasPrefix(h, "date,type,amount") {
		t.Errorf("il parametro lang ha la precedenza: %q", h)
	}
}

// rawGet esegue una GET con i cookie del client e restituisce stato e corpo grezzo.
func (c *client) rawGet(path string) (int, string) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = testHost
	req.RemoteAddr = "10.0.0.5:4000"
	for _, ck := range c.cookies {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}
