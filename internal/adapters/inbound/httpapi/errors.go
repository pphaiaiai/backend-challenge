package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

// Error codes returned in the "code" field of error responses.
const (
	codeInvalidRequest     = "invalid_request"
	codeValidationFailed   = "validation_failed"
	codeUnauthorized       = "unauthorized"
	codeInvalidCredentials = "invalid_credentials"
	codeInvalidToken       = "invalid_token"
	codeNotFound           = "not_found"
	codeMethodNotAllowed   = "method_not_allowed"
	codeEmailExists        = "email_already_exists"
	codeBodyTooLarge       = "request_too_large"
	codeInternal           = "internal_error"
)

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Fields  []fieldError `json:"fields,omitempty"`
}

type fieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func abortWithError(c *gin.Context, status int, code, message string, fields ...fieldError) {
	c.AbortWithStatusJSON(status, errorResponse{Error: errorBody{Code: code, Message: message, Fields: fields}})
}

// writeError maps an application error to an HTTP response. Unknown errors
// become a generic 500 so internal details never reach the client.
func writeError(c *gin.Context, err error) {
	var vErr *domain.ValidationError
	switch {
	case errors.As(err, &vErr):
		abortWithError(c, http.StatusBadRequest, codeValidationFailed, "request validation failed",
			fieldError{Field: vErr.Field, Message: vErr.Message})
	case errors.Is(err, domain.ErrUserNotFound):
		abortWithError(c, http.StatusNotFound, codeNotFound, "user not found")
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		abortWithError(c, http.StatusConflict, codeEmailExists, "email already exists")
	case errors.Is(err, domain.ErrInvalidCredentials):
		abortWithError(c, http.StatusUnauthorized, codeInvalidCredentials, "invalid email or password")
	case errors.Is(err, domain.ErrInvalidToken):
		abortWithError(c, http.StatusUnauthorized, codeInvalidToken, "invalid or expired token")
	default:
		_ = c.Error(err) // recorded for the request log
		abortWithError(c, http.StatusInternalServerError, codeInternal, "internal server error")
	}
}

// bindJSON decodes the request body into dst, rejecting unknown fields, and
// validates it against its binding tags.
func bindJSON(c *gin.Context, dst any) error {
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return binding.Validator.ValidateStruct(dst)
}

// writeBindError maps a decoding or binding-validation error to a 4xx response.
func writeBindError(c *gin.Context, err error) {
	var vErrs validator.ValidationErrors
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &vErrs):
		fields := make([]fieldError, 0, len(vErrs))
		for _, fe := range vErrs {
			fields = append(fields, fieldError{Field: fe.Field(), Message: fieldMessage(fe)})
		}
		abortWithError(c, http.StatusBadRequest, codeValidationFailed, "request validation failed", fields...)
	case errors.As(err, &tooLarge):
		abortWithError(c, http.StatusRequestEntityTooLarge, codeBodyTooLarge, "request body is too large")
	case errors.Is(err, io.EOF):
		abortWithError(c, http.StatusBadRequest, codeInvalidRequest, "request body is required")
	default:
		msg := strings.TrimPrefix(err.Error(), "json: ")
		abortWithError(c, http.StatusBadRequest, codeInvalidRequest, "invalid request: "+msg)
	}
}

func fieldMessage(fe validator.FieldError) string {
	unit := ""
	if fe.Kind() == reflect.String {
		unit = " characters"
	}
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + fe.Param() + unit
	case "max":
		return "must be at most " + fe.Param() + unit
	default:
		return "is invalid"
	}
}

var jsonNamesOnce sync.Once

// useJSONFieldNames makes validation errors report JSON or query field names
// ("email") instead of Go field names ("Email").
func useJSONFieldNames() {
	jsonNamesOnce.Do(func() {
		v, ok := binding.Validator.Engine().(*validator.Validate)
		if !ok {
			return
		}
		v.RegisterTagNameFunc(func(f reflect.StructField) string {
			for _, tag := range []string{"json", "form"} {
				if name, _, _ := strings.Cut(f.Tag.Get(tag), ","); name != "" && name != "-" {
					return name
				}
			}
			return f.Name
		})
	})
}
