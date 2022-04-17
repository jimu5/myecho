package handler

import (
	"github.com/gofiber/fiber/v2"
)

const (
	ValidateError = 5001
	LoginError    = 6001
)

const (
	LoginErrorMsg = "账号或密码错误"
)

type Error struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// 验证失败返回
func ValidateErrorResponse(c *fiber.Ctx, msg string) error {
	return c.Status(403).JSON(Error{Code: ValidateError, Msg: msg})
}
