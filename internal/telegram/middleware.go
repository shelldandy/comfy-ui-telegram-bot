package telegram

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Whitelist manages allowed user IDs
type Whitelist struct {
	allowed map[int64]struct{}
	logger  *slog.Logger
}

// NewWhitelist creates a new whitelist from a slice of user IDs
func NewWhitelist(userIDs []int64, logger *slog.Logger) *Whitelist {
	allowed := make(map[int64]struct{}, len(userIDs))
	for _, id := range userIDs {
		allowed[id] = struct{}{}
	}
	return &Whitelist{
		allowed: allowed,
		logger:  logger,
	}
}

// IsAllowed checks if a user is whitelisted
func (w *Whitelist) IsAllowed(userID int64) bool {
	_, ok := w.allowed[userID]
	return ok
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
