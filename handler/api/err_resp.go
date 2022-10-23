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
)

type Error struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// 解析失败返回
func ParseErrorResponse(c *fiber.Ctx, msg string) error {
	return c.Status(400).JSON(Error{Code: ParseError, Msg: msg})
}

// 未找到返回
func NotFoundErrorResponse(c *fiber.Ctx, msg string) error {
	return c.Status(404).JSON(Error{Code: 404, Msg: msg})
}

// 验证失败返回
func ValidateErrorResponse(c *fiber.Ctx, msg string) error {
	return c.Status(403).JSON(Error{Code: ValidateError, Msg: msg})
}

// 鉴权失败返回
func UnauthorizedErrorResponse(c *fiber.Ctx) error {
	return c.Status(401).JSON(Error{Code: Unauthorized, Msg: UnauthorizedErrorMsg})
}

// 内部错误
func InternalErrorResponse(c *fiber.Ctx, code int, msg string) error {
	return c.Status(500).JSON(Error{Code: code, Msg: msg})
}
