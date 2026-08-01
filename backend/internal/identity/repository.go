package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

var ErrEmailTaken = errors.New("email is already registered")

func (r *Repository) Register(ctx context.Context, input RegistrationInput) (User, error) {
	if err := input.Validate(); err != nil {
		return User{}, err
	}
	hash, err := HashPassword(input.Password)
	if err != nil {
		return User{}, err
	}
	var user User
	err = r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING id::text, email, display_name, created_at`, input.Email, hash, input.DisplayName).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.CreatedAt)
	if err == nil {
		return user, nil
	}
	if isUniqueViolation(err) {
		return User{}, ErrEmailTaken
	}
	return User{}, fmt.Errorf("insert user: %w", err)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func NewRepository(pool *pgxpool.Pool) (*Repository, error) {
	if pool == nil {
		return nil, fmt.Errorf("identity repository requires a PostgreSQL pool")
	}
	return &Repository{pool: pool}, nil
}
