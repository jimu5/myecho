package handler

import (
	"github.com/gofiber/fiber/v2"
)

const (
	Unauthorized  = 4001
	ValidateError = 5001
	LoginError    = 6001
)

const (
	LoginErrorMsg        = "账号或密码错误"
	UnauthorizedErrorMsg = "未登录"
)

type Error struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// 验证失败返回
func ValidateErrorResponse(c *fiber.Ctx, msg string) error {
	return c.Status(403).JSON(Error{Code: ValidateError, Msg: msg})
}

// 鉴权失败返回
func UnauthorizedErrorResponse(c *fiber.Ctx) error {
	return c.Status(401).JSON(Error{Code: Unauthorized, Msg: UnauthorizedErrorMsg})
}
