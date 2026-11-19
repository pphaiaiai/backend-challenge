package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

var fixedNow = time.Date(2026, 11, 19, 10, 30, 0, 123456789, time.UTC)

// newTestUserService returns a UserService with fakes and a fixed clock.
func newTestUserService() (*UserService, *fakeUserRepo, *fakeHasher) {
	repo := newFakeUserRepo()
	hasher := &fakeHasher{}
	svc := NewUserService(repo, hasher)
	svc.now = func() time.Time { return fixedNow }
	return svc, repo, hasher
}

// seedUser creates a user or fails the test.
func seedUser(t *testing.T, svc *UserService, name, email string) *domain.User {
	t.Helper()
	u, err := svc.Create(context.Background(), CreateUserInput{Name: name, Email: email, Password: "password123"})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func TestUserService_Create(t *testing.T) {
	t.Run("stores normalized user with hashed password", func(t *testing.T) {
		svc, repo, _ := newTestUserService()

		got, err := svc.Create(context.Background(), CreateUserInput{
			Name:     "  Alice  ",
			Email:    " Alice@Example.COM ",
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.ID == "" {
			t.Error("ID was not set")
		}
		if got.Name != "Alice" {
			t.Errorf("Name = %q, want %q", got.Name, "Alice")
		}
		if got.Email != "alice@example.com" {
			t.Errorf("Email = %q, want %q", got.Email, "alice@example.com")
		}
		if got.PasswordHash != "hashed:password123" {
			t.Errorf("PasswordHash = %q, password was not hashed", got.PasswordHash)
		}
		if want := fixedNow.Truncate(time.Millisecond); !got.CreatedAt.Equal(want) {
			t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, want)
		}
		if n, _ := repo.Count(context.Background()); n != 1 {
			t.Errorf("repository has %d users, want 1", n)
		}
	})

	validationTests := []struct {
		name      string
		input     CreateUserInput
		wantField string
	}{
		{"empty name", CreateUserInput{Name: " ", Email: "a@example.com", Password: "password123"}, "name"},
		{"invalid email", CreateUserInput{Name: "A", Email: "not-an-email", Password: "password123"}, "email"},
		{"short password", CreateUserInput{Name: "A", Email: "a@example.com", Password: "short"}, "password"},
		{"password over 72 bytes", CreateUserInput{Name: "A", Email: "a@example.com", Password: strings.Repeat("x", 73)}, "password"},
	}
	for _, tt := range validationTests {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			svc, repo, _ := newTestUserService()

			_, err := svc.Create(context.Background(), tt.input)

			assertValidationError(t, err, tt.wantField)
			if n, _ := repo.Count(context.Background()); n != 0 {
				t.Errorf("invalid user was stored")
			}
		})
	}

	t.Run("rejects duplicate email regardless of case", func(t *testing.T) {
		svc, _, _ := newTestUserService()
		seedUser(t, svc, "Alice", "alice@example.com")

		_, err := svc.Create(context.Background(), CreateUserInput{
			Name: "Other", Email: "ALICE@example.com", Password: "password123",
		})

		if !errors.Is(err, domain.ErrEmailAlreadyExists) {
			t.Fatalf("err = %v, want ErrEmailAlreadyExists", err)
		}
	})

	t.Run("returns hasher error", func(t *testing.T) {
		svc, _, hasher := newTestUserService()
		hasher.hashErr = errors.New("boom")

		_, err := svc.Create(context.Background(), CreateUserInput{
			Name: "A", Email: "a@example.com", Password: "password123",
		})

		if !errors.Is(err, hasher.hashErr) {
			t.Fatalf("err = %v, want wrapped hasher error", err)
		}
	})
}

func TestUserService_Get(t *testing.T) {
	svc, _, _ := newTestUserService()
	alice := seedUser(t, svc, "Alice", "alice@example.com")

	t.Run("found", func(t *testing.T) {
		got, err := svc.Get(context.Background(), alice.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Email != alice.Email {
			t.Errorf("Email = %q, want %q", got.Email, alice.Email)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.Get(context.Background(), "missing")
		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})
}

func TestUserService_List(t *testing.T) {
	tests := []struct {
		name                  string
		input                 ListUsersInput
		wantOffset, wantLimit int64
		wantCount             int
	}{
		{"defaults when zero", ListUsersInput{}, 0, DefaultListLimit, 3},
		{"caps limit at maximum", ListUsersInput{Limit: 1000}, 0, MaxListLimit, 3},
		{"negative offset becomes zero", ListUsersInput{Offset: -5, Limit: 2}, 0, 2, 2},
		{"offset skips users", ListUsersInput{Offset: 2, Limit: 10}, 2, 10, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newTestUserService()
			seedUser(t, svc, "A", "a@example.com")
			seedUser(t, svc, "B", "b@example.com")
			seedUser(t, svc, "C", "c@example.com")

			got, err := svc.List(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if repo.lastOffset != tt.wantOffset || repo.lastLimit != tt.wantLimit {
				t.Errorf("repo.List called with offset=%d limit=%d, want offset=%d limit=%d",
					repo.lastOffset, repo.lastLimit, tt.wantOffset, tt.wantLimit)
			}
			if len(got.Users) != tt.wantCount {
				t.Errorf("got %d users, want %d", len(got.Users), tt.wantCount)
			}
			if got.Total != 3 {
				t.Errorf("Total = %d, want 3", got.Total)
			}
		})
	}

	t.Run("returns repository error", func(t *testing.T) {
		svc, repo, _ := newTestUserService()
		repo.err = errors.New("db down")

		_, err := svc.List(context.Background(), ListUsersInput{})

		if !errors.Is(err, repo.err) {
			t.Fatalf("err = %v, want wrapped repository error", err)
		}
	})
}

func TestUserService_Update(t *testing.T) {
	ptr := func(s string) *string { return &s }

	t.Run("updates name only", func(t *testing.T) {
		svc, _, _ := newTestUserService()
		alice := seedUser(t, svc, "Alice", "alice@example.com")

		got, err := svc.Update(context.Background(), alice.ID, UpdateUserInput{Name: ptr(" Alice Smith ")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.Name != "Alice Smith" || got.Email != "alice@example.com" {
			t.Errorf("got name=%q email=%q", got.Name, got.Email)
		}
		stored, _ := svc.Get(context.Background(), alice.ID)
		if stored.Name != "Alice Smith" {
			t.Errorf("stored Name = %q, change was not saved", stored.Name)
		}
	})

	t.Run("updates email with normalization", func(t *testing.T) {
		svc, _, _ := newTestUserService()
		alice := seedUser(t, svc, "Alice", "alice@example.com")

		got, err := svc.Update(context.Background(), alice.ID, UpdateUserInput{Email: ptr("NEW@Example.com")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Email != "new@example.com" {
			t.Errorf("Email = %q, want %q", got.Email, "new@example.com")
		}
	})

	t.Run("keeps own email without conflict", func(t *testing.T) {
		svc, _, _ := newTestUserService()
		alice := seedUser(t, svc, "Alice", "alice@example.com")

		_, err := svc.Update(context.Background(), alice.ID, UpdateUserInput{Email: ptr("alice@example.com")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects empty update", func(t *testing.T) {
		svc, _, _ := newTestUserService()
		alice := seedUser(t, svc, "Alice", "alice@example.com")

		_, err := svc.Update(context.Background(), alice.ID, UpdateUserInput{})
		assertValidationError(t, err, "body")
	})

	t.Run("rejects invalid email before reading the database", func(t *testing.T) {
		svc, repo, _ := newTestUserService()
		repo.err = errors.New("database must not be called")

		_, err := svc.Update(context.Background(), "any-id", UpdateUserInput{Email: ptr("bad")})
		assertValidationError(t, err, "email")
	})

	t.Run("rejects email used by another user", func(t *testing.T) {
		svc, _, _ := newTestUserService()
		alice := seedUser(t, svc, "Alice", "alice@example.com")
		seedUser(t, svc, "Bob", "bob@example.com")

		_, err := svc.Update(context.Background(), alice.ID, UpdateUserInput{Email: ptr("bob@example.com")})
		if !errors.Is(err, domain.ErrEmailAlreadyExists) {
			t.Fatalf("err = %v, want ErrEmailAlreadyExists", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc, _, _ := newTestUserService()

		_, err := svc.Update(context.Background(), "missing", UpdateUserInput{Name: ptr("X")})
		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})
}

func TestUserService_Delete(t *testing.T) {
	t.Run("deletes existing user", func(t *testing.T) {
		svc, _, _ := newTestUserService()
		alice := seedUser(t, svc, "Alice", "alice@example.com")

		if err := svc.Delete(context.Background(), alice.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := svc.Get(context.Background(), alice.ID); !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("user still exists after delete (err = %v)", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc, _, _ := newTestUserService()

		err := svc.Delete(context.Background(), "missing")
		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})
}

func TestUserService_Count(t *testing.T) {
	svc, _, _ := newTestUserService()
	seedUser(t, svc, "A", "a@example.com")
	seedUser(t, svc, "B", "b@example.com")

	n, err := svc.Count(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Fatalf("Count = %d, want 2", n)
	}
}

// assertValidationError fails the test unless err is a ValidationError for field.
func assertValidationError(t *testing.T, err error, field string) {
	t.Helper()
	var vErr *domain.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("err = %v, want *domain.ValidationError", err)
	}
	if vErr.Field != field {
		t.Fatalf("ValidationError.Field = %q, want %q", vErr.Field, field)
	}
}
