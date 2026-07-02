package generations

import "time"

// Record holds the metadata needed to service the inline action buttons
// attached to a generated image: the original prompt and the ComfyUI file
// reference used to re-download the full-resolution image on demand.
type Record struct {
	ID        string
	UserID    int64
	Prompt    string
	Filename  string
	Subfolder string
	ImgType   string
	CreatedAt time.Time
}

// Store defines the interface for generation persistence
type Store interface {
	// Save persists a generation record
	Save(rec *Record) error

	// Get retrieves a generation record by ID, returning (nil, nil) if not found
	Get(id string) (*Record, error)

	// Close releases resources
	Close() error
}
