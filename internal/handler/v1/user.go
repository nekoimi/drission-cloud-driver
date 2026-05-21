package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/go-project-template/internal/pkg/errcode"
	"github.com/nekoimi/go-project-template/internal/pkg/response"
	"github.com/nekoimi/go-project-template/internal/service"
)

type UserHandler struct {
	userService service.UserService
	logger      *zap.Logger
}

func NewUserHandler(userService service.UserService, logger *zap.Logger) *UserHandler {
	return &UserHandler{userService: userService, logger: logger}
}

// GetProfile godoc
// @Summary      Get current user profile
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.APIResponse{data=dto.UserResponse}
// @Failure      401  {object}  response.APIResponse
// @Failure      404  {object}  response.APIResponse
// @Router       /users/profile [get]
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.AppErr(c, errcode.New(errcode.Unauthorized))
		return
	}

	uidStr, ok := userID.(string)
	if !ok || uidStr == "" {
		response.AppErr(c, errcode.New(errcode.Unauthorized))
		return
	}

	profile, err := h.userService.GetProfile(c.Request.Context(), uidStr)
	if err != nil {
		if appErr, ok := response.IsAppError(err); ok {
			response.AppErr(c, appErr)
			return
		}
		h.logger.Error("get profile failed", zap.Error(err))
		response.ErrorWithMsg(c, http.StatusInternalServerError, errcode.Internal, "internal error")
		return
	}

	response.Success(c, profile)
}
