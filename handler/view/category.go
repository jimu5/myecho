package view

import (
	"github.com/gofiber/fiber/v2"
	"myecho/model"
	"myecho/service"
)

func CategoryArticleAll(c *fiber.Ctx) error {
	categories, err := service.S.Category.AllByType(model.CategoryTypeArticle)
	if err != nil {
		return err
	}
	return c.Render("category", respToMap(categories))
}
