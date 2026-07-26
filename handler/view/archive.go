package view

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

type TagArchiveItem struct {
	UID   string `json:"uid"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type ArchiveGroup struct {
	Month    string               `json:"month"`
	Articles []mysql.ArticleModel `json:"articles"`
}

func TagArchive(c *fiber.Ctx) error {
	tags := make([]TagArchiveItem, 0)
	err := connect.Database.Table("tags").
		Select("tags.uid, tags.name, COUNT(articles.id) AS count").
		Joins("JOIN article_tags ON article_tags.tag_uid = tags.uid").
		Joins("JOIN articles ON articles.uid = article_tags.article_uid").
		Where("articles.status in ? AND articles.type = ? AND articles.post_time <= ?", displayableStatuses(), model.ArticleTypePost, time.Now()).
		Group("tags.uid, tags.name").
		Order("tags.name asc").
		Scan(&tags).Error
	if err != nil {
		return err
	}
	return c.Render("tags", respToMap(c, tags, PageMeta{
		Description: "文章标签归档",
		Canonical:   absoluteURL(c),
		OGTitle:     "标签",
	}))
}

func Archive(c *fiber.Ctx) error {
	articles := make([]mysql.ArticleModel, 0)
	if err := connect.Database.Model(&mysql.ArticleModel{}).
		Select("id, slug, title, post_time").
		Where("status in ? AND type = ? AND post_time <= ?", displayableStatuses(), model.ArticleTypePost, time.Now()).
		Order("post_time desc").
		Find(&articles).Error; err != nil {
		return err
	}
	groups := make([]ArchiveGroup, 0)
	groupIndex := make(map[string]int)
	for _, article := range articles {
		month := archiveMonth(article.PostTime)
		idx, ok := groupIndex[month]
		if !ok {
			groups = append(groups, ArchiveGroup{Month: month})
			idx = len(groups) - 1
			groupIndex[month] = idx
		}
		groups[idx].Articles = append(groups[idx].Articles, article)
	}
	return c.Render("archive", respToMap(c, groups, PageMeta{
		Description: "按月份归档的文章列表",
		Canonical:   absoluteURL(c),
		OGTitle:     "归档",
	}))
}

func archiveMonth(value time.Time) string {
	if value.IsZero() {
		value = time.Now()
	}
	return value.Format("2006年1月")
}

func displayableStatuses() []mysql.ArticleStatus {
	return []mysql.ArticleStatus{mysql.ARTILCE_STATUS_PUBLIC, mysql.ARTICLE_STATUS_TOP}
}
