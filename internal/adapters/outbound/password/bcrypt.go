// Package password implements application.PasswordHasher with bcrypt.
package password

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// BcryptHasher hashes passwords with bcrypt.
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher returns a hasher with the given cost
// (bcrypt.MinCost to bcrypt.MaxCost).
func NewBcryptHasher(cost int) (*BcryptHasher, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("bcrypt cost must be between %d and %d, got %d",
			bcrypt.MinCost, bcrypt.MaxCost, cost)
	}
	return &BcryptHasher{cost: cost}, nil
}

// Hash returns the bcrypt hash of password, including its salt and cost.
func (h *BcryptHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(hash), nil
}

// Compare returns nil if password matches hash.
func (h *BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
