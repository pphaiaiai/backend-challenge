// Command api runs the user management service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/pphaiaiai/backend-challenge/internal/adapters/inbound/grpcapi"
	"github.com/pphaiaiai/backend-challenge/internal/adapters/inbound/httpapi"
	"github.com/pphaiaiai/backend-challenge/internal/adapters/inbound/worker"
	"github.com/pphaiaiai/backend-challenge/internal/adapters/outbound/mongodb"
	"github.com/pphaiaiai/backend-challenge/internal/adapters/outbound/password"
	"github.com/pphaiaiai/backend-challenge/internal/adapters/outbound/token"
	"github.com/pphaiaiai/backend-challenge/internal/application"
	"github.com/pphaiaiai/backend-challenge/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("service stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	// ctx is cancelled on SIGINT (Ctrl+C) or SIGTERM (docker stop).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	client, err := mongodb.Connect(connectCtx, cfg.MongoURI, cfg.MongoTimeout)
	cancel()
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Disconnect(ctx); err != nil {
			logger.Error("disconnect mongodb", "error", err)
		}
	}()

	repo := mongodb.NewUserRepository(client.Database(cfg.MongoDatabase))
	if err := repo.EnsureIndexes(ctx); err != nil {
		return err
	}

	hasher, err := password.NewBcryptHasher(cfg.BcryptCost)
	if err != nil {
		return err
	}
	tokens, err := token.NewJWTService(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	if err != nil {
		return err
	}

	users := application.NewUserService(repo, hasher)
	auth, err := application.NewAuthService(repo, hasher, tokens)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(users, auth, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	grpcServer := grpcapi.NewServer(users, auth, logger, cfg.GRPCReflection)
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("grpc listen: %w", err)
	}
	reporter := worker.NewUserCountReporter(users, cfg.UserCountInterval, logger)

	// If any component fails, gctx is cancelled and the others shut down too.
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		logger.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		logger.Info("grpc server listening", "addr", cfg.GRPCAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			return fmt.Errorf("grpc server: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		reporter.Run(gctx)
		return nil
	})

	g.Go(func() error {
		<-gctx.Done()
		logger.Info("shutting down", "timeout", cfg.ShutdownTimeout.String())

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		// Both servers stop accepting new work and wait for in-flight calls.
		grpcStopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(grpcStopped)
		}()
		httpErr := httpServer.Shutdown(shutdownCtx)

		select {
		case <-grpcStopped:
		case <-shutdownCtx.Done():
			grpcServer.Stop() // timeout: close remaining connections
		}
		if httpErr != nil {
			return fmt.Errorf("http shutdown: %w", httpErr)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}
	logger.Info("shutdown complete")
	return nil
}
