package view

import "github.com/gofiber/fiber/v2"

func NotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).Render("404", respToMap(c, nil, PageMeta{
		Description: "页面不存在",
		Canonical:   absoluteURL(c),
		OGTitle:     "页面不存在",
	}))
}
