package application

import (
	"context"
	"errors"
	"testing"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

// newTestAuthService returns an AuthService and a UserService that share one
// fake repository.
func newTestAuthService(t *testing.T) (*AuthService, *UserService, *fakeHasher, *fakeTokens) {
	t.Helper()
	repo := newFakeUserRepo()
	hasher := &fakeHasher{}
	tokens := &fakeTokens{}
	auth, err := NewAuthService(repo, hasher, tokens)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	return auth, NewUserService(repo, hasher), hasher, tokens
}

func TestAuthService_Login(t *testing.T) {
	t.Run("returns token for valid credentials", func(t *testing.T) {
		auth, users, _, _ := newTestAuthService(t)
		alice := seedUser(t, users, "Alice", "alice@example.com")

		token, err := auth.Login(context.Background(), LoginInput{Email: " ALICE@example.com", Password: "password123"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token.Value != "token-for-"+alice.ID {
			t.Errorf("token = %q, want token for %s", token.Value, alice.ID)
		}
		if !token.ExpiresAt.Equal(fakeTokenExpiry) {
			t.Errorf("ExpiresAt = %v, want %v", token.ExpiresAt, fakeTokenExpiry)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		auth, users, _, _ := newTestAuthService(t)
		seedUser(t, users, "Alice", "alice@example.com")

		_, err := auth.Login(context.Background(), LoginInput{Email: "alice@example.com", Password: "wrong-password"})
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("err = %v, want ErrInvalidCredentials", err)
		}
	})

	t.Run("unknown email gives the same error and still compares a hash", func(t *testing.T) {
		auth, _, hasher, _ := newTestAuthService(t)

		_, err := auth.Login(context.Background(), LoginInput{Email: "nobody@example.com", Password: "password123"})
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("err = %v, want ErrInvalidCredentials", err)
		}
		if hasher.compareCalls != 1 {
			t.Errorf("Compare called %d times, want 1 (timing equalization)", hasher.compareCalls)
		}
	})

	t.Run("token issuing failure is returned", func(t *testing.T) {
		auth, users, _, tokens := newTestAuthService(t)
		seedUser(t, users, "Alice", "alice@example.com")
		tokens.issueErr = errors.New("signing failed")

		_, err := auth.Login(context.Background(), LoginInput{Email: "alice@example.com", Password: "password123"})
		if !errors.Is(err, tokens.issueErr) {
			t.Fatalf("err = %v, want wrapped issue error", err)
		}
	})
}

func TestAuthService_Authenticate(t *testing.T) {
	auth, _, _, _ := newTestAuthService(t)

	t.Run("valid token returns user ID", func(t *testing.T) {
		userID, err := auth.Authenticate(context.Background(), "token-for-user-42")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if userID != "user-42" {
			t.Errorf("userID = %q, want %q", userID, "user-42")
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := auth.Authenticate(context.Background(), "garbage")
		if !errors.Is(err, domain.ErrInvalidToken) {
			t.Fatalf("err = %v, want ErrInvalidToken", err)
		}
	})
}

func TestNewAuthService_HasherFailure(t *testing.T) {
	_, err := NewAuthService(newFakeUserRepo(), &fakeHasher{hashErr: errors.New("boom")}, &fakeTokens{})
	if err == nil {
		t.Fatal("expected error when the dummy hash cannot be created")
	}
}
