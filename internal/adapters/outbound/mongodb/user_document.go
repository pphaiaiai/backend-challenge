package mongodb

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

// userDocument is the stored form of a user.
type userDocument struct {
	ID           bson.ObjectID `bson:"_id"`
	Name         string        `bson:"name"`
	Email        string        `bson:"email"`
	PasswordHash string        `bson:"passwordHash"`
	CreatedAt    time.Time     `bson:"createdAt"`
}

func toDocument(u *domain.User, id bson.ObjectID) userDocument {
	return userDocument{
		ID:           id,
		Name:         u.Name,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
	}
}

func (d userDocument) toDomain() *domain.User {
	return &domain.User{
		ID:           d.ID.Hex(),
		Name:         d.Name,
		Email:        d.Email,
		PasswordHash: d.PasswordHash,
		CreatedAt:    d.CreatedAt.UTC(),
	}
}

// parseID converts a domain ID to an ObjectID. A malformed ID cannot match
// any user, so it is reported as domain.ErrUserNotFound.
func parseID(id string) (bson.ObjectID, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return bson.ObjectID{}, domain.ErrUserNotFound
	}
	return oid, nil
}
