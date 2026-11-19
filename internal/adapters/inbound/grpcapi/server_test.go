package grpcapi_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	userv1 "github.com/pphaiaiai/backend-challenge/gen/user/v1"
	"github.com/pphaiaiai/backend-challenge/internal/adapters/inbound/grpcapi"
	"github.com/pphaiaiai/backend-challenge/internal/application"
	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

const validToken = "valid-token"

var alice = &domain.User{
	ID:        "user-1",
	Name:      "Alice",
	Email:     "alice@example.com",
	CreatedAt: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC),
}

type stubUsers struct {
	create func(context.Context, application.CreateUserInput) (*domain.User, error)
	get    func(context.Context, string) (*domain.User, error)
}

func (s *stubUsers) Create(ctx context.Context, in application.CreateUserInput) (*domain.User, error) {
	return s.create(ctx, in)
}

func (s *stubUsers) Get(ctx context.Context, id string) (*domain.User, error) {
	return s.get(ctx, id)
}

type stubAuth struct{}

func (stubAuth) Authenticate(_ context.Context, token string) (string, error) {
	if token != validToken {
		return "", domain.ErrInvalidToken
	}
	return "user-1", nil
}

// newClient serves the API over an in-memory connection and returns a client.
func newClient(t *testing.T, users *stubUsers) (userv1.UserServiceClient, *bytes.Buffer) {
	t.Helper()
	logs := &bytes.Buffer{}
	srv := grpcapi.NewServer(users, stubAuth{}, slog.New(slog.NewTextHandler(logs, nil)), false)

	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return userv1.NewUserServiceClient(conn), logs
}

func withToken(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}

func assertCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code = %s, want %s (err: %v)", got, want, err)
	}
}

func TestCreateUser(t *testing.T) {
	var got application.CreateUserInput
	client, _ := newClient(t, &stubUsers{create: func(_ context.Context, in application.CreateUserInput) (*domain.User, error) {
		got = in
		return alice, nil
	}})

	resp, err := client.CreateUser(withToken(validToken), &userv1.CreateUserRequest{
		Name: "Alice", Email: "alice@example.com", Password: "password123",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if got.Name != "Alice" || got.Email != "alice@example.com" || got.Password != "password123" {
		t.Errorf("service received %+v", got)
	}
	u := resp.GetUser()
	if u.GetId() != "user-1" || u.GetEmail() != "alice@example.com" || !u.GetCreatedAt().AsTime().Equal(alice.CreatedAt) {
		t.Errorf("unexpected user: %v", u)
	}
}

func TestCreateUser_Errors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{"validation", &domain.ValidationError{Field: "email", Message: "must be a valid email address"}, codes.InvalidArgument},
		{"duplicate email", domain.ErrEmailAlreadyExists, codes.AlreadyExists},
		{"unexpected", errors.New("database exploded"), codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newClient(t, &stubUsers{create: func(context.Context, application.CreateUserInput) (*domain.User, error) {
				return nil, tt.err
			}})

			_, err := client.CreateUser(withToken(validToken), &userv1.CreateUserRequest{})

			assertCode(t, err, tt.code)
			if strings.Contains(err.Error(), "exploded") {
				t.Errorf("internal error leaked to client: %v", err)
			}
		})
	}
}

func TestGetUser(t *testing.T) {
	client, _ := newClient(t, &stubUsers{get: func(_ context.Context, id string) (*domain.User, error) {
		if id == "user-1" {
			return alice, nil
		}
		return nil, domain.ErrUserNotFound
	}})

	resp, err := client.GetUser(withToken(validToken), &userv1.GetUserRequest{Id: "user-1"})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if resp.GetUser().GetName() != "Alice" {
		t.Errorf("unexpected user: %v", resp.GetUser())
	}

	_, err = client.GetUser(withToken(validToken), &userv1.GetUserRequest{Id: "missing"})
	assertCode(t, err, codes.NotFound)
}

func TestAuthentication(t *testing.T) {
	called := false
	client, _ := newClient(t, &stubUsers{get: func(context.Context, string) (*domain.User, error) {
		called = true
		return alice, nil
	}})

	tests := []struct {
		name string
		ctx  context.Context
	}{
		{"no metadata", context.Background()},
		{"wrong scheme", metadata.AppendToOutgoingContext(context.Background(), "authorization", "Basic abc")},
		{"invalid token", withToken("forged")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetUser(tt.ctx, &userv1.GetUserRequest{Id: "user-1"})
			assertCode(t, err, codes.Unauthenticated)
		})
	}
	if called {
		t.Error("service was called without a valid token")
	}
}

func TestRecoveryAndLogging(t *testing.T) {
	client, logs := newClient(t, &stubUsers{get: func(context.Context, string) (*domain.User, error) {
		panic("boom")
	}})

	_, err := client.GetUser(withToken(validToken), &userv1.GetUserRequest{Id: "user-1"})

	assertCode(t, err, codes.Internal)
	for _, want := range []string{"panic recovered", "method=/user.v1.UserService/GetUser", "code=Internal"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("logs do not contain %q:\n%s", want, logs)
		}
	}
}

func TestReflection(t *testing.T) {
	srv := grpcapi.NewServer(&stubUsers{}, stubAuth{}, slog.Default(), true)
	if _, ok := srv.GetServiceInfo()["grpc.reflection.v1.ServerReflection"]; !ok {
		t.Error("reflection service is not registered")
	}
}
