package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nekoimi/go-project-template/internal/pkg/errcode"
)

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Code:    errcode.OK.Value,
		Message: errcode.OK.Message,
		Data:    data,
	})
}

func Error(c *gin.Context, httpStatus int, code *errcode.Code) {
	c.JSON(httpStatus, APIResponse{
		Code:    code.Value,
		Message: code.Message,
	})
}

func ErrorWithMsg(c *gin.Context, httpStatus int, code *errcode.Code, msg string) {
	c.JSON(httpStatus, APIResponse{
		Code:    code.Value,
		Message: msg,
	})
}

// AppErr 根据 AppError 返回响应，自动映射 HTTP 状态码
func AppErr(c *gin.Context, appErr *errcode.AppError) {
	httpStatus := httpStatusFromCode(appErr.Code.Value)
	resp := APIResponse{
		Code:    appErr.Code.Value,
		Message: appErr.Code.Message,
	}
	if appErr.Detail != "" {
		resp.Error = appErr.Detail
	} else if appErr.Err != nil {
		resp.Error = appErr.Err.Error()
	}
	c.JSON(httpStatus, resp)
}

// ValidationError 返回验证错误
func ValidationError(c *gin.Context, details interface{}) {
	c.JSON(http.StatusUnprocessableEntity, APIResponse{
		Code:    errcode.Validation.Value,
		Message: errcode.Validation.Message,
		Error:   details,
	})
}

// httpStatusFromCode 根据业务错误码前缀映射 HTTP 状态码
func httpStatusFromCode(code int) int {
	switch {
	case code >= 40000 && code < 40100:
		return http.StatusBadRequest
	case code >= 40100 && code < 40200:
		return http.StatusUnauthorized
	case code >= 40300 && code < 40400:
		return http.StatusForbidden
	case code >= 40400 && code < 40500:
		return http.StatusNotFound
	case code >= 40900 && code < 41000:
		return http.StatusConflict
	case code >= 42200 && code < 42300:
		return http.StatusUnprocessableEntity
	case code >= 42900 && code < 43000:
		return http.StatusTooManyRequests
	case code >= 50000:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// IsAppError 检查 error 是否为 AppError
func IsAppError(err error) (*errcode.AppError, bool) {
	var appErr *errcode.AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
