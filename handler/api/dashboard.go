package api

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/model"
)

type dashboardArticle struct {
	ID           uint              `json:"id"`
	Title        string            `json:"title"`
	Slug         string            `json:"slug"`
	Type         model.ArticleType `json:"type"`
	Status       int8              `json:"status"`
	ReadCount    uint              `json:"read_count"`
	CommentCount uint              `json:"comment_count"`
	PostTime     time.Time         `json:"post_time"`
}

type dashboardResponse struct {
	ArticleCount        int64              `json:"article_count"`
	DraftCount          int64              `json:"draft_count"`
	PendingCommentCount int64              `json:"pending_comment_count"`
	PopularArticles     []dashboardArticle `json:"popular_articles"`
	RecentArticles      []dashboardArticle `json:"recent_articles"`
}

func Dashboard(c *fiber.Ctx) error {
	var result dashboardResponse
	if err := connect.Database.Model(&model.Article{}).Count(&result.ArticleCount).Error; err != nil {
		return err
	}
	if err := connect.Database.Model(&model.Article{}).
		Where("status = ?", int8(mysql.ARTICLE_STATUS_DRAFT)).
		Count(&result.DraftCount).Error; err != nil {
		return err
	}
	if err := connect.Database.Model(&model.Comment{}).
		Where("status = ?", int8(model.CommentStatusPending)).
		Count(&result.PendingCommentCount).Error; err != nil {
		return err
	}
	if err := connect.Database.Model(&model.Article{}).
		Order("read_count desc, comment_count desc, post_time desc").
		Limit(5).
		Find(&result.PopularArticles).Error; err != nil {
		return err
	}
	if err := connect.Database.Model(&model.Article{}).
		Order("created_at desc, id desc").
		Limit(5).
		Find(&result.RecentArticles).Error; err != nil {
		return err
	}
	return handler.Success(c, result)
}
