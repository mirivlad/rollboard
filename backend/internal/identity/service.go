package identity

import (
	"context"
	"time"
)

// Service is the identity contract used by the HTTP transport.
// Repository implements it with PostgreSQL-backed storage.
type Service interface {
	Register(context.Context, RegistrationInput) (User, error)
	Authenticate(context.Context, string, string) (User, error)
	CreateGuest(context.Context, string) (Guest, error)
	CreateGuestSession(context.Context, string, time.Time) (Session, string, error)
	CreateUserSession(context.Context, string, time.Time) (Session, string, error)
	LookupSession(context.Context, string, time.Time) (*Actor, error)
	DeleteSession(context.Context, string) error
}
