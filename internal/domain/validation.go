package domain

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

// Input limits.
const (
	MaxNameLength     = 100 // characters
	MaxEmailLength    = 254 // bytes (RFC 5321)
	MinPasswordLength = 8   // characters
	MaxPasswordLength = 72  // bytes; bcrypt ignores anything longer
)

// NormalizeName trims surrounding whitespace.
func NormalizeName(name string) string {
	return strings.TrimSpace(name)
}

// NormalizeEmail trims whitespace and lower-cases the address.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateName checks a normalized name.
func ValidateName(name string) error {
	if name == "" {
		return &ValidationError{Field: "name", Message: "is required"}
	}
	if utf8.RuneCountInString(name) > MaxNameLength {
		return &ValidationError{Field: "name", Message: "must be at most 100 characters"}
	}
	return nil
}

// ValidateEmail checks a normalized email address.
func ValidateEmail(email string) error {
	if email == "" {
		return &ValidationError{Field: "email", Message: "is required"}
	}
	if len(email) > MaxEmailLength {
		return &ValidationError{Field: "email", Message: "must be at most 254 characters"}
	}
	// ParseAddress also accepts "Name <addr>"; require a bare address.
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return &ValidationError{Field: "email", Message: "must be a valid email address"}
	}
	return nil
}

// ValidatePassword checks the length of a plain-text password. The minimum is
// counted in characters and the maximum in bytes, which is the bcrypt limit.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return &ValidationError{Field: "password", Message: "must be at least 8 characters"}
	}
	if len(password) > MaxPasswordLength {
		return &ValidationError{Field: "password", Message: "must be at most 72 bytes"}
	}
	return nil
}
