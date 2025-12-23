package telegram

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"comfy-tg-bot/internal/admin"
)

// Whitelist manages allowed user IDs
type Whitelist struct {
	staticAllowed map[int64]struct{}
	adminStore    admin.Store
	adminUserID   int64
	logger        *slog.Logger
}

// NewWhitelist creates a new whitelist from a slice of user IDs
func NewWhitelist(userIDs []int64, adminStore admin.Store, adminUserID int64, logger *slog.Logger) *Whitelist {
	allowed := make(map[int64]struct{}, len(userIDs))
	for _, id := range userIDs {
		allowed[id] = struct{}{}
	}
	return &Whitelist{
		staticAllowed: allowed,
		adminStore:    adminStore,
		adminUserID:   adminUserID,
		logger:        logger,
	}
}

// IsAllowed checks if a user is whitelisted (static or dynamically approved)
func (w *Whitelist) IsAllowed(userID int64) bool {
	// Check static list first (fastest)
	if _, ok := w.staticAllowed[userID]; ok {
		return true
	}

	// Check if user is admin
	if userID == w.adminUserID && w.adminUserID != 0 {
		return true
	}

	// Check dynamic approved users from database
	if w.adminStore != nil {
		approved, err := w.adminStore.IsApproved(userID)
		if err != nil {
			w.logger.Error("failed to check approved status", "error", err, "user_id", userID)
			return false
		}
		return approved
	}

	return false
}

// IsAdmin checks if a user is the admin
func (w *Whitelist) IsAdmin(userID int64) bool {
	return w.adminUserID != 0 && userID == w.adminUserID
}

// AdminUserID returns the admin user ID
func (w *Whitelist) AdminUserID() int64 {
	return w.adminUserID
}

// CheckAccess validates user access and logs unauthorized attempts
// Returns the user ID and whether access is granted
func (w *Whitelist) CheckAccess(update tgbotapi.Update) (int64, bool) {
	var userID int64
	var username string

	if update.Message != nil && update.Message.From != nil {
		userID = update.Message.From.ID
		username = update.Message.From.UserName
	} else if update.CallbackQuery != nil && update.CallbackQuery.From != nil {
		userID = update.CallbackQuery.From.ID
		username = update.CallbackQuery.From.UserName
	} else {
		return 0, false
	}

	if !w.IsAllowed(userID) {
		w.logger.Warn("unauthorized access attempt",
			"user_id", userID,
			"username", username,
		)
		return userID, false
	}

	return userID, true
}
