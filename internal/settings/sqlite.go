package settings

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite for persistence
type SQLiteStore struct {
	db       *sql.DB
	defaults DefaultSettings
}

// NewSQLiteStore creates a new SQLite-backed settings store
func NewSQLiteStore(dbPath string, defaults DefaultSettings) (*SQLiteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath+"?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite works best with a single writer
	db.SetMaxOpenConns(1)

	// Create table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_settings (
			user_id INTEGER PRIMARY KEY,
			send_original INTEGER NOT NULL DEFAULT 1,
			send_compressed INTEGER NOT NULL DEFAULT 1
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}

	return &SQLiteStore{db: db, defaults: defaults}, nil
}

// Get retrieves user settings, returning defaults if none exist
func (s *SQLiteStore) Get(userID int64) (*UserSettings, error) {
	var us UserSettings
	err := s.db.QueryRow(
		"SELECT user_id, send_original, send_compressed FROM user_settings WHERE user_id = ?",
		userID,
	).Scan(&us.UserID, &us.SendOriginal, &us.SendCompressed)

	if err == sql.ErrNoRows {
		// Return defaults for new users
		return &UserSettings{
			UserID:         userID,
			SendOriginal:   s.defaults.SendOriginal,
			SendCompressed: s.defaults.SendCompressed,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user settings: %w", err)
	}
	return &us, nil
}

// Save persists user settings using upsert
func (s *SQLiteStore) Save(us *UserSettings) error {
	if err := us.Validate(); err != nil {
		return err
	}

	_, err := s.db.Exec(`
		INSERT INTO user_settings (user_id, send_original, send_compressed)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			send_original = excluded.send_original,
			send_compressed = excluded.send_compressed
	`, us.UserID, us.SendOriginal, us.SendCompressed)

	if err != nil {
		return fmt.Errorf("save user settings: %w", err)
	}
	return nil
}

// Close releases database resources
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
