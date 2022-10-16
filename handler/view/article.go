package view

import (
	"github.com/gofiber/fiber/v2"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/api"
	"myecho/service"
)

func ArticleDisplayList(c *fiber.Ctx) error {
	queryParam := service.ArticleDisplayListQueryParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	pageInfo, data, err := service.S.Article.ArticleDisplayList(&queryParam)
	if err != nil {
		return err
	}
	pageInfoResp := getPageInfoRespByMysqlPageInfo(c, &pageInfo)
	return c.Render("index", respToMap(Pagination{PageInfo: pageInfoResp, PageData: data}))
}

func ArticleRetrieve(c *fiber.Ctx) error {
	queryParam := service.ArticleRetrieveQueryParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	article := new(mysql.ArticleModel)
	if err := handler.DetailPreHandleByParam(c, &article); err != nil {
		return api.NotFoundErrorResponse(c, err.Error())
	}
	queryParam.ID = article.ID
	res, err := service.S.Article.ArticleRetrieve(&queryParam)
	if err != nil {
		return err
	}
	return c.Render("article", respToMap(res))
}
