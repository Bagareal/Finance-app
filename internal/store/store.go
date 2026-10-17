// Package store contiene lo schema SQLite e tutta la logica di dominio.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrInvalid indica un input non valido; il messaggio è mostrabile all'utente.
type ErrInvalid string

func (e ErrInvalid) Error() string { return string(e) }

func invalid(format string, a ...any) error { return ErrInvalid(fmt.Sprintf(format, a...)) }

var ErrNotFound = errors.New("non trovato")

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS sessions (token TEXT PRIMARY KEY, expires INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS accounts (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NOT NULL DEFAULT 'corrente',
  initial_cents INTEGER NOT NULL DEFAULT 0,
  archived INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS categories (
  id INTEGER PRIMARY KEY,
  parent_id INTEGER REFERENCES categories(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('income','expense')),
  color TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS transactions (
  id INTEGER PRIMARY KEY,
  date TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('income','expense','transfer')),
  amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
  account_id INTEGER NOT NULL REFERENCES accounts(id),
  to_account_id INTEGER REFERENCES accounts(id),
  category_id INTEGER REFERENCES categories(id),
  note TEXT NOT NULL DEFAULT '',
  recurring_id INTEGER
);
CREATE INDEX IF NOT EXISTS tx_date ON transactions(date);
CREATE INDEX IF NOT EXISTS tx_account ON transactions(account_id);
CREATE TABLE IF NOT EXISTS recurring (
  id INTEGER PRIMARY KEY,
  kind TEXT NOT NULL CHECK (kind IN ('income','expense','transfer')),
  amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
  account_id INTEGER NOT NULL REFERENCES accounts(id),
  to_account_id INTEGER REFERENCES accounts(id),
  category_id INTEGER REFERENCES categories(id),
  note TEXT NOT NULL DEFAULT '',
  freq TEXT NOT NULL CHECK (freq IN ('weekly','monthly','yearly')),
  start_date TEXT NOT NULL,
  end_date TEXT NOT NULL DEFAULT '',
  next_date TEXT NOT NULL,
  active INTEGER NOT NULL DEFAULT 1
);
`

// Open apre (o crea) il database in path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // app per un solo utente: evita problemi di lock
	if _, err := db.Exec(schema + devicesSchema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// BackupTo scrive una copia consistente del database in dest.
func (s *Store) BackupTo(dest string) error {
	_, err := s.db.Exec("VACUUM INTO ?", dest)
	return err
}

func parseDate(d string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return t, invalid("data non valida: %q (atteso AAAA-MM-GG)", d)
	}
	return t, nil
}

func today() string { return time.Now().Format("2006-01-02") }

// --- settings / sessions ---

func (s *Store) GetSetting(key string) (string, bool) {
	var v string
	if err := s.db.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&v); err != nil {
		return "", false
	}
	return v, true
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}

func (s *Store) DeleteSetting(key string) { s.db.Exec("DELETE FROM settings WHERE key=?", key) }

func (s *Store) CreateSession(token string, expires time.Time) error {
	s.db.Exec("DELETE FROM sessions WHERE expires < ?", time.Now().Unix())
	_, err := s.db.Exec("INSERT INTO sessions(token,expires) VALUES(?,?)", token, expires.Unix())
	return err
}

func (s *Store) SessionValid(token string) bool {
	var exp int64
	if err := s.db.QueryRow("SELECT expires FROM sessions WHERE token=?", token).Scan(&exp); err != nil {
		return false
	}
	return exp > time.Now().Unix()
}

func (s *Store) DeleteSession(token string) { s.db.Exec("DELETE FROM sessions WHERE token=?", token) }

func (s *Store) DeleteAllSessions() { s.db.Exec("DELETE FROM sessions") }
