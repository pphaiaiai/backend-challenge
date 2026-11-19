package token

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/pphaiaiai/backend-challenge/internal/application"
)

var _ application.TokenService = (*JWTService)(nil)

const (
	testSecret = "this-is-a-test-secret-with-32-bytes!!"
	testIssuer = "backend-challenge"
)

var t0 = time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)

// newTestService returns a JWTService whose clock is fixed at t0.
func newTestService(t *testing.T) *JWTService {
	t.Helper()
	s, err := NewJWTService(testSecret, testIssuer, time.Hour)
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	s.now = func() time.Time { return t0 }
	return s
}

// sign signs arbitrary claims, for building tokens that must be rejected.
func sign(t *testing.T, method jwt.SigningMethod, claims jwt.Claims, key any) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func validClaims() jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Subject:   "user-1",
		Issuer:    testIssuer,
		IssuedAt:  jwt.NewNumericDate(t0),
		ExpiresAt: jwt.NewNumericDate(t0.Add(time.Hour)),
	}
}

func TestJWTService_IssueAndVerify(t *testing.T) {
	s := newTestService(t)

	tok, err := s.Issue("user-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if want := t0.Add(time.Hour); !tok.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", tok.ExpiresAt, want)
	}
	if parts := strings.Split(tok.Value, "."); len(parts) != 3 {
		t.Fatalf("token %q is not header.payload.signature", tok.Value)
	}

	userID, err := s.Verify(tok.Value)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if userID != "user-1" {
		t.Errorf("userID = %q, want %q", userID, "user-1")
	}
}

func TestJWTService_VerifyRejects(t *testing.T) {
	s := newTestService(t)

	noExp := validClaims()
	noExp.ExpiresAt = nil
	otherIssuer := validClaims()
	otherIssuer.Issuer = "someone-else"
	noSubject := validClaims()
	noSubject.Subject = ""

	tests := []struct {
		name    string
		token   string
		wantErr error // nil means "any error"
	}{
		{
			name:    "garbage",
			token:   "not-a-jwt",
			wantErr: jwt.ErrTokenMalformed,
		},
		{
			name:    "signed with another secret",
			token:   sign(t, jwt.SigningMethodHS256, validClaims(), []byte("another-secret-that-is-32-bytes-long!")),
			wantErr: jwt.ErrTokenSignatureInvalid,
		},
		{
			name:    `"alg": "none" (unsigned)`,
			token:   sign(t, jwt.SigningMethodNone, validClaims(), jwt.UnsafeAllowNoneSignatureType),
			wantErr: jwt.ErrTokenSignatureInvalid,
		},
		{
			name:    "HS512 instead of HS256",
			token:   sign(t, jwt.SigningMethodHS512, validClaims(), []byte(testSecret)),
			wantErr: jwt.ErrTokenSignatureInvalid,
		},
		{
			name:    "wrong issuer",
			token:   sign(t, jwt.SigningMethodHS256, otherIssuer, []byte(testSecret)),
			wantErr: jwt.ErrTokenInvalidIssuer,
		},
		{
			name:    "missing exp",
			token:   sign(t, jwt.SigningMethodHS256, noExp, []byte(testSecret)),
			wantErr: jwt.ErrTokenRequiredClaimMissing,
		},
		{
			name:  "missing subject",
			token: sign(t, jwt.SigningMethodHS256, noSubject, []byte(testSecret)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.Verify(tt.token)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestJWTService_VerifyRejectsExpiredToken(t *testing.T) {
	s := newTestService(t)
	tok, _ := s.Issue("user-1")

	s.now = func() time.Time { return t0.Add(time.Hour + time.Second) }

	_, err := s.Verify(tok.Value)
	if !errors.Is(err, jwt.ErrTokenExpired) {
		t.Fatalf("err = %v, want ErrTokenExpired", err)
	}
}

func TestJWTService_VerifyRejectsTamperedPayload(t *testing.T) {
	s := newTestService(t)
	alice, _ := s.Issue("alice")
	bob, _ := s.Issue("bob")

	// Keep Alice's header and signature but swap in Bob's payload.
	a := strings.Split(alice.Value, ".")
	b := strings.Split(bob.Value, ".")
	forged := a[0] + "." + b[1] + "." + a[2]

	_, err := s.Verify(forged)
	if !errors.Is(err, jwt.ErrTokenSignatureInvalid) {
		t.Fatalf("err = %v, want ErrTokenSignatureInvalid", err)
	}
}

func TestNewJWTService_ValidatesConfig(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		issuer string
		ttl    time.Duration
	}{
		{"short secret", "too-short", testIssuer, time.Hour},
		{"empty issuer", testSecret, "", time.Hour},
		{"zero ttl", testSecret, testIssuer, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewJWTService(tt.secret, tt.issuer, tt.ttl); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
