// Package server espone l'API JSON, l'autenticazione e i file statici.
package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Bagareal/Finance-app/internal/store"
)

const (
	cookieName   = "fa_session"
	maxBodyBytes = 8 << 20
)

type Server struct {
	st      *store.Store
	version string
	mux     *http.ServeMux

	failMu   sync.Mutex
	failures map[string]int
	chal     challenges
}

func New(st *store.Store, web fs.FS, version string) http.Handler {
	s := &Server{st: st, version: version, mux: http.NewServeMux(), failures: map[string]int{}, chal: challenges{m: map[string]time.Time{}}}
	s.routes(web)
	return s.mux
}

type handler func(w http.ResponseWriter, r *http.Request) (any, error)

type httpError struct {
	code int
	msg  string
}

func (e httpError) Error() string { return e.msg }

func badRequest(msg string) error { return httpError{http.StatusBadRequest, msg} }

func (s *Server) api(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		out, err := h(w, r)
		if err != nil {
			writeErr(w, err)
			return
		}
		if out == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(out)
	}
}

func writeErr(w http.ResponseWriter, err error) {
	code, msg := http.StatusInternalServerError, "internal error"
	var he httpError
	var inv store.ErrInvalid
	switch {
	case errors.As(err, &he):
		code, msg = he.code, he.msg
	case errors.As(err, &inv):
		code, msg = http.StatusBadRequest, inv.Error()
	case errors.Is(err, store.ErrNotFound):
		code, msg = http.StatusNotFound, "not found"
	default:
		log.Printf("errore: %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return badRequest("invalid request")
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, badRequest("invalid id")
	}
	return id, nil
}

func intParam(r *http.Request, name string) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	return v
}

// --- auth ---

func (s *Server) authed(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	return err == nil && s.st.SessionValid(c.Value)
}

func (s *Server) protect(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authed(r) {
			writeErr(w, httpError{http.StatusUnauthorized, "sign-in required"})
			return
		}
		h(w, r)
	}
}

// --- routes ---

func (s *Server) routes(web fs.FS) {
	m := s.mux
	p := func(h handler) http.HandlerFunc { return s.protect(s.api(h)) }

	s.authRoutes(p)

	m.HandleFunc("GET /api/accounts", p(func(w http.ResponseWriter, r *http.Request) (any, error) { return s.st.ListAccounts() }))
	m.HandleFunc("POST /api/accounts", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		var a store.Account
		if err := decode(r, &a); err != nil {
			return nil, err
		}
		a.ID = 0
		if err := s.st.CreateAccount(&a); err != nil {
			return nil, err
		}
		return s.st.GetAccount(a.ID)
	}))
	m.HandleFunc("PUT /api/accounts/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		var a store.Account
		if err := decode(r, &a); err != nil {
			return nil, err
		}
		a.ID = id
		if err := s.st.UpdateAccount(&a); err != nil {
			return nil, err
		}
		return s.st.GetAccount(id)
	}))
	m.HandleFunc("DELETE /api/accounts/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		return nil, s.st.DeleteAccount(id)
	}))
	m.HandleFunc("GET /api/accounts/{id}/values", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		return s.st.ListValues(id)
	}))
	m.HandleFunc("POST /api/accounts/{id}/values", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		var v store.AccountValue
		if err := decode(r, &v); err != nil {
			return nil, err
		}
		if err := s.st.SetValue(id, v.Date, v.ValueCents); err != nil {
			return nil, err
		}
		return s.st.GetAccount(id)
	}))
	m.HandleFunc("DELETE /api/account-values/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		return nil, s.st.DeleteValue(id)
	}))

	m.HandleFunc("GET /api/categories", p(func(w http.ResponseWriter, r *http.Request) (any, error) { return s.st.ListCategories() }))
	m.HandleFunc("POST /api/categories", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		var c store.Category
		if err := decode(r, &c); err != nil {
			return nil, err
		}
		c.ID = 0
		if err := s.st.CreateCategory(&c); err != nil {
			return nil, err
		}
		return c, nil
	}))
	m.HandleFunc("PUT /api/categories/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		var c store.Category
		if err := decode(r, &c); err != nil {
			return nil, err
		}
		c.ID = id
		if err := s.st.UpdateCategory(&c); err != nil {
			return nil, err
		}
		return c, nil
	}))
	m.HandleFunc("DELETE /api/categories/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		var moveTo *int64
		if v := intParam(r, "move_to"); v > 0 {
			moveTo = &v
		}
		return nil, s.st.DeleteCategory(id, moveTo)
	}))

	m.HandleFunc("GET /api/transactions", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		q := r.URL.Query()
		return s.st.ListTx(store.TxFilter{
			From: q.Get("from"), To: q.Get("to"), Kind: q.Get("kind"),
			AccountID: intParam(r, "account"), CategoryID: intParam(r, "category"),
			Limit: int(intParam(r, "limit")), Offset: int(intParam(r, "offset")),
		})
	}))
	m.HandleFunc("POST /api/transactions", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		var t store.Tx
		if err := decode(r, &t); err != nil {
			return nil, err
		}
		t.ID, t.RecurringID = 0, nil
		if err := s.st.CreateTx(&t); err != nil {
			return nil, err
		}
		return t, nil
	}))
	m.HandleFunc("PUT /api/transactions/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		var t store.Tx
		if err := decode(r, &t); err != nil {
			return nil, err
		}
		t.ID = id
		if err := s.st.UpdateTxScope(&t, r.URL.Query().Get("scope")); err != nil {
			return nil, err
		}
		return t, nil
	}))
	m.HandleFunc("DELETE /api/transactions/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		return nil, s.st.DeleteTxScope(id, r.URL.Query().Get("scope"))
	}))

	m.HandleFunc("GET /api/recurring", p(func(w http.ResponseWriter, r *http.Request) (any, error) { return s.st.ListRecurring() }))
	m.HandleFunc("POST /api/recurring", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		var x store.Recurring
		if err := decode(r, &x); err != nil {
			return nil, err
		}
		x.ID = 0
		if err := s.st.CreateRecurring(&x); err != nil {
			return nil, err
		}
		s.st.RunRecurring("") // genera subito eventuali arretrati
		return x, nil
	}))
	m.HandleFunc("PUT /api/recurring/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		var x store.Recurring
		if err := decode(r, &x); err != nil {
			return nil, err
		}
		x.ID = id
		if err := s.st.UpdateRecurring(&x); err != nil {
			return nil, err
		}
		s.st.RunRecurring("")
		return x, nil
	}))
	m.HandleFunc("DELETE /api/recurring/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		return nil, s.st.DeleteRecurring(id)
	}))

	s.proRoutes(p)

	m.HandleFunc("GET /api/summary", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		year := int(intParam(r, "year"))
		if year == 0 {
			year = time.Now().Year()
		}
		return s.st.Summary(year, int(intParam(r, "month")))
	}))

	m.HandleFunc("GET /api/export.csv", s.protect(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="movimenti-`+time.Now().Format("2006-01-02")+`.csv"`)
		lang := r.URL.Query().Get("lang")
		if lang != "en" && lang != "it" {
			lang = s.lang()
		}
		if err := s.st.ExportCSV(w, lang); err != nil {
			log.Printf("export: %v", err)
		}
	}))
	m.HandleFunc("POST /api/import", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, badRequest("file too large")
		}
		n, err := s.st.ImportCSV(data)
		if err != nil {
			return nil, err
		}
		return map[string]int{"imported": n}, nil
	}))
	m.HandleFunc("GET /api/backup", s.protect(func(w http.ResponseWriter, r *http.Request) {
		dir, err := os.MkdirTemp("", "fa-backup")
		if err != nil {
			writeErr(w, err)
			return
		}
		defer os.RemoveAll(dir)
		dest := filepath.Join(dir, "finance.db")
		if err := s.st.BackupTo(dest); err != nil {
			writeErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="finance-`+time.Now().Format("2006-01-02")+`.db"`)
		http.ServeFile(w, r, dest)
	}))

	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, httpError{http.StatusNotFound, "not found"})
	})

	// L'HTML non va mai in cache e punta agli asset con ?v=<versione>: a ogni release gli URL cambiano,
	// così né il browser né un reverse proxy / CDN possono servire file vecchi.
	index, _ := fs.ReadFile(web, "index.html")
	page := bytes.ReplaceAll(index, []byte("__V__"), []byte(url.QueryEscape(s.version)))
	files := http.FileServerFS(web)
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Write(page)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
}
