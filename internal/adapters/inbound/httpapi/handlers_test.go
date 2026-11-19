package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pphaiaiai/backend-challenge/internal/application"
	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

func TestRegister(t *testing.T) {
	t.Run("creates the user", func(t *testing.T) {
		var got application.CreateUserInput
		users := &stubUserService{create: func(_ context.Context, in application.CreateUserInput) (*domain.User, error) {
			got = in
			return alice, nil
		}}
		srv := newServer(t, users, nil)

		rec := srv.do(http.MethodPost, "/api/v1/auth/register",
			`{"name":"Alice","email":"alice@example.com","password":"password123"}`, "")

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
		}
		if got.Name != "Alice" || got.Email != "alice@example.com" || got.Password != "password123" {
			t.Errorf("service received %+v", got)
		}
		if loc := rec.Header().Get("Location"); loc != "/api/v1/users/user-1" {
			t.Errorf("Location = %q", loc)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `"id":"user-1"`) || !strings.Contains(body, `"created_at":"2026-11-19T10:00:00Z"`) {
			t.Errorf("unexpected body: %s", body)
		}
		if strings.Contains(strings.ToLower(body), "password") {
			t.Errorf("response must not contain the password: %s", body)
		}
	})

	t.Run("rejects invalid fields", func(t *testing.T) {
		srv := newServer(t, &stubUserService{}, nil)

		rec := srv.do(http.MethodPost, "/api/v1/auth/register", `{"name":"","email":"nope","password":"short"}`, "")

		resp := assertError(t, rec, http.StatusBadRequest, "validation_failed")
		fields := map[string]string{}
		for _, f := range resp.Error.Fields {
			fields[f.Field] = f.Message
		}
		want := map[string]string{
			"name":     "is required",
			"email":    "must be a valid email address",
			"password": "must be at least 8 characters",
		}
		for field, msg := range want {
			if fields[field] != msg {
				t.Errorf("field %q message = %q, want %q", field, fields[field], msg)
			}
		}
	})

	malformed := []struct {
		name string
		body string
	}{
		{"empty body", ""},
		{"invalid JSON", `{"name":`},
		{"unknown field", `{"name":"A","email":"a@example.com","password":"password123","role":"admin"}`},
		{"wrong type", `{"name":123,"email":"a@example.com","password":"password123"}`},
	}
	for _, tt := range malformed {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			srv := newServer(t, &stubUserService{}, nil)
			rec := srv.do(http.MethodPost, "/api/v1/auth/register", tt.body, "")
			assertError(t, rec, http.StatusBadRequest, "invalid_request")
		})
	}

	t.Run("maps service errors", func(t *testing.T) {
		tests := []struct {
			err    error
			status int
			code   string
		}{
			{domain.ErrEmailAlreadyExists, http.StatusConflict, "email_already_exists"},
			{&domain.ValidationError{Field: "name", Message: "is required"}, http.StatusBadRequest, "validation_failed"},
			{errors.New("database exploded"), http.StatusInternalServerError, "internal_error"},
		}
		for _, tt := range tests {
			users := &stubUserService{create: func(context.Context, application.CreateUserInput) (*domain.User, error) {
				return nil, tt.err
			}}
			srv := newServer(t, users, nil)

			rec := srv.do(http.MethodPost, "/api/v1/auth/register",
				`{"name":"A","email":"a@example.com","password":"password123"}`, "")

			resp := assertError(t, rec, tt.status, tt.code)
			if strings.Contains(resp.Error.Message, "exploded") {
				t.Errorf("internal error leaked to client: %q", resp.Error.Message)
			}
		}
	})
}

func TestLogin(t *testing.T) {
	t.Run("returns a bearer token", func(t *testing.T) {
		expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
		auth := &stubAuthService{login: func(_ context.Context, in application.LoginInput) (application.Token, error) {
			if in.Email != "alice@example.com" || in.Password != "password123" {
				t.Errorf("service received %+v", in)
			}
			return application.Token{Value: "jwt-value", ExpiresAt: expires}, nil
		}}
		srv := newServer(t, &stubUserService{}, auth)

		rec := srv.do(http.MethodPost, "/api/v1/auth/login", `{"email":"alice@example.com","password":"password123"}`, "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d; body: %s", rec.Code, rec.Body)
		}
		resp := decode[struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			ExpiresIn   int64  `json:"expires_in"`
		}](t, rec)
		if resp.AccessToken != "jwt-value" || resp.TokenType != "Bearer" {
			t.Errorf("unexpected token response: %+v", resp)
		}
		if resp.ExpiresIn < 3590 || resp.ExpiresIn > 3600 {
			t.Errorf("expires_in = %d, want about 3600", resp.ExpiresIn)
		}
	})

	t.Run("rejects invalid credentials", func(t *testing.T) {
		auth := &stubAuthService{login: func(context.Context, application.LoginInput) (application.Token, error) {
			return application.Token{}, domain.ErrInvalidCredentials
		}}
		srv := newServer(t, &stubUserService{}, auth)

		rec := srv.do(http.MethodPost, "/api/v1/auth/login", `{"email":"alice@example.com","password":"wrong-pass"}`, "")

		assertError(t, rec, http.StatusUnauthorized, "invalid_credentials")
	})
}

func TestCreateUser_RequiresToken(t *testing.T) {
	users := &stubUserService{create: func(context.Context, application.CreateUserInput) (*domain.User, error) {
		return alice, nil
	}}
	srv := newServer(t, users, nil)
	body := `{"name":"Alice","email":"alice@example.com","password":"password123"}`

	assertError(t, srv.do(http.MethodPost, "/api/v1/users", body, ""), http.StatusUnauthorized, "unauthorized")

	if rec := srv.do(http.MethodPost, "/api/v1/users", body, validToken); rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
}

func TestGetUser(t *testing.T) {
	users := &stubUserService{get: func(_ context.Context, id string) (*domain.User, error) {
		if id == "user-1" {
			return alice, nil
		}
		return nil, domain.ErrUserNotFound
	}}
	srv := newServer(t, users, nil)

	rec := srv.do(http.MethodGet, "/api/v1/users/user-1", "", validToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"email":"alice@example.com"`) {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body)
	}

	assertError(t, srv.do(http.MethodGet, "/api/v1/users/missing", "", validToken), http.StatusNotFound, "not_found")
}

func TestListUsers(t *testing.T) {
	var got application.ListUsersInput
	users := &stubUserService{list: func(_ context.Context, in application.ListUsersInput) (*application.ListUsersResult, error) {
		got = in
		return &application.ListUsersResult{Users: []*domain.User{alice}, Total: 7, Offset: in.Offset, Limit: in.Limit}, nil
	}}
	srv := newServer(t, users, nil)

	rec := srv.do(http.MethodGet, "/api/v1/users?offset=5&limit=1", "", validToken)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body)
	}
	if got.Offset != 5 || got.Limit != 1 {
		t.Errorf("service received %+v", got)
	}
	resp := decode[struct {
		Users []struct{ ID string } `json:"users"`
		Total int64                 `json:"total"`
	}](t, rec)
	if len(resp.Users) != 1 || resp.Users[0].ID != "user-1" || resp.Total != 7 {
		t.Errorf("unexpected body: %s", rec.Body)
	}

	for _, query := range []string{"limit=abc", "limit=1000", "offset=-1"} {
		rec := srv.do(http.MethodGet, "/api/v1/users?"+query, "", validToken)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("?%s: status = %d, want 400", query, rec.Code)
		}
	}
}

func TestUpdateUser(t *testing.T) {
	t.Run("passes only the fields that were sent", func(t *testing.T) {
		var got application.UpdateUserInput
		users := &stubUserService{update: func(_ context.Context, id string, in application.UpdateUserInput) (*domain.User, error) {
			got = in
			return alice, nil
		}}
		srv := newServer(t, users, nil)

		rec := srv.do(http.MethodPatch, "/api/v1/users/user-1", `{"name":"Alice Smith"}`, validToken)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d; body: %s", rec.Code, rec.Body)
		}
		if got.Name == nil || *got.Name != "Alice Smith" || got.Email != nil {
			t.Errorf("service received name=%v email=%v", got.Name, got.Email)
		}
	})

	t.Run("rejects fields that cannot be updated", func(t *testing.T) {
		srv := newServer(t, &stubUserService{}, nil)
		rec := srv.do(http.MethodPatch, "/api/v1/users/user-1", `{"password":"new-password"}`, validToken)
		assertError(t, rec, http.StatusBadRequest, "invalid_request")
	})

	t.Run("rejects an invalid email", func(t *testing.T) {
		srv := newServer(t, &stubUserService{}, nil)
		rec := srv.do(http.MethodPatch, "/api/v1/users/user-1", `{"email":"not-an-email"}`, validToken)
		assertError(t, rec, http.StatusBadRequest, "validation_failed")
	})
}

func TestDeleteUser(t *testing.T) {
	users := &stubUserService{delete: func(_ context.Context, id string) error {
		if id != "user-1" {
			return domain.ErrUserNotFound
		}
		return nil
	}}
	srv := newServer(t, users, nil)

	rec := srv.do(http.MethodDelete, "/api/v1/users/user-1", "", validToken)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status = %d, body = %q; want 204 with empty body", rec.Code, rec.Body)
	}

	assertError(t, srv.do(http.MethodDelete, "/api/v1/users/other", "", validToken), http.StatusNotFound, "not_found")
}

func TestGetMe(t *testing.T) {
	var gotID string
	users := &stubUserService{get: func(_ context.Context, id string) (*domain.User, error) {
		gotID = id
		return alice, nil
	}}
	srv := newServer(t, users, nil)

	rec := srv.do(http.MethodGet, "/api/v1/users/me", "", validToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body)
	}
	if gotID != "user-1" {
		t.Errorf("service was asked for %q, want the authenticated user %q", gotID, "user-1")
	}

	assertError(t, srv.do(http.MethodGet, "/api/v1/users/me", "", ""), http.StatusUnauthorized, "unauthorized")

	// /:id must still work next to the static /me route.
	if rec := srv.do(http.MethodGet, "/api/v1/users/abc", "", validToken); gotID != "abc" || rec.Code != http.StatusOK {
		t.Errorf("GET /users/abc: status %d, service got %q", rec.Code, gotID)
	}
}
