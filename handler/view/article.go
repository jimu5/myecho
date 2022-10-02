package view

import (
	"github.com/gofiber/fiber/v2"
	"myecho/handler"
	"myecho/service"
)

func ArticleDisplayList(c *fiber.Ctx) error {
	queryParam := service.ArticleDisplayListQueryParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	total, data, err := service.S.Article.ArticleDisplayList(&queryParam)
	if err != nil {
		return err
	}
	return c.Render("index", handler.Pagination{Total: total, Data: data})
}
