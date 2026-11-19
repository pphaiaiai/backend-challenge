package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pphaiaiai/backend-challenge/internal/adapters/inbound/httpapi"
	"github.com/pphaiaiai/backend-challenge/internal/application"
	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

const validToken = "valid-token"

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// stubUserService returns whatever its function fields return. Calling a
// method whose field is nil panics, which the router turns into a 500.
type stubUserService struct {
	create func(context.Context, application.CreateUserInput) (*domain.User, error)
	get    func(context.Context, string) (*domain.User, error)
	list   func(context.Context, application.ListUsersInput) (*application.ListUsersResult, error)
	update func(context.Context, string, application.UpdateUserInput) (*domain.User, error)
	delete func(context.Context, string) error
}

func (s *stubUserService) Create(ctx context.Context, in application.CreateUserInput) (*domain.User, error) {
	return s.create(ctx, in)
}

func (s *stubUserService) Get(ctx context.Context, id string) (*domain.User, error) {
	return s.get(ctx, id)
}

func (s *stubUserService) List(ctx context.Context, in application.ListUsersInput) (*application.ListUsersResult, error) {
	return s.list(ctx, in)
}

func (s *stubUserService) Update(ctx context.Context, id string, in application.UpdateUserInput) (*domain.User, error) {
	return s.update(ctx, id, in)
}

func (s *stubUserService) Delete(ctx context.Context, id string) error {
	return s.delete(ctx, id)
}

// stubAuthService accepts only validToken, which belongs to "user-1".
type stubAuthService struct {
	login func(context.Context, application.LoginInput) (application.Token, error)
}

func (s *stubAuthService) Login(ctx context.Context, in application.LoginInput) (application.Token, error) {
	return s.login(ctx, in)
}

func (s *stubAuthService) Authenticate(_ context.Context, token string) (string, error) {
	if token != validToken {
		return "", domain.ErrInvalidToken
	}
	return "user-1", nil
}

type testServer struct {
	t      *testing.T
	router http.Handler
	logs   *bytes.Buffer
}

func newServer(t *testing.T, users *stubUserService, auth *stubAuthService) *testServer {
	t.Helper()
	if auth == nil {
		auth = &stubAuthService{}
	}
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	return &testServer{t: t, router: httpapi.NewRouter(users, auth, logger), logs: logs}
}

// do sends a request. An empty token sends no Authorization header.
func (s *testServer) do(method, path, body, token string) *httptest.ResponseRecorder {
	s.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	return rec
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Fields  []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"fields"`
	} `json:"error"`
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return v
}

// assertError checks the status code and error code of a response.
func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) errorResponse {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, status, rec.Body)
	}
	resp := decode[errorResponse](t, rec)
	if resp.Error.Code != code {
		t.Fatalf("error code = %q, want %q; body: %s", resp.Error.Code, code, rec.Body)
	}
	return resp
}

var alice = &domain.User{
	ID:        "user-1",
	Name:      "Alice",
	Email:     "alice@example.com",
	CreatedAt: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC),
}
