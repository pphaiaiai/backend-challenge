// Package httpapi exposes the application over HTTP using Gin.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pphaiaiai/backend-challenge/internal/application"
	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

// maxBodyBytes limits the size of request bodies.
const maxBodyBytes = 1 << 20 // 1 MiB

// UserService is the subset of application.UserService used by the handlers.
type UserService interface {
	Create(ctx context.Context, in application.CreateUserInput) (*domain.User, error)
	Get(ctx context.Context, id string) (*domain.User, error)
	List(ctx context.Context, in application.ListUsersInput) (*application.ListUsersResult, error)
	Update(ctx context.Context, id string, in application.UpdateUserInput) (*domain.User, error)
	Delete(ctx context.Context, id string) error
}

// AuthService is the subset of application.AuthService used by the handlers.
type AuthService interface {
	Login(ctx context.Context, in application.LoginInput) (application.Token, error)
	Authenticate(ctx context.Context, token string) (string, error)
}

// NewRouter returns the HTTP handler for the API.
func NewRouter(users UserService, auth AuthService, logger *slog.Logger) *gin.Engine {
	useJSONFieldNames()
	h := &handler{users: users, auth: auth}

	r := gin.New()
	r.HandleMethodNotAllowed = true
	// Trust no proxy headers: the client IP is the connection's address.
	_ = r.SetTrustedProxies(nil)

	r.Use(RequestLogger(logger), Recovery(logger), LimitBodySize(maxBodyBytes))
	r.NoRoute(func(c *gin.Context) {
		abortWithError(c, http.StatusNotFound, codeNotFound, "route not found")
	})
	r.NoMethod(func(c *gin.Context) {
		abortWithError(c, http.StatusMethodNotAllowed, codeMethodNotAllowed, "method not allowed")
	})

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	v1.POST("/auth/register", h.createUser)
	v1.POST("/auth/login", h.login)

	protected := v1.Group("/users", RequireAuth(auth))
	protected.POST("", h.createUser)
	protected.GET("", h.listUsers)
	protected.GET("/:id", h.getUser)
	protected.PATCH("/:id", h.updateUser)
	protected.DELETE("/:id", h.deleteUser)

	return r
}

type handler struct {
	users UserService
	auth  AuthService
}
