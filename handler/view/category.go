package view

import (
	"github.com/gofiber/fiber/v2"
	"myecho/service"
)

func CategoryAll(c *fiber.Ctx) error {
	categories, err := service.S.Category.All()
	if err != nil {
		return err
	}
	return c.Render("category", respToMap(categories))
}
