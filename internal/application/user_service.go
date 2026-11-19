package application

import (
	"context"
	"fmt"
	"time"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

// Pagination limits for List.
const (
	DefaultListLimit int64 = 20
	MaxListLimit     int64 = 100
)

// CreateUserInput holds the fields for creating a user.
type CreateUserInput struct {
	Name     string
	Email    string
	Password string
}

// UpdateUserInput holds the fields to change. Nil fields are left unchanged.
type UpdateUserInput struct {
	Name  *string
	Email *string
}

// ListUsersInput controls pagination. Zero values select the defaults.
type ListUsersInput struct {
	Offset int64
	Limit  int64
}

// ListUsersResult is a page of users and the total number of users.
type ListUsersResult struct {
	Users  []*domain.User
	Total  int64
	Offset int64
	Limit  int64
}

// UserService implements the user management use cases.
type UserService struct {
	repo   UserRepository
	hasher PasswordHasher
	now    func() time.Time
}

// NewUserService creates a UserService.
func NewUserService(repo UserRepository, hasher PasswordHasher) *UserService {
	return &UserService{repo: repo, hasher: hasher, now: time.Now}
}

// Create validates the input, hashes the password and stores the user.
func (s *UserService) Create(ctx context.Context, in CreateUserInput) (*domain.User, error) {
	name := domain.NormalizeName(in.Name)
	email := domain.NormalizeEmail(in.Email)

	if err := firstError(
		domain.ValidateName(name),
		domain.ValidateEmail(email),
		domain.ValidatePassword(in.Password),
	); err != nil {
		return nil, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		// MongoDB stores milliseconds; truncate so the response matches later reads.
		CreatedAt: s.now().UTC().Truncate(time.Millisecond),
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// Get returns the user with the given ID.
func (s *UserService) Get(ctx context.Context, id string) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// List returns a page of users and the total count.
func (s *UserService) List(ctx context.Context, in ListUsersInput) (*ListUsersResult, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}
	offset := max(in.Offset, 0)

	users, err := s.repo.List(ctx, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	return &ListUsersResult{Users: users, Total: total, Offset: offset, Limit: limit}, nil
}

// Update changes the name and/or email of a user.
func (s *UserService) Update(ctx context.Context, id string, in UpdateUserInput) (*domain.User, error) {
	if in.Name == nil && in.Email == nil {
		return nil, &domain.ValidationError{Field: "body", Message: "must contain name or email"}
	}

	// Validate before reading from the database.
	var name, email string
	if in.Name != nil {
		name = domain.NormalizeName(*in.Name)
		if err := domain.ValidateName(name); err != nil {
			return nil, err
		}
	}
	if in.Email != nil {
		email = domain.NormalizeEmail(*in.Email)
		if err := domain.ValidateEmail(email); err != nil {
			return nil, err
		}
	}

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if in.Name != nil {
		user.Name = name
	}
	if in.Email != nil {
		user.Email = email
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

// Delete removes the user with the given ID.
func (s *UserService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// Count returns the total number of users.
func (s *UserService) Count(ctx context.Context) (int64, error) {
	n, err := s.repo.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
