package view

import (
	"github.com/gofiber/fiber/v2"
	"myecho/config"
	"myecho/config/static_config"
	"myecho/dal/mysql"
	"net/url"
	"strconv"
	"strings"
)

type PageInfoResp struct {
	Next  string `json:"next"`
	Pre   string `json:"pre"`
	Total int64  `json:"total"`
}

type Pagination struct {
	PageInfo PageInfoResp `json:"page_info"`
	PageData interface{}  `json:"page_data"`
}

func GetFavicon(c *fiber.Ctx) error {
	err := c.SendFile(static_config.StorageIconPath)
	if err != nil {
		return c.SendStatus(404)
	}
	return nil
}

func getPageInfoRespByMysqlPageInfo(c *fiber.Ctx, pageInfoMysql *mysql.PageInfo) PageInfoResp {
	pageInfoResp := PageInfoResp{}
	if pageInfoMysql.Total == 0 {
		pageInfoResp.Total = pageInfoMysql.Total
		return pageInfoResp
	}
	// 计算上一页和下一页
	var (
		values url.Values
	)
	originURL := c.OriginalURL()
	rawURL := strings.Split(originURL, "?")
	if len(rawURL) <= 1 {
		// 没有参数, 默认的查询
		values, _ = url.ParseQuery("")
		if pageInfoMysql.Total > static_config.PageSize {
			values.Set("page", "2")
			pageInfoResp.Next = genRawUrl(rawURL[0], values.Encode())
		}
		return pageInfoResp
	}
	// 有参数的情况
	values, _ = url.ParseQuery(rawURL[1])
	if pageInfoMysql.Page > 1 {
		// 有上一页的情况
		values.Set("page", strconv.Itoa(pageInfoMysql.Page-1))
		pageInfoResp.Pre = genRawUrl(rawURL[0], values.Encode())
	}
	if int64(pageInfoMysql.Page*pageInfoMysql.PageSize) < pageInfoMysql.Total {
		// 都有
		values.Set("page", strconv.Itoa(pageInfoMysql.Page+1))
		pageInfoResp.Next = genRawUrl(rawURL[0], values.Encode())
	}
	return pageInfoResp
}

func genRawUrl(path, query string) string {
	return path + "?" + query
}

func respToMap(data interface{}) fiber.Map {
	return fiber.Map{
		"Data":     data,
		"Settings": config.MySqlSettingModelCache,
	}
}
