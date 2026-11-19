package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const userIDKey = "userID"

// RequestLogger logs the method, path, status and duration of every request.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"duration_ms", float64(time.Since(start).Microseconds()) / 1000,
			"client_ip", c.ClientIP(),
		}
		if userID := c.GetString(userIDKey); userID != "" {
			attrs = append(attrs, "user_id", userID)
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.Errors())
		}

		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		logger.Log(c.Request.Context(), level, "http request", attrs...)
	}
}

// Recovery turns a panic into a 500 JSON response and logs the stack trace.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		logger.Error("panic recovered", "panic", recovered, "stack", string(debug.Stack()))
		abortWithError(c, http.StatusInternalServerError, codeInternal, "internal server error")
	})
}

// LimitBodySize rejects request bodies larger than n bytes.
func LimitBodySize(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}

// RequireAuth accepts the request only with a valid "Authorization: Bearer
// <token>" header, and stores the authenticated user ID in the context.
func RequireAuth(auth AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			c.Header("WWW-Authenticate", "Bearer")
			abortWithError(c, http.StatusUnauthorized, codeUnauthorized, "missing or malformed Authorization header")
			return
		}

		userID, err := auth.Authenticate(c.Request.Context(), token)
		if err != nil {
			c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
			writeError(c, err)
			return
		}

		c.Set(userIDKey, userID)
		c.Next()
	}
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	token = strings.TrimSpace(token)
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}
