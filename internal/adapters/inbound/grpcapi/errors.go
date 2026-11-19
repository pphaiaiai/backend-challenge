package grpcapi

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

// toStatus maps an application error to a gRPC status. Unknown errors are
// logged and returned as a generic INTERNAL error.
func (s *userServer) toStatus(ctx context.Context, err error) error {
	var vErr *domain.ValidationError
	switch {
	case errors.As(err, &vErr):
		return status.Error(codes.InvalidArgument, vErr.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, "user not found")
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return status.Error(codes.AlreadyExists, "email already exists")
	default:
		s.logger.ErrorContext(ctx, "grpc internal error", "error", err)
		return status.Error(codes.Internal, "internal error")
	}
}
