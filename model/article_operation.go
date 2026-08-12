package model

import "time"

// ArticleRevision keeps the previous editable article state. Revisions are
// intentionally content-only: restoring one must not silently publish a draft.
type ArticleRevision struct {
	ID             uint                 `json:"id" gorm:"primarykey"`
	CreatedAt      time.Time            `json:"created_at"`
	ArticleID      uint                 `json:"article_id" gorm:"index"`
	Title          string               `json:"title" gorm:"size:128"`
	Slug           string               `json:"slug" gorm:"size:160"`
	Type           ArticleType          `json:"type"`
	ContentFormat  ArticleContentFormat `json:"content_format" gorm:"size:16"`
	Summary        string               `json:"summary" gorm:"size:255"`
	Content        string               `json:"content" gorm:"type:text"`
	SEOTitle       string               `json:"seo_title" gorm:"size:160"`
	SEODescription string               `json:"seo_description" gorm:"size:255"`
	ShareImage     string               `json:"share_image" gorm:"size:512"`
}

type ArticleSlugRedirect struct {
	ID         uint        `json:"id" gorm:"primarykey"`
	CreatedAt  time.Time   `json:"created_at"`
	ArticleUID string      `json:"article_uid" gorm:"size:20;index"`
	Slug       string      `json:"slug" gorm:"size:160;uniqueIndex:idx_article_slug_redirect"`
	Type       ArticleType `json:"type" gorm:"uniqueIndex:idx_article_slug_redirect"`
}

type ArticleDailyStat struct {
	ID         uint      `json:"id" gorm:"primarykey"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ArticleUID string    `json:"article_uid" gorm:"size:20;uniqueIndex:idx_article_daily_stat"`
	Date       string    `json:"date" gorm:"size:10;uniqueIndex:idx_article_daily_stat"`
	Views      uint      `json:"views" gorm:"default:0"`
}
