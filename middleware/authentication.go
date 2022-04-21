package middleware

import (
	"strings"

	"github.com/Kimiato/myecho/handler"
	"github.com/gofiber/fiber/v2"
)

func Authentication(c *fiber.Ctx) (err error) {
	auth := c.Get("Authorization")
	if auth == "" {
		return handler.UnauthorizedErrorResponse(c)
	}
	token := strings.Fields(auth)[1]
	user, err := GetUserByToken(token)
	if err != nil {
		return handler.UnauthorizedErrorResponse(c)
	}

	// 将用户信息保存下来
	c.Locals("user", &user)
	return c.Next()
}
