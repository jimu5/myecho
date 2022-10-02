package middleware

import (
	"myecho/handler/api"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func Authentication(c *fiber.Ctx) (err error) {
	auth := c.Get("Authorization")
	if auth == "" {
		return api.UnauthorizedErrorResponse(c)
	}
	token := strings.Fields(auth)[1]
	user, err := GetUserByToken(token)
	if err != nil {
		return api.UnauthorizedErrorResponse(c)
	}

	// 将用户信息保存下来
	c.Locals("user", &user)
	return c.Next()
}
