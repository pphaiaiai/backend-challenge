// Package grpcapi exposes the application over gRPC.
package grpcapi

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/timestamppb"

	userv1 "github.com/pphaiaiai/backend-challenge/gen/user/v1"
	"github.com/pphaiaiai/backend-challenge/internal/application"
	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

// UserService is the subset of application.UserService used by the server.
type UserService interface {
	Create(ctx context.Context, in application.CreateUserInput) (*domain.User, error)
	Get(ctx context.Context, id string) (*domain.User, error)
}

// Authenticator verifies access tokens.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (string, error)
}

// NewServer returns a gRPC server with the UserService registered. Every
// unary call is logged, recovered from panics and authenticated.
func NewServer(users UserService, auth Authenticator, logger *slog.Logger, enableReflection bool) *grpc.Server {
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(
		loggingInterceptor(logger),
		recoveryInterceptor(logger),
		authInterceptor(auth),
	))
	userv1.RegisterUserServiceServer(srv, &userServer{users: users, logger: logger})
	if enableReflection {
		// Lets tools such as grpcurl discover the API without the .proto file.
		reflection.Register(srv)
	}
	return srv
}

type userServer struct {
	userv1.UnimplementedUserServiceServer
	users  UserService
	logger *slog.Logger
}

func (s *userServer) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	user, err := s.users.Create(ctx, application.CreateUserInput{
		Name:     req.GetName(),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &userv1.CreateUserResponse{User: toProtoUser(user)}, nil
}

func (s *userServer) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	user, err := s.users.Get(ctx, req.GetId())
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &userv1.GetUserResponse{User: toProtoUser(user)}, nil
}

func toProtoUser(u *domain.User) *userv1.User {
	return &userv1.User{
		Id:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: timestamppb.New(u.CreatedAt),
	}
}
