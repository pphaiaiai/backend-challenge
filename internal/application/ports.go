// Package application implements the use cases. It depends on the domain and
// on the ports declared in this file; adapters provide the implementations.
package application

import (
	"context"
	"time"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

// UserRepository persists users.
type UserRepository interface {
	// Create stores the user and sets user.ID. It returns
	// domain.ErrEmailAlreadyExists if the email is taken.
	Create(ctx context.Context, user *domain.User) error

	// GetByID returns domain.ErrUserNotFound if the user does not exist.
	GetByID(ctx context.Context, id string) (*domain.User, error)

	// GetByEmail returns domain.ErrUserNotFound if the user does not exist.
	GetByEmail(ctx context.Context, email string) (*domain.User, error)

	// List returns up to limit users after skipping offset, oldest first.
	List(ctx context.Context, offset, limit int64) ([]*domain.User, error)

	// Update saves the user's name and email. It returns
	// domain.ErrUserNotFound or domain.ErrEmailAlreadyExists.
	Update(ctx context.Context, user *domain.User) error

	// Delete returns domain.ErrUserNotFound if the user does not exist.
	Delete(ctx context.Context, id string) error

	Count(ctx context.Context) (int64, error)
}

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	// Compare returns nil if password matches hash.
	Compare(hash, password string) error
}

// TokenService issues and verifies access tokens.
type TokenService interface {
	Issue(userID string) (Token, error)
	// Verify returns the user ID the token was issued for.
	Verify(token string) (userID string, err error)
}

// Token is a signed access token.
type Token struct {
	Value     string
	ExpiresAt time.Time
}
