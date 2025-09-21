package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// 定义错误响应结构
 type Error struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

const (
	Unauthorized = 4011
	UnauthorizedErrorMsg = "未登录"
)

// 未授权错误响应
func unauthorizedErrorResponse(c *fiber.Ctx) error {
	return c.Status(401).JSON(Error{Code: Unauthorized, Msg: UnauthorizedErrorMsg})
}

func Authentication(c *fiber.Ctx) (err error) {
	auth := c.Get("Authorization")
	if auth == "" {
		return unauthorizedErrorResponse(c)
	}
	token := strings.Fields(auth)[1]
	user, err := GetUserByToken(token)
	if err != nil {
		return unauthorizedErrorResponse(c)
	}

	// 将用户信息保存下来
	c.Locals("user", &user)
	return c.Next()
}
