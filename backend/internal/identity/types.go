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
