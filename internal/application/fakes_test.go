package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

// fakeUserRepo is an in-memory UserRepository that enforces unique emails.
type fakeUserRepo struct {
	mu     sync.Mutex
	users  []*domain.User // in creation order
	nextID int

	err error // if set, returned by every method

	lastOffset, lastLimit int64 // arguments of the last List call
}

func newFakeUserRepo() *fakeUserRepo { return &fakeUserRepo{} }

func (r *fakeUserRepo) Create(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	if r.findByEmail(u.Email) != nil {
		return domain.ErrEmailAlreadyExists
	}
	r.nextID++
	u.ID = fmt.Sprintf("user-%d", r.nextID)
	r.users = append(r.users, clone(u))
	return nil
}

func (r *fakeUserRepo) GetByID(_ context.Context, id string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	if u := r.findByID(id); u != nil {
		return clone(u), nil
	}
	return nil, domain.ErrUserNotFound
}

func (r *fakeUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	if u := r.findByEmail(email); u != nil {
		return clone(u), nil
	}
	return nil, domain.ErrUserNotFound
}

func (r *fakeUserRepo) List(_ context.Context, offset, limit int64) ([]*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastOffset, r.lastLimit = offset, limit
	if r.err != nil {
		return nil, r.err
	}
	out := []*domain.User{}
	for i := offset; i < int64(len(r.users)) && int64(len(out)) < limit; i++ {
		out = append(out, clone(r.users[i]))
	}
	return out, nil
}

func (r *fakeUserRepo) Update(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	stored := r.findByID(u.ID)
	if stored == nil {
		return domain.ErrUserNotFound
	}
	if other := r.findByEmail(u.Email); other != nil && other.ID != u.ID {
		return domain.ErrEmailAlreadyExists
	}
	stored.Name, stored.Email = u.Name, u.Email
	return nil
}

func (r *fakeUserRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	for i, u := range r.users {
		if u.ID == id {
			r.users = append(r.users[:i], r.users[i+1:]...)
			return nil
		}
	}
	return domain.ErrUserNotFound
}

func (r *fakeUserRepo) Count(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return 0, r.err
	}
	return int64(len(r.users)), nil
}

func (r *fakeUserRepo) findByID(id string) *domain.User {
	for _, u := range r.users {
		if u.ID == id {
			return u
		}
	}
	return nil
}

func (r *fakeUserRepo) findByEmail(email string) *domain.User {
	for _, u := range r.users {
		if u.Email == email {
			return u
		}
	}
	return nil
}

// clone returns a copy, as a real database would, so callers cannot change
// stored users through a returned pointer.
func clone(u *domain.User) *domain.User {
	c := *u
	return &c
}

// fakeHasher "hashes" by adding a prefix.
type fakeHasher struct {
	hashErr      error
	compareCalls int
}

func (h *fakeHasher) Hash(password string) (string, error) {
	if h.hashErr != nil {
		return "", h.hashErr
	}
	return "hashed:" + password, nil
}

func (h *fakeHasher) Compare(hash, password string) error {
	h.compareCalls++
	if hash != "hashed:"+password {
		return errors.New("password mismatch")
	}
	return nil
}

// fakeTokens issues tokens of the form "token-for-<userID>".
type fakeTokens struct {
	issueErr error
}

var fakeTokenExpiry = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

func (f *fakeTokens) Issue(userID string) (Token, error) {
	if f.issueErr != nil {
		return Token{}, f.issueErr
	}
	return Token{Value: "token-for-" + userID, ExpiresAt: fakeTokenExpiry}, nil
}

func (f *fakeTokens) Verify(token string) (string, error) {
	userID, ok := strings.CutPrefix(token, "token-for-")
	if !ok || userID == "" {
		return "", errors.New("bad token")
	}
	return userID, nil
}
