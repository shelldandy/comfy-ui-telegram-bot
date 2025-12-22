package settings

import "errors"

// ErrAtLeastOneRequired indicates that at least one image format must be enabled
var ErrAtLeastOneRequired = errors.New("at least one of send_original or send_compressed must be enabled")

// UserSettings represents per-user configuration
type UserSettings struct {
	UserID         int64
	SendOriginal   bool
	SendCompressed bool
}

// Validate ensures settings are valid
func (s *UserSettings) Validate() error {
	if !s.SendOriginal && !s.SendCompressed {
		return ErrAtLeastOneRequired
	}
	return nil
}

// Store defines the interface for settings persistence
type Store interface {
	// Get retrieves user settings, returning defaults if none exist
	Get(userID int64) (*UserSettings, error)
	// Save persists user settings
	Save(settings *UserSettings) error
	// Close releases resources
	Close() error
}

// DefaultSettings holds the global defaults from config
type DefaultSettings struct {
	SendOriginal   bool
	SendCompressed bool
}
