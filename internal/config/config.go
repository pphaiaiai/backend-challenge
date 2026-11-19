// Package config loads service configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Config holds all service settings.
type Config struct {
	HTTPAddr        string
	ShutdownTimeout time.Duration

	MongoURI      string
	MongoDatabase string
	MongoTimeout  time.Duration

	JWTSecret  string
	JWTIssuer  string
	JWTTTL     time.Duration
	BcryptCost int

	UserCountInterval time.Duration
}

// Load reads the configuration using getenv (normally os.Getenv).
// Unset variables take their defaults; JWT_SECRET is required.
func Load(getenv func(string) string) (Config, error) {
	l := loader{getenv: getenv}
	cfg := Config{
		HTTPAddr:        l.string("HTTP_ADDR", ":8080"),
		ShutdownTimeout: l.duration("SHUTDOWN_TIMEOUT", 10*time.Second),

		MongoURI:      l.string("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase: l.string("MONGO_DATABASE", "user_management"),
		MongoTimeout:  l.duration("MONGO_TIMEOUT", 5*time.Second),

		JWTSecret:  l.string("JWT_SECRET", ""),
		JWTIssuer:  l.string("JWT_ISSUER", "backend-challenge"),
		JWTTTL:     l.duration("JWT_TTL", time.Hour),
		BcryptCost: l.int("BCRYPT_COST", 10),

		UserCountInterval: l.duration("USER_COUNT_INTERVAL", 10*time.Second),
	}
	if cfg.JWTSecret == "" {
		l.errs = append(l.errs, errors.New("JWT_SECRET is required"))
	}
	if err := errors.Join(l.errs...); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

type loader struct {
	getenv func(string) string
	errs   []error
}

func (l *loader) string(key, def string) string {
	if v := l.getenv(key); v != "" {
		return v
	}
	return def
}

func (l *loader) duration(key string, def time.Duration) time.Duration {
	v := l.getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		l.errs = append(l.errs, fmt.Errorf("%s must be a positive duration such as 10s, got %q", key, v))
		return def
	}
	return d
}

func (l *loader) int(key string, def int) int {
	v := l.getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s must be an integer, got %q", key, v))
		return def
	}
	return n
}
