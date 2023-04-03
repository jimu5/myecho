package view

import (
	"bytes"
	"github.com/gofiber/fiber/v2"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/api"
	"myecho/service"
	"myecho/utils"
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
	// 解析成 markdown
	var buf bytes.Buffer
	if err = utils.MDParser.Convert([]byte(res.Detail.Content), &buf); err != nil {
		return err
	}
	res.Detail.Content = buf.String()
	return c.Render("article", respToMap(res))
}
