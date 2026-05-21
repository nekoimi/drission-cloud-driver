package errcode

import "fmt"

// 业务错误码体系
// 前缀: 400xx=请求错误, 401xx=未授权, 403xx=禁止, 404xx=不存在, 409xx=冲突, 422xx=验证错误, 500xx=服务器错误

var (
	OK = &Code{Value: 0, Message: "success"}

	// 通用
	BadRequest   = &Code{Value: 40000, Message: "bad request"}
	Unauthorized = &Code{Value: 40100, Message: "unauthorized"}
	Forbidden    = &Code{Value: 40300, Message: "forbidden"}
	NotFound     = &Code{Value: 40400, Message: "not found"}
	Conflict     = &Code{Value: 40900, Message: "conflict"}
	Internal     = &Code{Value: 50000, Message: "internal server error"}
	Validation   = &Code{Value: 42200, Message: "validation error"}
	TooManyReq   = &Code{Value: 42900, Message: "too many requests"}

	// 用户相关 2xxxx
	ErrUserNotFound  = &Code{Value: 40401, Message: "user not found"}
	ErrEmailExists   = &Code{Value: 40901, Message: "email already exists"}
	ErrUsernameExists = &Code{Value: 40902, Message: "username already exists"}
	ErrInvalidCreds  = &Code{Value: 40101, Message: "invalid email or password"}
)

type Code struct {
	Value   int    `json:"code"`
	Message string `json:"message"`
}

func (c *Code) WithMessage(msg string) *Code {
	return &Code{Value: c.Value, Message: msg}
}

// AppError 业务错误，支持 error wrapping
type AppError struct {
	Code   *Code  `json:"code"`
	Detail string `json:"detail,omitempty"`
	Err    error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Code.Value, e.Code.Message, e.Detail)
	}
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %s", e.Code.Value, e.Code.Message, e.Err.Error())
	}
	return fmt.Sprintf("[%d] %s", e.Code.Value, e.Code.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// New 创建业务错误
func New(code *Code) *AppError {
	return &AppError{Code: code}
}

// NewWithDetail 创建带详情的业务错误
func NewWithDetail(code *Code, detail string) *AppError {
	return &AppError{Code: code, Detail: detail}
}

// Wrap 创建包装底层 error 的业务错误
func Wrap(code *Code, err error) *AppError {
	return &AppError{Code: code, Err: err}
}
