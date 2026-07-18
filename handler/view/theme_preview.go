package view

import (
	"myecho/service"
	"time"

	"github.com/gofiber/fiber/v2"
)

func ThemePreview(c *fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		return c.Status(fiber.StatusForbidden).SendString("invalid theme preview token")
	}
	if _, err := service.S.Theme.ValidatePreviewToken(token); err != nil {
		return c.Status(fiber.StatusForbidden).SendString("invalid theme preview token")
	}
	c.Cookie(&fiber.Cookie{
		Name:     service.ThemePreviewCookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(service.ThemePreviewTokenTTL),
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   c.Protocol() == "https",
	})
	return c.Redirect(service.SafePreviewPath(c.Query("path")), fiber.StatusFound)
}

func ClearThemePreview(c *fiber.Ctx) error {
	expireThemePreviewCookie(c)
	return c.SendStatus(fiber.StatusNoContent)
}

func expireThemePreviewCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     service.ThemePreviewCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-time.Hour),
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   c.Protocol() == "https",
	})
}
