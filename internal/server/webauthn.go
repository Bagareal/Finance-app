package server

// Sblocco biometrico con WebAuthn (autenticatore della piattaforma: impronta, Face ID, Windows Hello).
// Implementazione minimale, solo per l'uso di questa app: attestazione "none", algoritmi ES256 e RS256,
// verifica di challenge, origin, rpIdHash, flag UP/UV e firma. Funziona solo su HTTPS (contesto sicuro).

import (
	"bytes"
	"crypto"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Bagareal/Finance-app/internal/store"
)

const (
	algES256   = -7
	algRS256   = -257
	flagUP     = 0x01
	flagUV     = 0x04
	flagAT     = 0x40
	challengeT = 2 * time.Minute
)

var b64 = base64.RawURLEncoding

// --- challenge monouso, in memoria ---

type challenges struct {
	mu sync.Mutex
	m  map[string]time.Time
}

func (c *challenges) new() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, exp := range c.m {
		if now.After(exp) {
			delete(c.m, k)
		}
	}
	ch := b64.EncodeToString([]byte(newToken()[:32]))
	c.m[ch] = now.Add(challengeT)
	return ch
}

// take consuma la challenge: vale una sola volta e solo se non è scaduta.
func (c *challenges) take(ch string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	exp, ok := c.m[ch]
	delete(c.m, ch)
	return ok && time.Now().Before(exp)
}

// --- host / origin ---

func requestHost(r *http.Request) string {
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		return strings.TrimSpace(strings.Split(h, ",")[0])
	}
	return r.Host
}

func rpID(r *http.Request) string {
	h := requestHost(r)
	if host, _, err := net.SplitHostPort(h); err == nil {
		return host
	}
	return h
}

func expectedOrigin(r *http.Request) string { return "https://" + requestHost(r) }

// --- CBOR minimale (solo ciò che serve a WebAuthn) ---

var errCBOR = errors.New("invalid CBOR")

func cborItem(b []byte, off, depth int) (any, int, error) {
	if depth > 8 || off >= len(b) {
		return nil, 0, errCBOR
	}
	major, info := b[off]>>5, b[off]&0x1f
	off++
	var arg uint64
	switch {
	case info < 24:
		arg = uint64(info)
	case info >= 24 && info <= 27:
		n := 1 << (info - 24)
		if off+n > len(b) {
			return nil, 0, errCBOR
		}
		for _, x := range b[off : off+n] {
			arg = arg<<8 | uint64(x)
		}
		off += n
	default:
		return nil, 0, errCBOR // lunghezze indefinite non supportate
	}
	switch major {
	case 0:
		if arg > 1<<62 {
			return nil, 0, errCBOR
		}
		return int64(arg), off, nil
	case 1:
		if arg > 1<<62 {
			return nil, 0, errCBOR
		}
		return -1 - int64(arg), off, nil
	case 2, 3:
		if arg > uint64(len(b)-off) {
			return nil, 0, errCBOR
		}
		d := b[off : off+int(arg)]
		off += int(arg)
		if major == 3 {
			return string(d), off, nil
		}
		return append([]byte(nil), d...), off, nil
	case 4:
		if arg > 1000 {
			return nil, 0, errCBOR
		}
		out := make([]any, 0, arg)
		for i := uint64(0); i < arg; i++ {
			v, n, err := cborItem(b, off, depth+1)
			if err != nil {
				return nil, 0, err
			}
			out, off = append(out, v), n
		}
		return out, off, nil
	case 5:
		if arg > 1000 {
			return nil, 0, errCBOR
		}
		out := make(map[any]any, arg)
		for i := uint64(0); i < arg; i++ {
			k, n, err := cborItem(b, off, depth+1)
			if err != nil {
				return nil, 0, err
			}
			switch k.(type) {
			case int64, string:
			default:
				return nil, 0, errCBOR
			}
			v, n2, err := cborItem(b, n, depth+1)
			if err != nil {
				return nil, 0, err
			}
			out[k], off = v, n2
		}
		return out, off, nil
	case 7:
		switch info {
		case 20:
			return false, off, nil
		case 21:
			return true, off, nil
		case 22:
			return nil, off, nil
		}
	}
	return nil, 0, errCBOR
}

// --- authenticator data ---

type authData struct {
	rpIDHash  [32]byte
	flags     byte
	signCount uint32
	credID    []byte
	cose      []byte // chiave pubblica COSE (solo in registrazione)
}

func parseAuthData(b []byte, attested bool) (*authData, error) {
	if len(b) < 37 {
		return nil, errors.New("authData too short")
	}
	a := &authData{flags: b[32], signCount: binary.BigEndian.Uint32(b[33:37])}
	copy(a.rpIDHash[:], b[:32])
	if !attested {
		return a, nil
	}
	if a.flags&flagAT == 0 || len(b) < 55 {
		return nil, errors.New("missing credential")
	}
	idLen := int(binary.BigEndian.Uint16(b[53:55]))
	if len(b) < 55+idLen {
		return nil, errors.New("authData too short")
	}
	a.credID = append([]byte(nil), b[55:55+idLen]...)
	rest := b[55+idLen:]
	_, n, err := cborItem(rest, 0, 0)
	if err != nil {
		return nil, err
	}
	a.cose = append([]byte(nil), rest[:n]...)
	return a, nil
}

// --- chiavi pubbliche COSE ---

func intOf(v any) (int64, bool) { i, ok := v.(int64); return i, ok }

func parseCOSE(raw []byte) (crypto.PublicKey, int, error) {
	v, _, err := cborItem(raw, 0, 0)
	m, ok := v.(map[any]any)
	if err != nil || !ok {
		return nil, 0, errors.New("invalid COSE key")
	}
	kty, _ := intOf(m[int64(1)])
	alg, _ := intOf(m[int64(3)])
	switch {
	case alg == algES256 && kty == 2:
		crv, _ := intOf(m[int64(-1)])
		x, _ := m[int64(-2)].([]byte)
		y, _ := m[int64(-3)].([]byte)
		if crv != 1 || len(x) != 32 || len(y) != 32 {
			return nil, 0, errors.New("invalid EC key")
		}
		if _, err := ecdh.P256().NewPublicKey(append([]byte{4}, append(append([]byte{}, x...), y...)...)); err != nil {
			return nil, 0, errors.New("invalid EC point")
		}
		return &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, algES256, nil
	case alg == algRS256 && kty == 3:
		n, _ := m[int64(-1)].([]byte)
		e, _ := m[int64(-2)].([]byte)
		N := new(big.Int).SetBytes(n)
		E := new(big.Int).SetBytes(e)
		if N.BitLen() < 2048 || !E.IsInt64() || E.Int64() < 3 {
			return nil, 0, errors.New("invalid RSA key")
		}
		return &rsa.PublicKey{N: N, E: int(E.Int64())}, algRS256, nil
	}
	return nil, 0, errors.New("unsupported algorithm")
}

func verifySig(key crypto.PublicKey, alg int, signed, sig []byte) bool {
	h := sha256.Sum256(signed)
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		return alg == algES256 && ecdsa.VerifyASN1(k, h[:], sig)
	case *rsa.PublicKey:
		return alg == algRS256 && rsa.VerifyPKCS1v15(k, crypto.SHA256, h[:], sig) == nil
	}
	return false
}

// --- client data ---

type clientData struct {
	Type        string `json:"type"`
	Challenge   string `json:"challenge"`
	Origin      string `json:"origin"`
	CrossOrigin bool   `json:"crossOrigin"`
}

func (s *Server) checkClientData(r *http.Request, raw []byte, typ string) error {
	var cd clientData
	if err := json.Unmarshal(raw, &cd); err != nil {
		return badRequest("invalid biometric data")
	}
	if cd.Type != typ || cd.CrossOrigin || cd.Origin != expectedOrigin(r) {
		return badRequest("biometric request is not valid for this address")
	}
	if !s.chal.take(cd.Challenge) {
		return badRequest("biometric request expired: try again")
	}
	return nil
}

func checkAuthFlags(a *authData, r *http.Request) error {
	want := sha256.Sum256([]byte(rpID(r)))
	if !bytes.Equal(a.rpIDHash[:], want[:]) {
		return badRequest("domain mismatch")
	}
	if a.flags&flagUP == 0 || a.flags&flagUV == 0 {
		return badRequest("biometric verification was not performed")
	}
	return nil
}

func dec(s string) ([]byte, error) {
	b, err := b64.DecodeString(s)
	if err != nil {
		return nil, badRequest("invalid biometric data")
	}
	return b, nil
}

// --- endpoint ---

func (s *Server) bioRegisterBegin(w http.ResponseWriter, r *http.Request) (any, error) {
	if !isHTTPS(r) {
		return nil, badRequest("biometric unlock requires HTTPS")
	}
	dev := s.deviceToken(r)
	if dev == "" {
		return nil, badRequest("device not recognised: sign out and sign in with the password")
	}
	exclude := []map[string]string{}
	for _, c := range s.st.DeviceCredentials(dev) {
		exclude = append(exclude, map[string]string{"type": "public-key", "id": c.ID})
	}
	return map[string]any{
		"challenge": s.chal.new(),
		"rp":        map[string]string{"id": rpID(r), "name": "Finance App"},
		"user":      map[string]string{"id": b64.EncodeToString([]byte("owner")), "name": "utente", "displayName": "Finance App"},
		"pubKeyCredParams": []map[string]any{
			{"type": "public-key", "alg": algES256}, {"type": "public-key", "alg": algRS256},
		},
		"authenticatorSelection": map[string]string{"authenticatorAttachment": "platform", "userVerification": "required", "residentKey": "discouraged"},
		"attestation":            "none",
		"timeout":                60000,
		"excludeCredentials":     exclude,
	}, nil
}

func (s *Server) bioRegisterFinish(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct{ ClientDataJSON, AttestationObject string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	dev := s.deviceToken(r)
	if dev == "" {
		return nil, badRequest("device not recognised")
	}
	cdj, err := dec(in.ClientDataJSON)
	if err != nil {
		return nil, err
	}
	if err := s.checkClientData(r, cdj, "webauthn.create"); err != nil {
		return nil, err
	}
	att, err := dec(in.AttestationObject)
	if err != nil {
		return nil, err
	}
	v, _, cerr := cborItem(att, 0, 0)
	m, ok := v.(map[any]any)
	raw, _ := m["authData"].([]byte)
	if cerr != nil || !ok || raw == nil {
		return nil, badRequest("invalid attestation")
	}
	ad, err := parseAuthData(raw, true)
	if err != nil {
		return nil, badRequest("invalid attestation")
	}
	if err := checkAuthFlags(ad, r); err != nil {
		return nil, err
	}
	_, alg, err := parseCOSE(ad.cose)
	if err != nil {
		return nil, badRequest(err.Error())
	}
	err = s.st.AddCredential(store.Credential{ID: b64.EncodeToString(ad.credID), Device: dev, Alg: alg, Key: ad.cose, SignCount: ad.signCount})
	if err != nil {
		return nil, badRequest("credential already registered")
	}
	return nil, nil
}

func (s *Server) bioRemove(w http.ResponseWriter, r *http.Request) (any, error) {
	if dev := s.deviceToken(r); dev != "" {
		s.st.DeleteDeviceCredentials(dev)
	}
	return nil, nil
}

func (s *Server) bioLoginBegin(w http.ResponseWriter, r *http.Request) (any, error) {
	dev := s.deviceToken(r)
	if dev == "" || !isHTTPS(r) {
		return nil, httpError{http.StatusUnauthorized, "use the password"}
	}
	allow := []map[string]string{}
	for _, c := range s.st.DeviceCredentials(dev) {
		allow = append(allow, map[string]string{"type": "public-key", "id": c.ID})
	}
	if len(allow) == 0 {
		return nil, httpError{http.StatusUnauthorized, "biometric unlock is not enabled on this device"}
	}
	return map[string]any{"challenge": s.chal.new(), "rpId": rpID(r), "allowCredentials": allow, "userVerification": "required", "timeout": 60000}, nil
}

func (s *Server) bioLoginFinish(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct{ ID, ClientDataJSON, AuthenticatorData, Signature string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	fail := httpError{http.StatusUnauthorized, "biometric unlock failed"}
	dev := s.deviceToken(r)
	cred, err := s.st.GetCredential(in.ID)
	if dev == "" || err != nil || cred.Device != dev {
		return nil, fail
	}
	cdj, err1 := dec(in.ClientDataJSON)
	adRaw, err2 := dec(in.AuthenticatorData)
	sig, err3 := dec(in.Signature)
	if err1 != nil || err2 != nil || err3 != nil {
		return nil, fail
	}
	if err := s.checkClientData(r, cdj, "webauthn.get"); err != nil {
		return nil, fail
	}
	ad, err := parseAuthData(adRaw, false)
	if err != nil || checkAuthFlags(ad, r) != nil {
		return nil, fail
	}
	key, alg, err := parseCOSE(cred.Key)
	if err != nil {
		return nil, fail
	}
	ch := sha256.Sum256(cdj)
	if !verifySig(key, alg, append(append([]byte{}, adRaw...), ch[:]...), sig) {
		return nil, fail
	}
	// il contatore deve crescere (se l'autenticatore lo usa): protegge dalle credenziali clonate
	if (ad.signCount != 0 || cred.SignCount != 0) && ad.signCount <= cred.SignCount {
		return nil, fail
	}
	s.st.SetSignCount(cred.ID, ad.signCount)
	return nil, s.startSession(w, r)
}
