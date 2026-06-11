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
	return c.Render("category", respToMap(c, categories, PageMeta{
		Description: "文章分类归档",
		Canonical:   absoluteURL(c),
		OGTitle:     "分类",
	}))
}
