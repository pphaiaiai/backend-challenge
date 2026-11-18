package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

func TestNormalizeEmail(t *testing.T) {
	got := domain.NormalizeEmail("  Alice@Example.COM ")
	if want := "alice@example.com"; got != want {
		t.Fatalf("NormalizeEmail() = %q, want %q", got, want)
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "Alice"},
		{name: "thai characters", input: "สมชาย ใจดี"},
		{name: "exactly 100 characters", input: strings.Repeat("ก", 100)},
		{name: "empty", input: "", wantErr: true},
		{name: "101 characters", input: strings.Repeat("a", 101), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateName(tt.input)
			assertFieldError(t, err, "name", tt.wantErr)
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "alice@example.com"},
		{name: "plus addressing", input: "alice+test@example.co.th"},
		{name: "empty", input: "", wantErr: true},
		{name: "missing at sign", input: "alice.example.com", wantErr: true},
		{name: "missing domain", input: "alice@", wantErr: true},
		{name: "display name form", input: "Alice <alice@example.com>", wantErr: true},
		{name: "too long", input: strings.Repeat("a", 250) + "@x.co", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateEmail(tt.input)
			assertFieldError(t, err, "email", tt.wantErr)
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "s3cret-pass"},
		{name: "exactly 8 characters", input: "12345678"},
		{name: "thai characters", input: "รหัสผ่านดี"},
		{name: "exactly 72 bytes", input: strings.Repeat("a", 72)},
		{name: "7 characters", input: "1234567", wantErr: true},
		{name: "73 bytes", input: strings.Repeat("a", 73), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidatePassword(tt.input)
			assertFieldError(t, err, "password", tt.wantErr)
		})
	}
}

// assertFieldError checks that err is nil, or a ValidationError for field
// when wantErr is true.
func assertFieldError(t *testing.T, err error, field string, wantErr bool) {
	t.Helper()
	if !wantErr {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	var vErr *domain.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *domain.ValidationError, got %v", err)
	}
	if vErr.Field != field {
		t.Fatalf("ValidationError.Field = %q, want %q", vErr.Field, field)
	}
}
