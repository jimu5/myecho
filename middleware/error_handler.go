package middleware

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const (
	CommonBadError = 4001
)

func Custom404ErrorHandler(c *fiber.Ctx) error {
	if isJSONRoute(c.Path()) {
		return c.Status(fiber.StatusNotFound).JSON(map[string]interface{}{"code": 4041, "msg": "not found"})
	}
	if strings.HasPrefix(c.Path(), "/admin") {
		adminIndex := "./static/admin/index.html"
		if _, err := os.Stat(adminIndex); err == nil {
			return c.Status(fiber.StatusOK).SendFile(adminIndex)
		}
		return c.Status(fiber.StatusNotFound).JSON(map[string]interface{}{"code": 4041, "msg": "admin frontend build not found"})
	}
	return c.Status(fiber.StatusNotFound).SendString("not found")
}

func CommonErrorHandler(c *fiber.Ctx) error {
	err := c.Next()
	if err != nil && isJSONRoute(c.Path()) {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]interface{}{"code": CommonBadError, "msg": err.Error()})
	}
	return err
}

func isJSONRoute(path string) bool {
	return strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/mos")
}
