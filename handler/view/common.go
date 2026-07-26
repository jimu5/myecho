package view

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"myecho/config"
	"myecho/config/static_config"
	"myecho/dal/mysql"
	"myecho/model"
	"myecho/service"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PageInfoResp struct {
	Next       string `json:"next"`
	Pre        string `json:"pre"`
	Total      int64  `json:"total"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	TotalPages int    `json:"total_pages"`
}

type Pagination struct {
	PageInfo    PageInfoResp `json:"page_info"`
	PageData    interface{}  `json:"page_data"`
	FilterLabel string       `json:"filter_label"`
}

type PageMeta struct {
	Description string
	Canonical   string
	OGTitle     string
	OGType      string
	OGURL       string
	Image       string
	JSONLD      string
}

func GetFavicon(c *fiber.Ctx) error {
	err := c.SendFile(static_config.StorageIconPath)
	if err != nil {
		return c.SendStatus(404)
	}
	return nil
}

func getPageInfoRespByMysqlPageInfo(c *fiber.Ctx, pageInfoMysql *mysql.PageInfo) PageInfoResp {
	page := pageInfoMysql.Page
	if page < 1 {
		page = 1
	}
	pageSize := pageInfoMysql.PageSize
	if pageSize < 1 {
		pageSize = static_config.PageSize
	}
	pageInfoResp := PageInfoResp{
		Total:    pageInfoMysql.Total,
		Page:     page,
		PageSize: pageSize,
	}
	if pageInfoMysql.Total > 0 {
		pageInfoResp.TotalPages = int((pageInfoMysql.Total + int64(pageSize) - 1) / int64(pageSize))
	}
	if pageInfoMysql.Total == 0 {
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
		if pageInfoMysql.Total > int64(pageSize) {
			values.Set("page", "2")
			pageInfoResp.Next = genRawUrl(rawURL[0], values.Encode())
		}
		return pageInfoResp
	}
	// 有参数的情况
	values, _ = url.ParseQuery(rawURL[1])
	if page > 1 {
		// 有上一页的情况
		values.Set("page", strconv.Itoa(page-1))
		pageInfoResp.Pre = genRawUrl(rawURL[0], values.Encode())
	}
	if int64(page*pageSize) < pageInfoMysql.Total {
		// 都有
		values.Set("page", strconv.Itoa(page+1))
		pageInfoResp.Next = genRawUrl(rawURL[0], values.Encode())
	}
	return pageInfoResp
}

func genRawUrl(path, query string) string {
	return path + "?" + query
}

func respToMap(c *fiber.Ctx, data interface{}, meta ...PageMeta) fiber.Map {
	pageMeta := PageMeta{}
	if len(meta) > 0 {
		pageMeta = meta[0]
	}
	if pageMeta.OGType == "" {
		pageMeta.OGType = "website"
	}
	if pageMeta.OGURL == "" {
		pageMeta.OGURL = pageMeta.Canonical
	}
	if pageMeta.Description == "" {
		pageMeta.Description = getSettingString("SiteDescription")
	}
	if pageMeta.Image == "" {
		pageMeta.Image = settingAbsoluteURL(c, "SiteShareImage")
	}
	// 创建响应map
	resp := fiber.Map{
		"Data":                  data,
		"Settings":              config.MySqlSettingModelCache,
		"Meta":                  pageMeta,
		"CurrentYear":           time.Now().Year(),
		"NavigationStaticPages": []*service.StaticPage{},
		"SiteLogo":              settingAbsoluteURL(c, "SiteLogo"),
		"SiteSocialLinks":       siteSocialLinks(),
	}
	if pages, err := service.S.StaticPage.ListNavigationPages(); err == nil {
		resp["NavigationStaticPages"] = pages
	}

	theme, isPreview := resolveThemeForRequest(c)
	if theme != nil {
		resp["Theme"] = theme
		resp["ThemeAssetBase"] = service.ThemeAssetBaseURL(theme.Name)
		resp["IsThemePreview"] = isPreview
		config, err := (*model.Theme)(theme).GetConfig()
		if err != nil {
			config = make(map[string]interface{})
		}
		resp["ThemeConfig"] = config
		resp["SupportsColorMode"] = theme.IsDefault || config["supports_color_mode"] == true
	}

	return resp
}

func resolveThemeForRequest(c *fiber.Ctx) (*mysql.ThemeModel, bool) {
	if c != nil {
		token := c.Cookies(service.ThemePreviewCookieName)
		if token != "" {
			if theme, err := service.S.Theme.ValidatePreviewToken(token); err == nil && theme != nil {
				return theme, true
			}
			expireThemePreviewCookie(c)
		}
	}
	theme, err := service.S.Theme.GetActiveTheme()
	if err == nil {
		return theme, false
	}
	return nil, false
}

func absoluteURL(c *fiber.Ctx) string {
	return siteBaseURL(c) + c.Path()
}

func settingAbsoluteURL(c *fiber.Ctx, key string) string {
	value := strings.TrimSpace(getSettingString(key))
	if value == "" {
		return value
	}
	ref, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if ref.IsAbs() {
		if ref.Scheme != "http" && ref.Scheme != "https" {
			return ""
		}
		return ref.String()
	}
	base, err := url.Parse(siteBaseURL(c) + "/")
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}

func siteBaseURL(c *fiber.Ctx) string {
	if value := strings.TrimRight(strings.TrimSpace(getSettingString("BaseURL")), "/"); value != "" {
		if parsed, err := url.Parse(value); err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			return value
		}
	}
	if c == nil {
		return ""
	}
	return c.Protocol() + "://" + c.Hostname()
}

func siteSocialLinks() []string {
	value := strings.TrimSpace(getSettingString("SiteSocialLinks"))
	if value == "" {
		return nil
	}
	var candidates []string
	if err := json.Unmarshal([]byte(value), &candidates); err != nil {
		candidates = strings.Fields(value)
	}
	links := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		parsed, err := url.Parse(strings.TrimSpace(candidate))
		if err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			links = append(links, parsed.String())
		}
	}
	return links
}
