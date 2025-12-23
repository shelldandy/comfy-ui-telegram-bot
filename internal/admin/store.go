package admin

import "time"

// ApprovedUser represents a dynamically approved user
type ApprovedUser struct {
	UserID     int64
	Username   string
	ApprovedAt time.Time
	ApprovedBy int64
}

// PendingRequest tracks users waiting for admin approval
type PendingRequest struct {
	UserID      int64
	Username    string
	FirstName   string
	ChatID      int64
	RequestedAt time.Time
	NotifiedAt  *time.Time
	AdminMsgID  int
}

// ApprovedGroup represents a dynamically approved group
type ApprovedGroup struct {
	GroupID    int64
	Title      string
	ApprovedAt time.Time
	ApprovedBy int64
}

// PendingGroupRequest tracks groups waiting for admin approval
type PendingGroupRequest struct {
	GroupID     int64
	Title       string
	RequestedAt time.Time
	NotifiedAt  *time.Time
	AdminMsgID  int
}

// Store defines the interface for admin persistence
type Store interface {
	// IsApproved checks if a user has been approved
	IsApproved(userID int64) (bool, error)

	// AddApproved adds a user to the approved list
	AddApproved(user ApprovedUser) error

	// RemoveApproved removes a user from the approved list
	RemoveApproved(userID int64) error

	// GetPending retrieves a pending request by user ID
	GetPending(userID int64) (*PendingRequest, error)

	// AddPending adds a new pending request
	AddPending(req PendingRequest) error

	// RemovePending removes a pending request
	RemovePending(userID int64) error

	// UpdatePendingNotified marks a pending request as notified
	UpdatePendingNotified(userID int64, msgID int) error

	// IsGroupApproved checks if a group has been approved
	IsGroupApproved(groupID int64) (bool, error)

	// AddApprovedGroup adds a group to the approved list
	AddApprovedGroup(group ApprovedGroup) error

	// RemoveApprovedGroup removes a group from the approved list
	RemoveApprovedGroup(groupID int64) error

	// GetPendingGroup retrieves a pending group request by group ID
	GetPendingGroup(groupID int64) (*PendingGroupRequest, error)

	// AddPendingGroup adds a new pending group request
	AddPendingGroup(req PendingGroupRequest) error

	// RemovePendingGroup removes a pending group request
	RemovePendingGroup(groupID int64) error

	// UpdatePendingGroupNotified marks a pending group request as notified
	UpdatePendingGroupNotified(groupID int64, msgID int) error

	// Close releases resources
	Close() error
}
