// Package server espone l'API JSON, l'autenticazione e i file statici.
package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/Bagareal/Finance-app/internal/store"
)

const (
	cookieName   = "fa_session"
	sessionTTL   = 30 * 24 * time.Hour
	maxBodyBytes = 8 << 20
	minPassword  = 6
)

type Server struct {
	st      *store.Store
	version string
	mux     *http.ServeMux

	failMu   sync.Mutex
	failures map[string]int
}

func New(st *store.Store, web fs.FS, version string) http.Handler {
	s := &Server{st: st, version: version, mux: http.NewServeMux(), failures: map[string]int{}}
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
	code, msg := http.StatusInternalServerError, "errore interno"
	var he httpError
	var inv store.ErrInvalid
	switch {
	case errors.As(err, &he):
		code, msg = he.code, he.msg
	case errors.As(err, &inv):
		code, msg = http.StatusBadRequest, inv.Error()
	case errors.Is(err, store.ErrNotFound):
		code, msg = http.StatusNotFound, "non trovato"
	default:
		log.Printf("errore: %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return badRequest("richiesta non valida")
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, badRequest("id non valido")
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
			writeErr(w, httpError{http.StatusUnauthorized, "accesso richiesto"})
			return
		}
		h(w, r)
	}
}

func (s *Server) startSession(w http.ResponseWriter) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := hex.EncodeToString(b)
	exp := time.Now().Add(sessionTTL)
	if err := s.st.CreateSession(token, exp); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", Expires: exp, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return nil
}

func hashPassword(p string) (string, error) {
	if len(p) < minPassword {
		return "", badRequest("la password deve avere almeno 6 caratteri")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(h), err
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) (any, error) {
	_, hasPw := s.st.GetSetting("password_hash")
	return map[string]any{"setup": hasPw, "auth": s.authed(r), "version": s.version}, nil
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, ok := s.st.GetSetting("password_hash"); ok {
		return nil, httpError{http.StatusConflict, "password già impostata"}
	}
	var in struct{ Password string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	h, err := hashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	if err := s.st.SetSetting("password_hash", h); err != nil {
		return nil, err
	}
	return nil, s.startSession(w)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct{ Password string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	s.failMu.Lock()
	fails := s.failures[ip]
	s.failMu.Unlock()
	if fails >= 5 {
		time.Sleep(2 * time.Second) // rallenta i tentativi ripetuti
	}
	h, ok := s.st.GetSetting("password_hash")
	if !ok {
		return nil, httpError{http.StatusConflict, "password non ancora impostata"}
	}
	if bcrypt.CompareHashAndPassword([]byte(h), []byte(in.Password)) != nil {
		s.failMu.Lock()
		s.failures[ip]++
		s.failMu.Unlock()
		time.Sleep(400 * time.Millisecond)
		return nil, httpError{http.StatusUnauthorized, "password errata"}
	}
	s.failMu.Lock()
	delete(s.failures, ip)
	s.failMu.Unlock()
	return nil, s.startSession(w)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) (any, error) {
	if c, err := r.Cookie(cookieName); err == nil {
		s.st.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	return nil, nil
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct{ Current, New string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	h, _ := s.st.GetSetting("password_hash")
	if bcrypt.CompareHashAndPassword([]byte(h), []byte(in.Current)) != nil {
		return nil, httpError{http.StatusUnauthorized, "password attuale errata"}
	}
	nh, err := hashPassword(in.New)
	if err != nil {
		return nil, err
	}
	if err := s.st.SetSetting("password_hash", nh); err != nil {
		return nil, err
	}
	s.st.DeleteAllSessions()
	return nil, s.startSession(w)
}

// --- routes ---

func (s *Server) routes(web fs.FS) {
	m := s.mux
	p := func(h handler) http.HandlerFunc { return s.protect(s.api(h)) }

	m.HandleFunc("GET /api/status", s.api(s.status))
	m.HandleFunc("POST /api/setup", s.api(s.setup))
	m.HandleFunc("POST /api/login", s.api(s.login))
	m.HandleFunc("POST /api/logout", s.api(s.logout))
	m.HandleFunc("POST /api/password", p(s.changePassword))

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
		if err := s.st.UpdateTx(&t); err != nil {
			return nil, err
		}
		return t, nil
	}))
	m.HandleFunc("DELETE /api/transactions/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		return nil, s.st.DeleteTx(id)
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
		if err := s.st.ExportCSV(w); err != nil {
			log.Printf("export: %v", err)
		}
	}))
	m.HandleFunc("POST /api/import", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, badRequest("file troppo grande")
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
		writeErr(w, httpError{http.StatusNotFound, "non trovato"})
	})

	files := http.FileServerFS(web)
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
}
