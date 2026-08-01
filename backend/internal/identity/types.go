package identity

import "time"

type User struct {
	ID           string
	Email        string
	DisplayName  string
	PasswordHash string
	CreatedAt    time.Time
}

type PublicUser struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (u User) Public() PublicUser {
	return PublicUser{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, CreatedAt: u.CreatedAt}
}

type Guest struct {
	ID              string
	DisplayName     string
	ClaimedByUserID *string
	CreatedAt       time.Time
	ClaimedAt       *time.Time
}

type Session struct {
	ID          string
	UserID      *string
	GuestID     *string
	TokenDigest []byte
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// Actor identifies the currently authenticated account or guest.
// Exactly one field is non-nil for a valid active session.
type Actor struct {
	SessionID string
	User      *User
	Guest     *Guest
}
