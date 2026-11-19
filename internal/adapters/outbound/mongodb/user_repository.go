package mongodb

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

// UsersCollection is the collection that stores users.
const UsersCollection = "users"

// UserRepository stores users in MongoDB.
type UserRepository struct {
	coll *mongo.Collection
}

// NewUserRepository returns a repository for the users collection of db.
func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{coll: db.Collection(UsersCollection)}
}

// EnsureIndexes creates the unique index on email. It is safe to call on
// every start-up.
func (r *UserRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_email"),
	})
	if err != nil {
		return fmt.Errorf("create email index: %w", err)
	}
	return nil
}

// Create inserts the user and sets user.ID.
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	id := bson.NewObjectID()
	if _, err := r.coll.InsertOne(ctx, toDocument(user, id)); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrEmailAlreadyExists
		}
		return fmt.Errorf("insert user: %w", err)
	}
	user.ID = id.Hex()
	return nil
}

// GetByID returns the user with the given ID.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	oid, err := parseID(id)
	if err != nil {
		return nil, err
	}
	return r.findOne(ctx, bson.D{{Key: "_id", Value: oid}})
}

// GetByEmail returns the user with the given normalized email.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.findOne(ctx, bson.D{{Key: "email", Value: email}})
}

func (r *UserRepository) findOne(ctx context.Context, filter bson.D) (*domain.User, error) {
	var doc userDocument
	err := r.coll.FindOne(ctx, filter).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return doc.toDomain(), nil
}

// List returns up to limit users after skipping offset, oldest first.
func (r *UserRepository) List(ctx context.Context, offset, limit int64) ([]*domain.User, error) {
	// ObjectIDs start with a timestamp, so sorting by _id gives creation order.
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: 1}}).
		SetSkip(offset).
		SetLimit(limit)

	cursor, err := r.coll.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("find users: %w", err)
	}
	var docs []userDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}

	users := make([]*domain.User, 0, len(docs)) // non-nil, so JSON renders []
	for _, d := range docs {
		users = append(users, d.toDomain())
	}
	return users, nil
}

// Update sets the user's name and email.
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	oid, err := parseID(user.ID)
	if err != nil {
		return err
	}
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "name", Value: user.Name},
		{Key: "email", Value: user.Email},
	}}}

	res, err := r.coll.UpdateByID(ctx, oid, update)
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrEmailAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// Delete removes the user with the given ID.
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	oid, err := parseID(id)
	if err != nil {
		return err
	}
	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}})
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if res.DeletedCount == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// Count returns the exact number of users.
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	n, err := r.coll.CountDocuments(ctx, bson.D{})
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}
