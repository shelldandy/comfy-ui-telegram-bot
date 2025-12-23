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

// IsGroupAllowed checks if a group has been approved for bot usage
func (w *Whitelist) IsGroupAllowed(groupID int64) bool {
	if w.adminStore != nil {
		approved, err := w.adminStore.IsGroupApproved(groupID)
		if err != nil {
			w.logger.Error("failed to check group approved status", "error", err, "group_id", groupID)
			return false
		}
		return approved
	}
	return false
}

// CheckAccess validates access and returns context information
// Returns (userID, chatID, isGroup, allowed)
func (w *Whitelist) CheckAccess(update tgbotapi.Update) (userID int64, chatID int64, isGroup bool, allowed bool) {
	var username string

	if update.Message != nil {
		if update.Message.From != nil {
			userID = update.Message.From.ID
			username = update.Message.From.UserName
		}
		chatID = update.Message.Chat.ID
		isGroup = update.Message.Chat.IsGroup() || update.Message.Chat.IsSuperGroup()
	} else if update.CallbackQuery != nil && update.CallbackQuery.From != nil {
		userID = update.CallbackQuery.From.ID
		username = update.CallbackQuery.From.UserName
		if update.CallbackQuery.Message != nil {
			chatID = update.CallbackQuery.Message.Chat.ID
			isGroup = update.CallbackQuery.Message.Chat.IsGroup() ||
				update.CallbackQuery.Message.Chat.IsSuperGroup()
		}
	} else {
		return 0, 0, false, false
	}

	// For groups, check group approval (not individual user)
	if isGroup {
		if !w.IsGroupAllowed(chatID) {
			w.logger.Warn("unauthorized group access attempt",
				"group_id", chatID,
				"user_id", userID,
				"username", username,
			)
			return userID, chatID, true, false
		}
		return userID, chatID, true, true
	}

	// For private chats, use existing user whitelist logic
	if !w.IsAllowed(userID) {
		w.logger.Warn("unauthorized access attempt",
			"user_id", userID,
			"username", username,
		)
		return userID, chatID, false, false
	}

	return userID, chatID, false, true
}
