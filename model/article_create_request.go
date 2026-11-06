package model

import "time"

// ArticleCreateRequest commits the retry key and response with the article.
type ArticleCreateRequest struct {
	ID          uint   `gorm:"primaryKey"`
	OwnerID     uint   `gorm:"not null;uniqueIndex:idx_article_create_key,priority:1"`
	Key         string `gorm:"size:128;not null;uniqueIndex:idx_article_create_key,priority:2"`
	RequestHash string `gorm:"size:64;not null"`
	ArticleID   uint   `gorm:"not null"`
	Response    []byte `gorm:"not null"`
	CreatedAt   time.Time
}
