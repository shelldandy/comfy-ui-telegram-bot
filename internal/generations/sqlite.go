package generations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite for persistence
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-backed generations store
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
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

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS generations (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			prompt TEXT NOT NULL,
			filename TEXT NOT NULL,
			subfolder TEXT,
			img_type TEXT,
			created_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create generations table: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Save persists a generation record
func (s *SQLiteStore) Save(rec *Record) error {
	_, err := s.db.Exec(`
		INSERT INTO generations (id, user_id, prompt, filename, subfolder, img_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			user_id = excluded.user_id,
			prompt = excluded.prompt,
			filename = excluded.filename,
			subfolder = excluded.subfolder,
			img_type = excluded.img_type,
			created_at = excluded.created_at
	`, rec.ID, rec.UserID, rec.Prompt, rec.Filename, rec.Subfolder, rec.ImgType, rec.CreatedAt)

	if err != nil {
		return fmt.Errorf("save generation: %w", err)
	}
	return nil
}

// Get retrieves a generation record by ID
func (s *SQLiteStore) Get(id string) (*Record, error) {
	var rec Record
	err := s.db.QueryRow(`
		SELECT id, user_id, prompt, filename, subfolder, img_type, created_at
		FROM generations WHERE id = ?
	`, id).Scan(
		&rec.ID,
		&rec.UserID,
		&rec.Prompt,
		&rec.Filename,
		&rec.Subfolder,
		&rec.ImgType,
		&rec.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get generation: %w", err)
	}

	return &rec, nil
}

// Close releases database resources
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
