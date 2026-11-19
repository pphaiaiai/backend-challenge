package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/pphaiaiai/backend-challenge/internal/config"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{"JWT_SECRET": "secret"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := config.Config{
		HTTPAddr:          ":8080",
		GRPCAddr:          ":9090",
		GRPCReflection:    true,
		ShutdownTimeout:   10 * time.Second,
		MongoURI:          "mongodb://localhost:27017",
		MongoDatabase:     "user_management",
		MongoTimeout:      5 * time.Second,
		JWTSecret:         "secret",
		JWTIssuer:         "backend-challenge",
		JWTTTL:            time.Hour,
		BcryptCost:        10,
		UserCountInterval: 10 * time.Second,
	}
	if cfg != want {
		t.Errorf("Load() =\n%+v\nwant\n%+v", cfg, want)
	}
}

func TestLoad_Overrides(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{
		"JWT_SECRET":          "secret",
		"HTTP_ADDR":           ":9000",
		"MONGO_URI":           "mongodb://mongo:27017",
		"JWT_TTL":             "15m",
		"BCRYPT_COST":         "12",
		"USER_COUNT_INTERVAL": "1s",
		"GRPC_REFLECTION":     "false",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":9000" || cfg.MongoURI != "mongodb://mongo:27017" ||
		cfg.JWTTTL != 15*time.Minute || cfg.BcryptCost != 12 || cfg.UserCountInterval != time.Second ||
		cfg.GRPCReflection {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

func TestLoad_Errors(t *testing.T) {
	_, err := config.Load(env(map[string]string{
		"JWT_TTL":         "forever",
		"BCRYPT_COST":     "high",
		"GRPC_REFLECTION": "maybe",
	}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"JWT_SECRET", "JWT_TTL", "BCRYPT_COST", "GRPC_REFLECTION"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}
