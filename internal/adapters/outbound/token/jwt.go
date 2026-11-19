// Package token implements application.TokenService with HS256-signed JWTs.
package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/pphaiaiai/backend-challenge/internal/application"
)

// MinSecretLength is the minimum HMAC key size in bytes, matching the
// 256-bit HS256 signature.
const MinSecretLength = 32

// JWTService issues and verifies JWTs carrying the sub, iss, iat and exp claims.
type JWTService struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

// NewJWTService returns a JWTService, or an error if the configuration is unsafe.
func NewJWTService(secret, issuer string, ttl time.Duration) (*JWTService, error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("jwt secret must be at least %d bytes, got %d", MinSecretLength, len(secret))
	}
	if issuer == "" {
		return nil, errors.New("jwt issuer must not be empty")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("jwt ttl must be positive, got %s", ttl)
	}
	return &JWTService{secret: []byte(secret), issuer: issuer, ttl: ttl, now: time.Now}, nil
}

// Issue returns a signed token for userID.
func (s *JWTService) Issue(userID string) (application.Token, error) {
	// JWT timestamps have one-second precision.
	now := s.now().UTC().Truncate(time.Second)
	expiresAt := now.Add(s.ttl)

	claims := jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    s.issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return application.Token{}, fmt.Errorf("sign jwt: %w", err)
	}
	return application.Token{Value: signed, ExpiresAt: expiresAt}, nil
}

// Verify validates the token and returns its subject (the user ID).
func (s *JWTService) Verify(tokenString string) (string, error) {
	claims := &jwt.RegisteredClaims{}

	_, err := jwt.ParseWithClaims(tokenString, claims,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		// Reject every algorithm except HS256, including "none".
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		return "", fmt.Errorf("verify jwt: %w", err)
	}
	if claims.Subject == "" {
		return "", errors.New("verify jwt: missing subject")
	}
	return claims.Subject, nil
}
