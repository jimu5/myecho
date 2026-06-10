package api

import (
	"github.com/gofiber/fiber/v2"
)

const (
	CommonBadError = 4001
	ParseError     = 4002

	Unauthorized = 4011
	NotFound     = 4041

	ValidateError = 4031
	LoginError    = 4032

	InternalSQLError = 5001
)

const (
	LoginErrorMsg        = "账号或密码错误"
	UnauthorizedErrorMsg = "未登录"
	CanNotRegister       = "禁止注册"
	SuccessMsg           = "ok"
)

type Error struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func ErrorResponse(c *fiber.Ctx, status, code int, msg string) error {
	return c.Status(status).JSON(Error{Code: code, Msg: msg})
}

// 解析失败返回
func ParseErrorResponse(c *fiber.Ctx, msg string) error {
	return ErrorResponse(c, fiber.StatusBadRequest, ParseError, msg)
}

// 未找到返回
func NotFoundErrorResponse(c *fiber.Ctx, msg string) error {
	return ErrorResponse(c, fiber.StatusNotFound, NotFound, msg)
}

// 验证失败返回
func ValidateErrorResponse(c *fiber.Ctx, msg string) error {
	return ErrorResponse(c, fiber.StatusForbidden, ValidateError, msg)
}

// 鉴权失败返回
func UnauthorizedErrorResponse(c *fiber.Ctx) error {
	return ErrorResponse(c, fiber.StatusUnauthorized, Unauthorized, UnauthorizedErrorMsg)
}

// 内部错误
func InternalErrorResponse(c *fiber.Ctx, code int, msg string) error {
	return ErrorResponse(c, fiber.StatusInternalServerError, code, msg)
}
