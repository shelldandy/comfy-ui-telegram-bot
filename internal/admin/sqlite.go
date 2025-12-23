package admin

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite for persistence
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-backed admin store
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

	// Create approved_users table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS approved_users (
			user_id INTEGER PRIMARY KEY,
			username TEXT,
			approved_at DATETIME NOT NULL,
			approved_by INTEGER NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create approved_users table: %w", err)
	}

	// Create pending_requests table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS pending_requests (
			user_id INTEGER PRIMARY KEY,
			username TEXT,
			first_name TEXT,
			chat_id INTEGER NOT NULL,
			requested_at DATETIME NOT NULL,
			notified_at DATETIME,
			admin_msg_id INTEGER
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create pending_requests table: %w", err)
	}

	// Create approved_groups table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS approved_groups (
			group_id INTEGER PRIMARY KEY,
			title TEXT,
			approved_at DATETIME NOT NULL,
			approved_by INTEGER NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create approved_groups table: %w", err)
	}

	// Create pending_group_requests table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS pending_group_requests (
			group_id INTEGER PRIMARY KEY,
			title TEXT,
			requested_at DATETIME NOT NULL,
			notified_at DATETIME,
			admin_msg_id INTEGER
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create pending_group_requests table: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// IsApproved checks if a user has been approved
func (s *SQLiteStore) IsApproved(userID int64) (bool, error) {
	var exists int
	err := s.db.QueryRow(
		"SELECT 1 FROM approved_users WHERE user_id = ?",
		userID,
	).Scan(&exists)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check approved status: %w", err)
	}
	return true, nil
}

// AddApproved adds a user to the approved list
func (s *SQLiteStore) AddApproved(user ApprovedUser) error {
	_, err := s.db.Exec(`
		INSERT INTO approved_users (user_id, username, approved_at, approved_by)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			username = excluded.username,
			approved_at = excluded.approved_at,
			approved_by = excluded.approved_by
	`, user.UserID, user.Username, user.ApprovedAt, user.ApprovedBy)

	if err != nil {
		return fmt.Errorf("add approved user: %w", err)
	}
	return nil
}

// RemoveApproved removes a user from the approved list
func (s *SQLiteStore) RemoveApproved(userID int64) error {
	_, err := s.db.Exec("DELETE FROM approved_users WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("remove approved user: %w", err)
	}
	return nil
}

// GetPending retrieves a pending request by user ID
func (s *SQLiteStore) GetPending(userID int64) (*PendingRequest, error) {
	var req PendingRequest
	var notifiedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT user_id, username, first_name, chat_id, requested_at, notified_at, admin_msg_id
		FROM pending_requests WHERE user_id = ?
	`, userID).Scan(
		&req.UserID,
		&req.Username,
		&req.FirstName,
		&req.ChatID,
		&req.RequestedAt,
		&notifiedAt,
		&req.AdminMsgID,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pending request: %w", err)
	}

	if notifiedAt.Valid {
		req.NotifiedAt = &notifiedAt.Time
	}

	return &req, nil
}

// AddPending adds a new pending request
func (s *SQLiteStore) AddPending(req PendingRequest) error {
	_, err := s.db.Exec(`
		INSERT INTO pending_requests (user_id, username, first_name, chat_id, requested_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			username = excluded.username,
			first_name = excluded.first_name,
			chat_id = excluded.chat_id,
			requested_at = excluded.requested_at
	`, req.UserID, req.Username, req.FirstName, req.ChatID, req.RequestedAt)

	if err != nil {
		return fmt.Errorf("add pending request: %w", err)
	}
	return nil
}

// RemovePending removes a pending request
func (s *SQLiteStore) RemovePending(userID int64) error {
	_, err := s.db.Exec("DELETE FROM pending_requests WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("remove pending request: %w", err)
	}
	return nil
}

// UpdatePendingNotified marks a pending request as notified
func (s *SQLiteStore) UpdatePendingNotified(userID int64, msgID int) error {
	_, err := s.db.Exec(`
		UPDATE pending_requests
		SET notified_at = ?, admin_msg_id = ?
		WHERE user_id = ?
	`, time.Now(), msgID, userID)

	if err != nil {
		return fmt.Errorf("update pending notified: %w", err)
	}
	return nil
}

// Close releases database resources
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// IsGroupApproved checks if a group has been approved
func (s *SQLiteStore) IsGroupApproved(groupID int64) (bool, error) {
	var exists int
	err := s.db.QueryRow(
		"SELECT 1 FROM approved_groups WHERE group_id = ?",
		groupID,
	).Scan(&exists)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check group approved status: %w", err)
	}
	return true, nil
}

// AddApprovedGroup adds a group to the approved list
func (s *SQLiteStore) AddApprovedGroup(group ApprovedGroup) error {
	_, err := s.db.Exec(`
		INSERT INTO approved_groups (group_id, title, approved_at, approved_by)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(group_id) DO UPDATE SET
			title = excluded.title,
			approved_at = excluded.approved_at,
			approved_by = excluded.approved_by
	`, group.GroupID, group.Title, group.ApprovedAt, group.ApprovedBy)

	if err != nil {
		return fmt.Errorf("add approved group: %w", err)
	}
	return nil
}

// RemoveApprovedGroup removes a group from the approved list
func (s *SQLiteStore) RemoveApprovedGroup(groupID int64) error {
	_, err := s.db.Exec("DELETE FROM approved_groups WHERE group_id = ?", groupID)
	if err != nil {
		return fmt.Errorf("remove approved group: %w", err)
	}
	return nil
}

// GetPendingGroup retrieves a pending group request by group ID
func (s *SQLiteStore) GetPendingGroup(groupID int64) (*PendingGroupRequest, error) {
	var req PendingGroupRequest
	var notifiedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT group_id, title, requested_at, notified_at, admin_msg_id
		FROM pending_group_requests WHERE group_id = ?
	`, groupID).Scan(
		&req.GroupID,
		&req.Title,
		&req.RequestedAt,
		&notifiedAt,
		&req.AdminMsgID,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pending group request: %w", err)
	}

	if notifiedAt.Valid {
		req.NotifiedAt = &notifiedAt.Time
	}

	return &req, nil
}

// AddPendingGroup adds a new pending group request
func (s *SQLiteStore) AddPendingGroup(req PendingGroupRequest) error {
	_, err := s.db.Exec(`
		INSERT INTO pending_group_requests (group_id, title, requested_at)
		VALUES (?, ?, ?)
		ON CONFLICT(group_id) DO UPDATE SET
			title = excluded.title,
			requested_at = excluded.requested_at
	`, req.GroupID, req.Title, req.RequestedAt)

	if err != nil {
		return fmt.Errorf("add pending group request: %w", err)
	}
	return nil
}

// RemovePendingGroup removes a pending group request
func (s *SQLiteStore) RemovePendingGroup(groupID int64) error {
	_, err := s.db.Exec("DELETE FROM pending_group_requests WHERE group_id = ?", groupID)
	if err != nil {
		return fmt.Errorf("remove pending group request: %w", err)
	}
	return nil
}

// UpdatePendingGroupNotified marks a pending group request as notified
func (s *SQLiteStore) UpdatePendingGroupNotified(groupID int64, msgID int) error {
	_, err := s.db.Exec(`
		UPDATE pending_group_requests
		SET notified_at = ?, admin_msg_id = ?
		WHERE group_id = ?
	`, time.Now(), msgID, groupID)

	if err != nil {
		return fmt.Errorf("update pending group notified: %w", err)
	}
	return nil
}
