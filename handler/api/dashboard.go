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
	ScheduledCount      int64              `json:"scheduled_count"`
	ViewCount7Days      int64              `json:"view_count_7_days"`
	ViewCount30Days     int64              `json:"view_count_30_days"`
	CommentCount7Days   int64              `json:"comment_count_7_days"`
	CommentCount30Days  int64              `json:"comment_count_30_days"`
	ScheduledArticles   []dashboardArticle `json:"scheduled_articles"`
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
	now := time.Now()
	scheduledUntil := now.AddDate(0, 0, 7)
	scheduled := connect.Database.Model(&model.Article{}).
		Where("status IN ? AND post_time > ? AND post_time <= ?",
			[]int8{int8(mysql.ARTILCE_STATUS_PUBLIC), int8(mysql.ARTICLE_STATUS_TOP)},
			now,
			scheduledUntil)
	if err := scheduled.Count(&result.ScheduledCount).Error; err != nil {
		return err
	}
	if err := scheduled.Order("post_time asc, id asc").
		Find(&result.ScheduledArticles).Error; err != nil {
		return err
	}
	start7Days := startOfDashboardDay(now.AddDate(0, 0, -6))
	start30Days := startOfDashboardDay(now.AddDate(0, 0, -29))
	if err := dashboardViewCountSince(start7Days, &result.ViewCount7Days); err != nil {
		return err
	}
	if err := dashboardViewCountSince(start30Days, &result.ViewCount30Days); err != nil {
		return err
	}
	if err := connect.Database.Model(&model.Comment{}).
		Where("created_at >= ?", start7Days).
		Count(&result.CommentCount7Days).Error; err != nil {
		return err
	}
	if err := connect.Database.Model(&model.Comment{}).
		Where("created_at >= ?", start30Days).
		Count(&result.CommentCount30Days).Error; err != nil {
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

func startOfDashboardDay(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func dashboardViewCountSince(since time.Time, result *int64) error {
	return connect.Database.Model(&model.ArticleDailyStat{}).
		Select("COALESCE(SUM(views), 0)").
		Where("date >= ?", since.Format("2006-01-02")).
		Scan(result).Error
}
