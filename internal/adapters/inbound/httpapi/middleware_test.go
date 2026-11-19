package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

func TestRequestLogger(t *testing.T) {
	srv := newServer(t, &stubUserService{}, nil)

	srv.do(http.MethodGet, "/healthz", "", "")

	var entry map[string]any
	if err := json.Unmarshal(srv.logs.Bytes(), &entry); err != nil {
		t.Fatalf("log is not one JSON line: %q", srv.logs)
	}
	if entry["method"] != "GET" || entry["path"] != "/healthz" || entry["status"] != float64(200) {
		t.Errorf("unexpected log entry: %v", entry)
	}
	if _, ok := entry["duration_ms"].(float64); !ok {
		t.Errorf("duration_ms missing from log entry: %v", entry)
	}
}

func TestRequireAuth(t *testing.T) {
	users := &stubUserService{get: func(context.Context, string) (*domain.User, error) { return alice, nil }}
	srv := newServer(t, users, nil)

	tests := []struct {
		name   string
		header string
		status int
		code   string
	}{
		{"missing header", "", http.StatusUnauthorized, "unauthorized"},
		{"wrong scheme", "Basic dXNlcjpwYXNz", http.StatusUnauthorized, "unauthorized"},
		{"empty token", "Bearer ", http.StatusUnauthorized, "unauthorized"},
		{"invalid token", "Bearer forged", http.StatusUnauthorized, "invalid_token"},
		{"valid token", "Bearer " + validToken, http.StatusOK, ""},
		{"scheme is case-insensitive", "bearer " + validToken, http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-1", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			srv.router.ServeHTTP(rec, req)

			if tt.code == "" {
				if rec.Code != tt.status {
					t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.status, rec.Body)
				}
				return
			}
			assertError(t, rec, tt.status, tt.code)
			if !strings.HasPrefix(rec.Header().Get("WWW-Authenticate"), "Bearer") {
				t.Errorf("WWW-Authenticate = %q", rec.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestRecovery(t *testing.T) {
	users := &stubUserService{get: func(context.Context, string) (*domain.User, error) { panic("boom") }}
	srv := newServer(t, users, nil)

	rec := srv.do(http.MethodGet, "/api/v1/users/user-1", "", validToken)

	assertError(t, rec, http.StatusInternalServerError, "internal_error")
	if !strings.Contains(srv.logs.String(), "panic recovered") {
		t.Errorf("panic was not logged; logs: %s", srv.logs)
	}
}

func TestBodySizeLimit(t *testing.T) {
	srv := newServer(t, &stubUserService{}, nil)
	huge := `{"name":"` + strings.Repeat("a", 2<<20) + `"}`

	rec := srv.do(http.MethodPost, "/api/v1/auth/register", huge, "")

	assertError(t, rec, http.StatusRequestEntityTooLarge, "request_too_large")
}

func TestUnknownRoutes(t *testing.T) {
	srv := newServer(t, &stubUserService{}, nil)

	assertError(t, srv.do(http.MethodGet, "/nope", "", ""), http.StatusNotFound, "not_found")
	assertError(t, srv.do(http.MethodPut, "/api/v1/auth/login", "", ""), http.StatusMethodNotAllowed, "method_not_allowed")
}
