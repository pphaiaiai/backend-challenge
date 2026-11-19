package grpcapi

import (
	"context"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// loggingInterceptor logs the method, status code and duration of every call.
func loggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		code := status.Code(err)
		level := slog.LevelInfo
		if code == codes.Internal || code == codes.Unknown {
			level = slog.LevelError
		}
		logger.Log(ctx, level, "grpc request",
			"method", info.FullMethod,
			"code", code.String(),
			"duration_ms", float64(time.Since(start).Microseconds())/1000,
		)
		return resp, err
	}
}

// recoveryInterceptor turns a panic into an INTERNAL error and logs the stack.
func recoveryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorContext(ctx, "panic recovered",
					"method", info.FullMethod, "panic", r, "stack", string(debug.Stack()))
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

// authInterceptor requires the metadata "authorization: Bearer <token>".
func authInterceptor(auth Authenticator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		token, ok := bearerToken(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing or malformed authorization metadata")
		}
		if _, err := auth.Authenticate(ctx, token); err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}
		return handler(ctx, req)
	}
}

func bearerToken(ctx context.Context) (string, bool) {
	md, _ := metadata.FromIncomingContext(ctx)
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", false
	}
	scheme, token, ok := strings.Cut(values[0], " ")
	token = strings.TrimSpace(token)
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}
