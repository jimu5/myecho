package middleware

import (
	"github.com/gofiber/fiber/v2"

	"myecho/model"
)

func AuthorRequired(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*model.User)
	if !ok || user == nil {
		return unauthorizedErrorResponse(c)
	}
	if !model.IsValidPermissionType(user.PermissionType) {
		return forbiddenErrorResponse(c)
	}
	return c.Next()
}

func EditorRequired(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*model.User)
	if !ok || user == nil {
		return unauthorizedErrorResponse(c)
	}
	if !user.CanEditAllContent() {
		return forbiddenErrorResponse(c)
	}
	return c.Next()
}
