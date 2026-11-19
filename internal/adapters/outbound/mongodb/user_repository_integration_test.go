//go:build integration

// Integration tests against a real MongoDB. Each test uses a temporary
// database. Run with:
//
//	MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./internal/adapters/outbound/mongodb/...
package mongodb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

func newIntegrationRepo(t *testing.T) *UserRepository {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI not set; skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := Connect(ctx, uri, 5*time.Second)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	db := client.Database(fmt.Sprintf("user_mgmt_it_%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	})

	repo := NewUserRepository(db)
	if err := repo.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	return repo
}

func newUser(name, email string) *domain.User {
	return &domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: "$2a$10$hash",
		CreatedAt:    time.Now().UTC().Truncate(time.Millisecond),
	}
}

func TestIntegration_CreateAndGet(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	alice := newUser("Alice", "alice@example.com")
	if err := repo.Create(ctx, alice); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if alice.ID == "" {
		t.Fatal("Create did not set ID")
	}

	byID, err := repo.GetByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if *byID != *alice {
		t.Errorf("GetByID = %+v, want %+v", byID, alice)
	}

	byEmail, err := repo.GetByEmail(ctx, "alice@example.com")
	if err != nil || byEmail.ID != alice.ID {
		t.Errorf("GetByEmail = %+v, %v", byEmail, err)
	}

	if _, err := repo.GetByID(ctx, bson.NewObjectID().Hex()); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("GetByID(unknown) err = %v, want ErrUserNotFound", err)
	}
	if _, err := repo.GetByEmail(ctx, "nobody@example.com"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("GetByEmail(unknown) err = %v, want ErrUserNotFound", err)
	}
}

func TestIntegration_UniqueEmail(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	if err := repo.Create(ctx, newUser("Alice", "same@example.com")); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	err := repo.Create(ctx, newUser("Bob", "same@example.com"))
	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("second Create err = %v, want ErrEmailAlreadyExists", err)
	}

	// EnsureIndexes must be safe to call again on the next start-up.
	if err := repo.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes twice: %v", err)
	}
}

func TestIntegration_ListAndCount(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	empty, err := repo.List(ctx, 0, 10)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("List on empty collection = %v, %v; want empty non-nil slice", empty, err)
	}

	for _, name := range []string{"a", "b", "c"} {
		if err := repo.Create(ctx, newUser(name, name+"@example.com")); err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}

	page, err := repo.List(ctx, 1, 1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page) != 1 || page[0].Name != "b" {
		t.Errorf("List(offset=1, limit=1) = %+v, want [b]", page)
	}

	n, err := repo.Count(ctx)
	if err != nil || n != 3 {
		t.Errorf("Count = %d, %v; want 3", n, err)
	}
}

func TestIntegration_Update(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	alice := newUser("Alice", "alice@example.com")
	bob := newUser("Bob", "bob@example.com")
	_ = repo.Create(ctx, alice)
	_ = repo.Create(ctx, bob)

	alice.Name, alice.Email = "Alice Smith", "alice.smith@example.com"
	if err := repo.Update(ctx, alice); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(ctx, alice.ID)
	if got.Name != "Alice Smith" || got.Email != "alice.smith@example.com" {
		t.Errorf("after Update got %+v", got)
	}
	if got.PasswordHash != alice.PasswordHash {
		t.Error("Update must not touch the password hash")
	}

	alice.Email = "bob@example.com"
	if err := repo.Update(ctx, alice); !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Errorf("Update to taken email err = %v, want ErrEmailAlreadyExists", err)
	}

	ghost := newUser("Ghost", "ghost@example.com")
	ghost.ID = bson.NewObjectID().Hex()
	if err := repo.Update(ctx, ghost); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("Update(unknown) err = %v, want ErrUserNotFound", err)
	}
}

func TestIntegration_Delete(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	alice := newUser("Alice", "alice@example.com")
	_ = repo.Create(ctx, alice)

	if err := repo.Delete(ctx, alice.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, alice.ID); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("second Delete err = %v, want ErrUserNotFound", err)
	}
}
