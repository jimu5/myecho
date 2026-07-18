package view

import (
	"encoding/xml"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"myecho/config"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

type rssDoc struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func RSS(c *fiber.Ctx) error {
	articles, err := feedArticles()
	if err != nil {
		return err
	}
	siteTitle := getSettingString("SiteTitle")
	siteDescription := getSettingString("SiteIndexMetaKeyword")
	baseURL := requestBaseURL(c)
	items := make([]rssItem, 0, len(articles))
	for _, article := range articles {
		link := baseURL + articlePublicPath(article)
		items = append(items, rssItem{
			Title:       article.Title,
			Link:        link,
			GUID:        link,
			Description: article.Summary,
			PubDate:     article.PostTime.Format(time.RFC1123Z),
		})
	}
	doc := rssDoc{
		Version: "2.0",
		Channel: rssChannel{
			Title:       siteTitle,
			Link:        baseURL + "/",
			Description: siteDescription,
			Items:       items,
		},
	}
	return writeXML(c, doc)
}

func Sitemap(c *fiber.Ctx) error {
	articles, err := sitemapArticles()
	if err != nil {
		return err
	}
	baseURL := requestBaseURL(c)
	urls := []sitemapURL{
		{Loc: baseURL + "/"},
		{Loc: baseURL + "/article/categories"},
		{Loc: baseURL + "/tags"},
		{Loc: baseURL + "/archive"},
		{Loc: baseURL + "/links"},
	}
	for _, article := range articles {
		urls = append(urls, sitemapURL{
			Loc:     baseURL + articlePublicPath(article),
			LastMod: article.UpdatedAt.Format("2006-01-02"),
		})
	}
	return writeXML(c, sitemapURLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	})
}

func feedArticles() ([]mysql.ArticleModel, error) {
	articles := make([]mysql.ArticleModel, 0)
	err := connect.Database.Model(&mysql.ArticleModel{}).
		Where("status in ? AND type = ?", displayableStatuses(), model.ArticleTypePost).
		Order("post_time desc").
		Find(&articles).Error
	return articles, err
}

func sitemapArticles() ([]mysql.ArticleModel, error) {
	articles := make([]mysql.ArticleModel, 0)
	err := connect.Database.Model(&mysql.ArticleModel{}).
		Where("status in ? AND type in ?", displayableStatuses(), []model.ArticleType{model.ArticleTypePost, model.ArticleTypePage}).
		Order("post_time desc").
		Find(&articles).Error
	return articles, err
}

func articlePublicPath(article mysql.ArticleModel) string {
	if article.Slug != "" {
		if article.Type == model.ArticleTypePage {
			return "/pages/" + article.Slug
		}
		return "/posts/" + article.Slug
	}
	return "/articles/" + uintToString(article.ID)
}

func writeXML(c *fiber.Ctx, payload interface{}) error {
	body, err := xml.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	c.Type("xml", "utf-8")
	return c.Send(append([]byte(xml.Header), body...))
}

func requestBaseURL(c *fiber.Ctx) string {
	return c.Protocol() + "://" + c.Hostname()
}

func getSettingString(key string) string {
	if config.MySqlSettingModelCache == nil {
		return ""
	}
	return config.MySqlSettingModelCache.GetStringValue(key)
}

func uintToString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
