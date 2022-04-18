package handler

import (
	"github.com/Kimiato/myecho/config"
	"github.com/Kimiato/myecho/model"
	"github.com/gofiber/fiber/v2"
)

func ListArticle(c *fiber.Ctx) error {
	var articles []model.Article
	var total int64
	// 总数
	config.Database.Find(&[]model.Article{}).Count(&total)
	// 分页查询
	config.Database.Scopes(Paginate(c)).Find(&articles)
	return PaginateData(c, total, articles)
}
