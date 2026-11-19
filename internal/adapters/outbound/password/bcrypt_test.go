package password_test

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/pphaiaiai/backend-challenge/internal/adapters/outbound/password"
	"github.com/pphaiaiai/backend-challenge/internal/application"
)

var _ application.PasswordHasher = (*password.BcryptHasher)(nil)

// newHasher uses MinCost to keep tests fast.
func newHasher(t *testing.T) *password.BcryptHasher {
	t.Helper()
	h, err := password.NewBcryptHasher(bcrypt.MinCost)
	if err != nil {
		t.Fatalf("NewBcryptHasher: %v", err)
	}
	return h
}

func TestBcryptHasher_HashAndCompare(t *testing.T) {
	h := newHasher(t)

	hash, err := h.Hash("correct-horse")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hash == "correct-horse" || !strings.HasPrefix(hash, "$2a$") {
		t.Fatalf("hash %q does not look like a bcrypt hash", hash)
	}

	if err := h.Compare(hash, "correct-horse"); err != nil {
		t.Errorf("Compare with the right password: %v", err)
	}
	if err := h.Compare(hash, "wrong-horse"); err == nil {
		t.Error("Compare with a wrong password returned nil")
	}
}

func TestBcryptHasher_SaltsEveryHash(t *testing.T) {
	h := newHasher(t)

	a, _ := h.Hash("same-password")
	b, _ := h.Hash("same-password")

	if a == b {
		t.Fatal("two hashes of the same password are identical; salt is missing")
	}
}

func TestBcryptHasher_RejectsPasswordsOver72Bytes(t *testing.T) {
	h := newHasher(t)

	if _, err := h.Hash(strings.Repeat("a", 73)); err == nil {
		t.Fatal("expected an error for a 73-byte password")
	}
}

func TestNewBcryptHasher_ValidatesCost(t *testing.T) {
	for _, cost := range []int{bcrypt.MinCost - 1, bcrypt.MaxCost + 1} {
		if _, err := password.NewBcryptHasher(cost); err == nil {
			t.Errorf("cost %d: expected an error", cost)
		}
	}
}
