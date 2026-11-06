package model

import "time"

// ArticlePreview is a short-lived, revocable grant for rendering an immutable
// article snapshot. The raw bearer token is never stored in this record.
type ArticlePreview struct {
	BaseModel
	ArticleID uint       `json:"-" gorm:"not null;index:idx_article_previews_article_creator,priority:1"`
	CreatorID uint       `json:"-" gorm:"not null;index:idx_article_previews_article_creator,priority:2"`
	TokenHash string     `json:"-" gorm:"size:64;not null;uniqueIndex"`
	Snapshot  string     `json:"-" gorm:"type:text;not null"`
	ExpiresAt time.Time  `json:"-" gorm:"not null;index"`
	RevokedAt *time.Time `json:"-" gorm:"index"`
}

// ArticlePreviewSnapshot contains only public presentation data. In
// particular, it deliberately has no password hash, email, or auth fields.
type ArticlePreviewSnapshot struct {
	EditVersion    uint64               `json:"edit_version"`
	UID            string               `json:"uid"`
	Title          string               `json:"title"`
	Slug           string               `json:"slug"`
	SEOTitle       string               `json:"seo_title"`
	SEODescription string               `json:"seo_description"`
	ShareImage     string               `json:"share_image"`
	Type           ArticleType          `json:"type"`
	ContentFormat  ArticleContentFormat `json:"content_format"`
	Summary        string               `json:"summary"`
	Content        string               `json:"content"`
	CategoryUID    string               `json:"category_uid"`
	CategoryName   string               `json:"category_name"`
	Tags           []ArticlePreviewTag  `json:"tags"`
	AuthorName     string               `json:"author_name"`
	AuthorNickName string               `json:"author_nick_name"`
	AuthorSlug     string               `json:"author_slug"`
	AuthorAvatar   string               `json:"author_avatar"`
	IsAllowComment *bool                `json:"is_allow_comment"`
	PostTime       time.Time            `json:"post_time"`
	UpdatedAt      time.Time            `json:"updated_at"`
	ReadCount      uint                 `json:"read_count"`
}

type ArticlePreviewTag struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
}
