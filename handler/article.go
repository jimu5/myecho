package handler

import (
	"github.com/gofiber/fiber/v2"
	"log"
	"myecho/dal"
	"myecho/handler/rtype"
	"myecho/handler/validator"
	"myecho/model"
)

func ArticleList(c *fiber.Ctx) error {
	total, err := dal.MySqlDB.Article.CountAll()
	if err != nil {
		return err
	}
	articles, err := PageFind(c, dal.MySqlDB.Article.PageFindAll)
	// 分页查询
	if err != nil {
		return err
	}
	res := rtype.MultiModelToArticleResponse(articles)
	return PaginateData(c, total, res)
}

type ArticleRetrieveQueryParam struct {
	NoRead bool `json:"read"`
}

func ArticleRetrieve(c *fiber.Ctx) error {
	var article model.Article
	if err := DetailPreHandle(c, &article); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	afterArticle, err := dal.MySqlDB.Article.FindByID(article.ID)
	if err != nil {
		return err
	}
	res := rtype.ModelToArticleResponse(&afterArticle)
	queryParam := ArticleRetrieveQueryParam{}
	if err = c.QueryParser(&queryParam); err != nil {
		return err
	}
	if !queryParam.NoRead {
		go func() {
			if err := dal.MySqlDB.Article.AddReadCountByID(article.ID, 1); err != nil {
				log.Println(err)
			}
		}()
	}
	return c.JSON(&res)
}

func ArticleCreate(c *fiber.Ctx) error {
	var article model.Article
	var detail model.ArticleDetail
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
	user := GetUserFromCtx(c)
	article.AuthorID = user.ID
	article.Author = &user

	article.Tags = getTags(r.TagIDs)

	err = dal.MySqlDB.Article.Create(&article)
	if err != nil {
		return err
	}
	res := rtype.ModelToArticleResponse(&article)
	return c.Status(fiber.StatusCreated).JSON(res)
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

	structAssign(&article, &r)
	article.Detail = &model.ArticleDetail{Content: r.Content}
	tags := getTags(r.TagIDs)
	article.Tags = tags

	if err := dal.MySqlDB.Article.Update(&article); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	//config.Database.Debug().Model(&article).Omit("User").Updates(&article)
	article, err := dal.MySqlDB.Article.FindByID(article.ID)
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	res := rtype.ModelToArticleResponse(&article)
	return c.Status(fiber.StatusOK).JSON(&res)
}

// 删除文章
func ArticleDelete(c *fiber.Ctx) error {
	var article model.Article
	if err := DetailPreHandle(c, &article); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if err := dal.MySqlDB.Article.DeleteByID(article.ID); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func getTags(tagIDs []uint) []*model.Tag {
	if len(tagIDs) == 0 {
		return nil
	}
	tags := make([]*model.Tag, len(tagIDs))
	for i, id := range tagIDs {
		tag := &model.Tag{}
		tag.ID = id
		tags[i] = tag
	}
	FindTags(tags)
	return tags
}
