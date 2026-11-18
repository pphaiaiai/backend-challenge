package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

// LoginInput holds login credentials.
type LoginInput struct {
	Email    string
	Password string
}

// AuthService handles login and token verification.
type AuthService struct {
	users  UserRepository
	hasher PasswordHasher
	tokens TokenService

	// dummyHash is compared when the email is unknown, so that both login
	// failure paths take a similar amount of time.
	dummyHash string
}

// NewAuthService creates an AuthService.
func NewAuthService(users UserRepository, hasher PasswordHasher, tokens TokenService) (*AuthService, error) {
	dummyHash, err := hasher.Hash("timing-equalization-dummy-password")
	if err != nil {
		return nil, fmt.Errorf("prepare dummy hash: %w", err)
	}
	return &AuthService{users: users, hasher: hasher, tokens: tokens, dummyHash: dummyHash}, nil
}

// Login returns an access token for valid credentials. An unknown email and
// a wrong password both return domain.ErrInvalidCredentials.
func (s *AuthService) Login(ctx context.Context, in LoginInput) (Token, error) {
	email := domain.NormalizeEmail(in.Email)

	user, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, domain.ErrUserNotFound) {
		_ = s.hasher.Compare(s.dummyHash, in.Password)
		return Token{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return Token{}, fmt.Errorf("find user by email: %w", err)
	}

	if err := s.hasher.Compare(user.PasswordHash, in.Password); err != nil {
		return Token{}, domain.ErrInvalidCredentials
	}

	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		return Token{}, fmt.Errorf("issue token: %w", err)
	}
	return token, nil
}

// Authenticate verifies a token and returns the user ID it was issued for.
func (s *AuthService) Authenticate(_ context.Context, token string) (string, error) {
	userID, err := s.tokens.Verify(token)
	if err != nil {
		return "", domain.ErrInvalidToken
	}
	return userID, nil
}
