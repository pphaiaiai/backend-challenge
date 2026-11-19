// Package domain contains the core entities and business rules.
// It depends only on the standard library.
package domain

import "time"

// User is a registered user. ID is an opaque identifier assigned by the
// repository.
type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}
