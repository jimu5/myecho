package middleware

import (
	"myecho/model"

	"github.com/gofiber/fiber/v2"
)

// 定义错误响应结构
type Error struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

const (
	Unauthorized         = 4011
	UnauthorizedErrorMsg = "未登录"
	Forbidden            = 4033
	ForbiddenErrorMsg    = "无权限"
)

// 未授权错误响应
func unauthorizedErrorResponse(c *fiber.Ctx) error {
	return c.Status(401).JSON(Error{Code: Unauthorized, Msg: UnauthorizedErrorMsg})
}

func forbiddenErrorResponse(c *fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).JSON(Error{Code: Forbidden, Msg: ForbiddenErrorMsg})
}

func AdminRequired(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*model.User)
	if !ok || user == nil {
		return unauthorizedErrorResponse(c)
	}
	if user.PermissionType != model.Admin {
		return forbiddenErrorResponse(c)
	}
	return c.Next()
}
