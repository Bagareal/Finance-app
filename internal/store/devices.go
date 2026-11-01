package store

import (
	"database/sql"
	"time"
)

// Un "dispositivo" è un browser che ha fatto il login con la password almeno una volta.
// Solo i dispositivi noti possono sbloccare l'app con PIN o biometria.

const devicesSchema = `
CREATE TABLE IF NOT EXISTS devices (
  token TEXT PRIMARY KEY,
  created INTEGER NOT NULL,
  failures INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS credentials (
  id TEXT PRIMARY KEY,
  device TEXT NOT NULL REFERENCES devices(token) ON DELETE CASCADE,
  alg INTEGER NOT NULL,
  key BLOB NOT NULL,
  sign_count INTEGER NOT NULL DEFAULT 0
);
`

type Credential struct {
	ID        string
	Device    string
	Alg       int
	Key       []byte // chiave pubblica in formato COSE
	SignCount uint32
}

func (s *Store) CreateDevice(token string) error {
	_, err := s.db.Exec("INSERT INTO devices(token, created) VALUES(?,?)", token, time.Now().Unix())
	return err
}

func (s *Store) DeviceValid(token string) bool {
	if token == "" {
		return false
	}
	var n int
	s.db.QueryRow("SELECT COUNT(*) FROM devices WHERE token=?", token).Scan(&n)
	return n > 0
}

// DeviceFail registra un PIN sbagliato e restituisce il numero di errori consecutivi.
func (s *Store) DeviceFail(token string) int {
	s.db.Exec("UPDATE devices SET failures = failures + 1 WHERE token=?", token)
	var n int
	s.db.QueryRow("SELECT failures FROM devices WHERE token=?", token).Scan(&n)
	return n
}

func (s *Store) DeviceReset(token string) {
	s.db.Exec("UPDATE devices SET failures = 0 WHERE token=?", token)
}

// DeleteDevice dimentica il dispositivo (e le sue credenziali): servirà di nuovo la password.
func (s *Store) DeleteDevice(token string) {
	s.db.Exec("DELETE FROM devices WHERE token=?", token)
}

func (s *Store) DeleteAllDevices() { s.db.Exec("DELETE FROM devices") }

func (s *Store) AddCredential(c Credential) error {
	_, err := s.db.Exec("INSERT INTO credentials(id,device,alg,key,sign_count) VALUES(?,?,?,?,?)", c.ID, c.Device, c.Alg, c.Key, c.SignCount)
	return err
}

func (s *Store) GetCredential(id string) (*Credential, error) {
	var c Credential
	err := s.db.QueryRow("SELECT id,device,alg,key,sign_count FROM credentials WHERE id=?", id).Scan(&c.ID, &c.Device, &c.Alg, &c.Key, &c.SignCount)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *Store) DeviceCredentials(device string) []Credential {
	rows, err := s.db.Query("SELECT id,device,alg,key,sign_count FROM credentials WHERE device=?", device)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Credential
	for rows.Next() {
		var c Credential
		if rows.Scan(&c.ID, &c.Device, &c.Alg, &c.Key, &c.SignCount) == nil {
			out = append(out, c)
		}
	}
	return out
}

func (s *Store) SetSignCount(id string, n uint32) {
	s.db.Exec("UPDATE credentials SET sign_count=? WHERE id=?", n, id)
}

func (s *Store) DeleteDeviceCredentials(device string) {
	s.db.Exec("DELETE FROM credentials WHERE device=?", device)
}
