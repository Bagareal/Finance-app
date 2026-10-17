package server

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionTTL   = 30 * 24 * time.Hour
	deviceTTL    = 400 * 24 * time.Hour
	deviceCookie = "fa_device"
	minPassword  = 6
	maxPinFails  = 5
)

var pinRe = regexp.MustCompile(`^[0-9]{4,6}$`)

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // senza casualità non si può continuare in sicurezza
	}
	return hex.EncodeToString(b)
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func (s *Server) setCookie(w http.ResponseWriter, r *http.Request, name, value string, ttl time.Duration) {
	c := &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isHTTPS(r)}
	if ttl > 0 {
		c.Expires = time.Now().Add(ttl)
	} else {
		c.MaxAge = -1
	}
	http.SetCookie(w, c)
}

// deviceToken restituisce il token del dispositivo se è ancora valido, altrimenti "".
func (s *Server) deviceToken(r *http.Request) string {
	if c, err := r.Cookie(deviceCookie); err == nil && s.st.DeviceValid(c.Value) {
		return c.Value
	}
	return ""
}

func (s *Server) ensureDevice(w http.ResponseWriter, r *http.Request) error {
	if s.deviceToken(r) != "" {
		return nil
	}
	tok := newToken()
	if err := s.st.CreateDevice(tok); err != nil {
		return err
	}
	s.setCookie(w, r, deviceCookie, tok, deviceTTL)
	return nil
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request) error {
	token := newToken()
	if err := s.st.CreateSession(token, time.Now().Add(sessionTTL)); err != nil {
		return err
	}
	s.setCookie(w, r, cookieName, token, sessionTTL)
	return s.ensureDevice(w, r)
}

func hashSecret(secret string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	return string(h), err
}

func hashPassword(p string) (string, error) {
	if len(p) < minPassword {
		return "", badRequest("la password deve avere almeno 6 caratteri")
	}
	return hashSecret(p)
}

func (s *Server) checkPassword(p string) bool {
	h, ok := s.st.GetSetting("password_hash")
	return ok && bcrypt.CompareHashAndPassword([]byte(h), []byte(p)) == nil
}

func clientIP(r *http.Request) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func (s *Server) tooManyFails(ip string) bool {
	s.failMu.Lock()
	defer s.failMu.Unlock()
	return s.failures[ip] >= 5
}

func (s *Server) noteFail(ip string, reset bool) {
	s.failMu.Lock()
	defer s.failMu.Unlock()
	if reset {
		delete(s.failures, ip)
	} else {
		s.failures[ip]++
	}
}

func (s *Server) authRoutes(p func(handler) http.HandlerFunc) {
	m := s.mux
	m.HandleFunc("GET /api/status", s.api(s.status))
	m.HandleFunc("POST /api/setup", s.api(s.setup))
	m.HandleFunc("POST /api/login", s.api(s.login))
	m.HandleFunc("POST /api/logout", s.api(s.logout))
	m.HandleFunc("POST /api/password", p(s.changePassword))
	m.HandleFunc("POST /api/pin/set", p(s.pinSet))
	m.HandleFunc("POST /api/pin/remove", p(s.pinRemove))
	m.HandleFunc("POST /api/pin/login", s.api(s.pinLogin))
	m.HandleFunc("POST /api/bio/register/begin", p(s.bioRegisterBegin))
	m.HandleFunc("POST /api/bio/register/finish", p(s.bioRegisterFinish))
	m.HandleFunc("POST /api/bio/remove", p(s.bioRemove))
	m.HandleFunc("POST /api/bio/login/begin", s.api(s.bioLoginBegin))
	m.HandleFunc("POST /api/bio/login/finish", s.api(s.bioLoginFinish))
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) (any, error) {
	_, hasPw := s.st.GetSetting("password_hash")
	_, hasPin := s.st.GetSetting("pin_hash")
	dev := s.deviceToken(r)
	pinLen := 0
	if hasPin {
		v, _ := s.st.GetSetting("pin_len")
		pinLen, _ = strconv.Atoi(v)
	}
	return map[string]any{
		"setup": hasPw, "auth": s.authed(r), "version": s.version,
		"device": dev != "", "pin": hasPin, "pin_len": pinLen,
		"bio": dev != "" && len(s.st.DeviceCredentials(dev)) > 0,
	}, nil
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
	return nil, s.startSession(w, r)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct{ Password string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	ip := clientIP(r)
	if s.tooManyFails(ip) {
		time.Sleep(2 * time.Second) // rallenta i tentativi ripetuti
	}
	if _, ok := s.st.GetSetting("password_hash"); !ok {
		return nil, httpError{http.StatusConflict, "password non ancora impostata"}
	}
	if !s.checkPassword(in.Password) {
		s.noteFail(ip, false)
		time.Sleep(400 * time.Millisecond)
		return nil, httpError{http.StatusUnauthorized, "password errata"}
	}
	s.noteFail(ip, true)
	return nil, s.startSession(w, r)
}

// logout chiude la sessione ma il dispositivo resta fidato: si potrà sbloccare con PIN o biometria.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) (any, error) {
	if c, err := r.Cookie(cookieName); err == nil {
		s.st.DeleteSession(c.Value)
	}
	s.setCookie(w, r, cookieName, "", 0)
	return nil, nil
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct{ Current, New string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if !s.checkPassword(in.Current) {
		return nil, httpError{http.StatusUnauthorized, "password attuale errata"}
	}
	nh, err := hashPassword(in.New)
	if err != nil {
		return nil, err
	}
	if err := s.st.SetSetting("password_hash", nh); err != nil {
		return nil, err
	}
	// una nuova password revoca ogni sessione e dispositivo noto (e quindi PIN e biometria già attivi)
	s.st.DeleteAllSessions()
	s.st.DeleteAllDevices()
	return nil, s.startSession(w, r)
}

// --- PIN ---

func (s *Server) pinSet(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct{ Password, Pin string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if !s.checkPassword(in.Password) {
		return nil, httpError{http.StatusForbidden, "password errata"}
	}
	if !pinRe.MatchString(in.Pin) {
		return nil, badRequest("il PIN deve avere da 4 a 6 cifre")
	}
	h, err := hashSecret(in.Pin)
	if err != nil {
		return nil, err
	}
	if err := s.st.SetSetting("pin_hash", h); err != nil {
		return nil, err
	}
	return nil, s.st.SetSetting("pin_len", strconv.Itoa(len(in.Pin)))
}

func (s *Server) pinRemove(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct{ Password string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if !s.checkPassword(in.Password) {
		return nil, httpError{http.StatusForbidden, "password errata"}
	}
	s.st.DeleteSetting("pin_hash")
	s.st.DeleteSetting("pin_len")
	return nil, nil
}

func (s *Server) pinLogin(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct{ Pin string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	dev := s.deviceToken(r)
	h, ok := s.st.GetSetting("pin_hash")
	if dev == "" || !ok {
		return nil, httpError{http.StatusUnauthorized, "usa la password"}
	}
	if bcrypt.CompareHashAndPassword([]byte(h), []byte(in.Pin)) != nil {
		time.Sleep(400 * time.Millisecond)
		if s.st.DeviceFail(dev) >= maxPinFails {
			s.st.DeleteDevice(dev) // troppi errori: serve di nuovo la password
			s.setCookie(w, r, deviceCookie, "", 0)
			return nil, httpError{http.StatusUnauthorized, "troppi tentativi: accedi con la password"}
		}
		return nil, httpError{http.StatusUnauthorized, "PIN errato"}
	}
	s.st.DeviceReset(dev)
	return nil, s.startSession(w, r)
}
