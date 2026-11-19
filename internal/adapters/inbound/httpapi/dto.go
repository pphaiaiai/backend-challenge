package httpapi

import (
	"time"

	"github.com/pphaiaiai/backend-challenge/internal/application"
	"github.com/pphaiaiai/backend-challenge/internal/domain"
)

// createUserRequest is the body of register and create-user requests.
type createUserRequest struct {
	Name     string `json:"name" binding:"required,max=100"`
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// updateUserRequest uses pointers so that omitted fields stay unchanged.
type updateUserRequest struct {
	Name  *string `json:"name" binding:"omitempty,max=100"`
	Email *string `json:"email" binding:"omitempty,email,max=254"`
}

type listUsersQuery struct {
	Offset int64 `form:"offset" binding:"min=0"`
	Limit  int64 `form:"limit" binding:"min=0,max=100"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type listUsersResponse struct {
	Users  []userResponse `json:"users"`
	Total  int64          `json:"total"`
	Offset int64          `json:"offset"`
	Limit  int64          `json:"limit"`
}

type tokenResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int64     `json:"expires_in"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func toUserResponse(u *domain.User) userResponse {
	return userResponse{ID: u.ID, Name: u.Name, Email: u.Email, CreatedAt: u.CreatedAt}
}

func toListUsersResponse(r *application.ListUsersResult) listUsersResponse {
	users := make([]userResponse, 0, len(r.Users))
	for _, u := range r.Users {
		users = append(users, toUserResponse(u))
	}
	return listUsersResponse{Users: users, Total: r.Total, Offset: r.Offset, Limit: r.Limit}
}

func toTokenResponse(t application.Token) tokenResponse {
	return tokenResponse{
		AccessToken: t.Value,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(t.ExpiresAt).Round(time.Second).Seconds()),
		ExpiresAt:   t.ExpiresAt,
	}
}
