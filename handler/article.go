package handler

import (
	"myecho/config"
	"myecho/handler/rtype"
	"myecho/handler/validator"
	"myecho/model"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm/clause"
)

func ArticleList(c *fiber.Ctx) error {
	var articlesRes []rtype.ArticleResponse
	var total int64
	// 总数
	config.Database.Find(&[]model.Article{}).Count(&total)
	// 分页查询
	config.Database.Table("articles").Scopes(Paginate(c)).Preload(clause.Associations).Find(&articlesRes)
	return PaginateData(c, total, &articlesRes)
}

func ArticleRetrieve(c *fiber.Ctx) error {
	var article *model.Article
	if err := DetailPreHandle(c, &article); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	var res rtype.ArticleResponse
	res.ID = article.ID
	// TODO: 这里可以优化，减少一次sql查询
	config.Database.Table("articles").Preload(clause.Associations).Find(&res)
	return c.JSON(&res)
}

func ArticleCreate(c *fiber.Ctx) error {
	var article model.Article
	var detail model.ArticleDetail
	var res rtype.ArticleResponse
	var r rtype.ArticleRequest
	if err := c.BodyParser(&r); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	// 校验
	err := validator.ValidateArticleRequest(&r)
	if err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	detail.Content = r.Content
	structAssign(&article, &r)
	article.Detail = &detail
	article.AuthorID = c.Locals("user").(*model.User).ID
	article.Author = c.Locals("user").(*model.User)

	article.Tags = getTags(r.TagIDs)

	config.Database.Preload(clause.Associations).Create(&article).Scan(&res)
	return c.Status(fiber.StatusCreated).JSON(&res)
}

// 更新文章
func ArticleUpdate(c *fiber.Ctx) error {
	var article model.Article
	var r rtype.ArticleRequest
	if err := c.BodyParser(&r); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	// 校验
	if err := DetailPreHandle(c, &article); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if err := validator.ValidateArticleRequest(&r); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}

	var detail model.ArticleDetail
	if result := config.Database.Where("id = ?", article.DetailID).First(&detail); result.Error != nil {
		return InternalErrorResponse(c, InternalSQLError, result.Error.Error())
	}
	detail.Content = r.Content
	if result := config.Database.Save(&detail); result.Error != nil {
		return InternalErrorResponse(c, InternalSQLError, result.Error.Error())
	}
	structAssign(&article, &r)

	tags := getTags(r.TagIDs)
	article.Tags = tags
	config.Database.Model(&article).Omit("User").Updates(&article)

	var res rtype.ArticleResponse
	config.Database.Table("articles").Preload(clause.Associations).Find(&res, article.ID)
	return c.Status(fiber.StatusOK).JSON(&res)
}

// 删除文章
func ArticleDelete(c *fiber.Ctx) error {
	var article model.Article
	if err := DetailPreHandle(c, &article); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	config.Database.Select("Detail").Delete(&article)
	return c.SendStatus(fiber.StatusNoContent)
}

func getTags(tagIDs []uint) (tags []model.Tag) {
	tags = make([]model.Tag, len(tagIDs))
	for i, id := range tagIDs {
		tags[i].ID = id
	}
	FindTags(&tags)
	return
}
