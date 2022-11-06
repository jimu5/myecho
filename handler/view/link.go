package view

import (
	"github.com/gofiber/fiber/v2"
	"myecho/service"
)

func LinkAll(c *fiber.Ctx) error {
	links, err := service.S.Link.All(nil)
	if err != nil {
		return err
	}
	return c.Render("link", respToMap(links))
}
