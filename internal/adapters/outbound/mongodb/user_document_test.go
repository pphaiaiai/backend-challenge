package mongodb

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/pphaiaiai/backend-challenge/internal/application"
	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

var _ application.UserRepository = (*UserRepository)(nil)

func TestDocumentMappingRoundTrip(t *testing.T) {
	id := bson.NewObjectID()
	bangkok := time.FixedZone("ICT", 7*60*60)
	user := &domain.User{
		Name:         "Alice",
		Email:        "alice@example.com",
		PasswordHash: "$2a$10$hash",
		CreatedAt:    time.Date(2026, 11, 19, 17, 0, 0, 0, bangkok),
	}

	got := toDocument(user, id).toDomain()

	if got.ID != id.Hex() {
		t.Errorf("ID = %q, want %q", got.ID, id.Hex())
	}
	if got.Name != user.Name || got.Email != user.Email || got.PasswordHash != user.PasswordHash {
		t.Errorf("fields changed in round trip: %+v", got)
	}
	if !got.CreatedAt.Equal(user.CreatedAt) || got.CreatedAt.Location() != time.UTC {
		t.Errorf("CreatedAt = %v, want the same instant in UTC", got.CreatedAt)
	}
}

func TestParseID(t *testing.T) {
	valid := bson.NewObjectID()

	got, err := parseID(valid.Hex())
	if err != nil || got != valid {
		t.Fatalf("parseID(valid) = %v, %v", got, err)
	}

	for _, bad := range []string{"", "123", "not-an-object-id", "zzzzzzzzzzzzzzzzzzzzzzzz"} {
		if _, err := parseID(bad); !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("parseID(%q) err = %v, want ErrUserNotFound", bad, err)
		}
	}
}

// Nothing listens on port 1, so any query would fail with a connection error
// instead of ErrUserNotFound.
func TestRepositoryRejectsMalformedIDWithoutQuerying(t *testing.T) {
	client, err := mongo.Connect(options.Client().
		ApplyURI("mongodb://127.0.0.1:1").
		SetServerSelectionTimeout(100 * time.Millisecond))
	if err != nil {
		t.Fatalf("mongo.Connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })

	repo := NewUserRepository(client.Database("unused"))
	ctx := context.Background()

	if _, err := repo.GetByID(ctx, "bad-id"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("GetByID err = %v, want ErrUserNotFound", err)
	}
	if err := repo.Update(ctx, &domain.User{ID: "bad-id"}); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("Update err = %v, want ErrUserNotFound", err)
	}
	if err := repo.Delete(ctx, "bad-id"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("Delete err = %v, want ErrUserNotFound", err)
	}
}
