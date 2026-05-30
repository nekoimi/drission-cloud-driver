package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/errcode"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/response"
)

func badRequest(c *gin.Context, msg string) {
	response.ErrorWithMsg(c, http.StatusBadRequest, errcode.ErrInvalidRequest, msg)
}

func validationError(c *gin.Context, err error) {
	response.ValidationError(c, err.Error())
}

func notFound(c *gin.Context, code *errcode.Code, err error) {
	response.ErrorWithMsg(c, http.StatusNotFound, code, err.Error())
}

func operationFailed(c *gin.Context, err error) {
	if appErr, ok := response.IsAppError(err); ok {
		response.AppErr(c, appErr)
		return
	}

	response.ErrorWithMsg(c, http.StatusInternalServerError, errcode.ErrOperationFailed, err.Error())
}

func appError(c *gin.Context, err error) {
	if appErr, ok := response.IsAppError(err); ok {
		response.AppErr(c, appErr)
		return
	}

	operationFailed(c, err)
}
