package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pphaiaiai/backend-challenge/internal/application"
)

// createUser handles both public registration and the protected create-user
// endpoint; they differ only in the auth middleware in front of them.
func (h *handler) createUser(c *gin.Context) {
	var req createUserRequest
	if err := bindJSON(c, &req); err != nil {
		writeBindError(c, err)
		return
	}

	user, err := h.users.Create(c.Request.Context(), application.CreateUserInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeError(c, err)
		return
	}

	c.Header("Location", "/api/v1/users/"+user.ID)
	c.JSON(http.StatusCreated, toUserResponse(user))
}

func (h *handler) login(c *gin.Context) {
	var req loginRequest
	if err := bindJSON(c, &req); err != nil {
		writeBindError(c, err)
		return
	}

	token, err := h.auth.Login(c.Request.Context(), application.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTokenResponse(token))
}

func (h *handler) getUser(c *gin.Context) {
	user, err := h.users.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *handler) listUsers(c *gin.Context) {
	var q listUsersQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		writeBindError(c, err)
		return
	}

	result, err := h.users.List(c.Request.Context(), application.ListUsersInput{
		Offset: q.Offset,
		Limit:  q.Limit,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toListUsersResponse(result))
}

func (h *handler) updateUser(c *gin.Context) {
	var req updateUserRequest
	if err := bindJSON(c, &req); err != nil {
		writeBindError(c, err)
		return
	}

	user, err := h.users.Update(c.Request.Context(), c.Param("id"), application.UpdateUserInput{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *handler) deleteUser(c *gin.Context) {
	if err := h.users.Delete(c.Request.Context(), c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
