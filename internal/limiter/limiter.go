package limiter

import (
	"sync"
)

// UserLimiter limits concurrent requests per user
type UserLimiter struct {
	mu          sync.Mutex
	activeUsers map[int64]struct{}
	maxGlobal   int
	globalCount int
}

// NewUserLimiter creates a new user limiter
// maxGlobalConcurrent of 0 means unlimited global concurrent requests
func NewUserLimiter(maxGlobalConcurrent int) *UserLimiter {
	return &UserLimiter{
		activeUsers: make(map[int64]struct{}),
		maxGlobal:   maxGlobalConcurrent,
	}
}

// TryAcquire attempts to acquire a slot for a user
// Returns false if user already has an active request or global limit reached
func (l *UserLimiter) TryAcquire(userID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if user already has an active request
	if _, exists := l.activeUsers[userID]; exists {
		return false
	}

	// Check global limit (0 means unlimited)
	if l.maxGlobal > 0 && l.globalCount >= l.maxGlobal {
		return false
	}

	l.activeUsers[userID] = struct{}{}
	l.globalCount++
	return true
}

// Release releases a user's slot
func (l *UserLimiter) Release(userID int64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.activeUsers[userID]; exists {
		delete(l.activeUsers, userID)
		l.globalCount--
	}
}

// ActiveCount returns current active generation count
func (l *UserLimiter) ActiveCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.globalCount
}

// IsUserActive checks if a user has an active request
func (l *UserLimiter) IsUserActive(userID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, exists := l.activeUsers[userID]
	return exists
}
