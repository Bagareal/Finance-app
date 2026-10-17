package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Bagareal/Finance-app/internal/store"
)

func TestIndexIsVersionedAndNotCached(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	web := fstest.MapFS{
		"index.html": {Data: []byte(`<script src="app.js?v=__V__"></script>`)},
		"app.js":     {Data: []byte("x")},
	}
	h := New(st, web, "v1.2.3")

	for _, path := range []string{"/", "/index.html"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if body := rec.Body.String(); !strings.Contains(body, "app.js?v=v1.2.3") || strings.Contains(body, "__V__") {
			t.Errorf("%s: versione non iniettata: %q", path, body)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%s: Cache-Control = %q, atteso no-store", path, cc)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app.js?v=v1.2.3", nil))
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("app.js: %d, Cache-Control %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}
