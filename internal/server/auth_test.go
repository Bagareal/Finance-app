package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Bagareal/Finance-app/internal/store"
)

const testHost = "fin.example.test"

type client struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]*http.Cookie
	https   bool
}

func newServer(t *testing.T) *client {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	web := fstest.MapFS{"index.html": {Data: []byte("ok")}}
	return &client{t: t, h: New(st, web, "test"), cookies: map[string]*http.Cookie{}, https: true}
}

// device simula un secondo browser sullo stesso server (cookie separati).
func (c *client) device() *client {
	return &client{t: c.t, h: c.h, cookies: map[string]*http.Cookie{}, https: c.https}
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Host = testHost
	req.RemoteAddr = "10.0.0.5:4000"
	if c.https {
		req.Header.Set("X-Forwarded-Proto", "https")
	}
	for _, ck := range c.cookies {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		if ck.MaxAge < 0 || ck.Value == "" {
			delete(c.cookies, ck.Name)
		} else {
			c.cookies[ck.Name] = ck
		}
	}
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func (c *client) must(method, path string, body any, want int) map[string]any {
	c.t.Helper()
	code, out := c.do(method, path, body)
	if code != want {
		c.t.Fatalf("%s %s = %d (atteso %d): %v", method, path, code, want, out)
	}
	return out
}

func TestPinFlow(t *testing.T) {
	c := newServer(t)
	c.must("POST", "/api/setup", map[string]string{"password": "segreta1"}, 204)
	if st := c.must("GET", "/api/status", nil, 200); st["auth"] != true || st["device"] != true || st["pin"] != false {
		t.Fatalf("status dopo setup: %v", st)
	}
	c.must("POST", "/api/pin/set", map[string]string{"password": "sbagliata", "pin": "1234"}, 403)
	c.must("POST", "/api/pin/set", map[string]string{"password": "segreta1", "pin": "12ab"}, 400)
	c.must("POST", "/api/pin/set", map[string]string{"password": "segreta1", "pin": "123"}, 400)
	c.must("POST", "/api/pin/set", map[string]string{"password": "segreta1", "pin": "4821"}, 204)

	c.must("POST", "/api/logout", nil, 204)
	c.must("GET", "/api/accounts", nil, 401)
	if st := c.must("GET", "/api/status", nil, 200); st["auth"] != false || st["device"] != true || st["pin"] != true || st["pin_len"] != float64(4) {
		t.Fatalf("status da bloccato: %v", st)
	}
	c.must("POST", "/api/pin/login", map[string]string{"pin": "0000"}, 401)
	c.must("POST", "/api/pin/login", map[string]string{"pin": "4821"}, 204)
	c.must("GET", "/api/accounts", nil, 200)

	// un dispositivo che non ha mai fatto il login con la password non può usare il PIN
	c.device().must("POST", "/api/pin/login", map[string]string{"pin": "4821"}, 401)

	c.must("POST", "/api/pin/remove", map[string]string{"password": "segreta1"}, 204)
	c.must("POST", "/api/logout", nil, 204)
	c.must("POST", "/api/pin/login", map[string]string{"pin": "4821"}, 401)
}

func TestPinLockoutRequiresPassword(t *testing.T) {
	c := newServer(t)
	c.must("POST", "/api/setup", map[string]string{"password": "segreta1"}, 204)
	c.must("POST", "/api/pin/set", map[string]string{"password": "segreta1", "pin": "4821"}, 204)
	c.must("POST", "/api/logout", nil, 204)
	for i := 0; i < maxPinFails; i++ {
		c.must("POST", "/api/pin/login", map[string]string{"pin": "1111"}, 401)
	}
	// dopo 5 errori il dispositivo è dimenticato: nemmeno il PIN giusto funziona più
	c.must("POST", "/api/pin/login", map[string]string{"pin": "4821"}, 401)
	if st := c.must("GET", "/api/status", nil, 200); st["device"] != false {
		t.Fatalf("il dispositivo doveva essere revocato: %v", st)
	}
	c.must("POST", "/api/login", map[string]string{"password": "segreta1"}, 204)
	c.must("GET", "/api/accounts", nil, 200)
}

func TestPasswordChangeRevokesDevices(t *testing.T) {
	c := newServer(t)
	c.must("POST", "/api/setup", map[string]string{"password": "segreta1"}, 204)
	old := c.cookies[deviceCookie].Value
	c.must("POST", "/api/password", map[string]string{"current": "segreta1", "new": "nuova-pw-2"}, 204)
	if c.cookies[deviceCookie].Value == old {
		t.Error("dopo il cambio password il dispositivo deve essere nuovo")
	}
	c.must("POST", "/api/login", map[string]string{"password": "segreta1"}, 401)
	c.must("POST", "/api/login", map[string]string{"password": "nuova-pw-2"}, 204)
}

func TestChallengeSingleUse(t *testing.T) {
	ch := challenges{m: map[string]time.Time{}}
	v := ch.new()
	if !ch.take(v) {
		t.Error("la challenge appena creata deve valere")
	}
	if ch.take(v) {
		t.Error("la challenge deve valere una volta sola")
	}
	if ch.take("inventata") {
		t.Error("una challenge sconosciuta non deve valere")
	}
	ch.m["scaduta"] = time.Now().Add(-time.Second)
	if ch.take("scaduta") {
		t.Error("una challenge scaduta non deve valere")
	}
}

// --- autenticatore software per WebAuthn ---

func cborHead(major byte, n int) []byte {
	switch {
	case n < 24:
		return []byte{major<<5 | byte(n)}
	case n < 256:
		return []byte{major<<5 | 24, byte(n)}
	default:
		return []byte{major<<5 | 25, byte(n >> 8), byte(n)}
	}
}
func cborBytes(b []byte) []byte { return append(cborHead(2, len(b)), b...) }
func cborText(s string) []byte  { return append(cborHead(3, len(s)), s...) }
func cborInt(n int64) []byte {
	if n >= 0 {
		return cborHead(0, int(n))
	}
	return cborHead(1, int(-1-n))
}

type authenticator struct {
	key    *ecdsa.PrivateKey
	credID []byte
	count  uint32
}

func (a *authenticator) cose() []byte {
	x, y := a.key.X.FillBytes(make([]byte, 32)), a.key.Y.FillBytes(make([]byte, 32))
	b := []byte{0xa5}
	for _, part := range [][]byte{
		cborInt(1), cborInt(2), cborInt(3), cborInt(-7), cborInt(-1), cborInt(1),
		cborInt(-2), cborBytes(x), cborInt(-3), cborBytes(y),
	} {
		b = append(b, part...)
	}
	return b
}

func authDataFor(rp string, flags byte, count uint32, attested []byte) []byte {
	h := sha256.Sum256([]byte(rp))
	ad := append([]byte{}, h[:]...)
	ad = append(ad, flags)
	ad = binary.BigEndian.AppendUint32(ad, count)
	return append(ad, attested...)
}

func clientDataJSON(typ, challenge, origin string) []byte {
	b, _ := json.Marshal(map[string]any{"type": typ, "challenge": challenge, "origin": origin, "crossOrigin": false})
	return b
}

func (a *authenticator) register(c *client, origin string, flags byte) int {
	opts := c.must("POST", "/api/bio/register/begin", nil, 200)
	ch := opts["challenge"].(string)
	att := make([]byte, 16) // aaguid
	att = append(att, byte(len(a.credID)>>8), byte(len(a.credID)))
	att = append(att, a.credID...)
	att = append(att, a.cose()...)
	ad := authDataFor(testHost, flags, 0, att)
	obj := []byte{0xa3}
	for _, part := range [][]byte{
		cborText("fmt"), cborText("none"), cborText("attStmt"), {0xa0}, cborText("authData"), cborBytes(ad),
	} {
		obj = append(obj, part...)
	}
	code, _ := c.do("POST", "/api/bio/register/finish", map[string]string{
		"clientDataJSON":    b64.EncodeToString(clientDataJSON("webauthn.create", ch, origin)),
		"attestationObject": b64.EncodeToString(obj),
	})
	return code
}

func (a *authenticator) assert(t *testing.T, c *client, origin string, tamper bool) int {
	opts := c.must("POST", "/api/bio/login/begin", nil, 200)
	ch := opts["challenge"].(string)
	a.count++
	ad := authDataFor(testHost, flagUP|flagUV, a.count, nil)
	cdj := clientDataJSON("webauthn.get", ch, origin)
	h := sha256.Sum256(cdj)
	digest := sha256.Sum256(append(append([]byte{}, ad...), h[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	if tamper {
		sig[len(sig)-1] ^= 0xff
	}
	code, _ := c.do("POST", "/api/bio/login/finish", map[string]string{
		"id": b64.EncodeToString(a.credID), "clientDataJSON": b64.EncodeToString(cdj),
		"authenticatorData": b64.EncodeToString(ad), "signature": b64.EncodeToString(sig),
	})
	return code
}

func TestBiometricFlow(t *testing.T) {
	c := newServer(t)
	c.must("POST", "/api/setup", map[string]string{"password": "segreta1"}, 204)
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	au := &authenticator{key: key, credID: []byte("credenziale-di-prova-0123456789")}
	origin := "https://" + testHost

	if code := au.register(c, "https://evil.example", flagUP|flagUV|flagAT); code != 400 {
		t.Errorf("registrazione con origin sbagliata: atteso 400, ottenuto %d", code)
	}
	if code := au.register(c, origin, flagUP|flagAT); code != 400 {
		t.Errorf("registrazione senza verifica utente: atteso 400, ottenuto %d", code)
	}
	if code := au.register(c, origin, flagUP|flagUV|flagAT); code != 204 {
		t.Fatalf("registrazione: %d", code)
	}
	if st := c.must("GET", "/api/status", nil, 200); st["bio"] != true {
		t.Fatalf("bio dovrebbe essere attiva: %v", st)
	}

	c.must("POST", "/api/logout", nil, 204)
	c.must("GET", "/api/accounts", nil, 401)

	if code := au.assert(t, c, origin, true); code != 401 {
		t.Errorf("firma alterata: atteso 401, ottenuto %d", code)
	}
	if code := au.assert(t, c, "https://evil.example", false); code != 401 {
		t.Errorf("origin sbagliata: atteso 401, ottenuto %d", code)
	}
	c.must("GET", "/api/accounts", nil, 401) // i tentativi falliti non aprono la sessione
	if code := au.assert(t, c, origin, false); code != 204 {
		t.Fatalf("sblocco biometrico: %d", code)
	}
	c.must("GET", "/api/accounts", nil, 200)

	// replay: un contatore che non cresce viene rifiutato
	c.must("POST", "/api/logout", nil, 204)
	au.count = 0
	if code := au.assert(t, c, origin, false); code != 401 {
		t.Errorf("replay del contatore: atteso 401, ottenuto %d", code)
	}

	// un altro dispositivo non ha credenziali
	other := c.device()
	other.must("POST", "/api/login", map[string]string{"password": "segreta1"}, 204)
	other.must("POST", "/api/logout", nil, 204)
	other.must("POST", "/api/bio/login/begin", nil, 401)

	// rimozione
	c.must("POST", "/api/login", map[string]string{"password": "segreta1"}, 204)
	c.must("POST", "/api/bio/remove", nil, 204)
	if st := c.must("GET", "/api/status", nil, 200); st["bio"] != false {
		t.Fatalf("bio dovrebbe essere disattivata: %v", st)
	}
}

func TestBiometricNeedsHTTPS(t *testing.T) {
	c := newServer(t)
	c.https = false
	c.must("POST", "/api/setup", map[string]string{"password": "segreta1"}, 204)
	c.must("POST", "/api/bio/register/begin", nil, 400)
}

func TestCBORRejectsGarbage(t *testing.T) {
	for _, in := range [][]byte{nil, {0x5f}, {0xbf}, {0x9b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, {0x42, 0x01}} {
		if _, _, err := cborItem(in, 0, 0); err == nil {
			t.Errorf("CBOR % x doveva essere rifiutato", in)
		}
	}
}
