package model

import (
	"gorm.io/gorm"
)

// 分类
type Category struct {
	gorm.Model
	Name     string    `json:"name" gorm:"size:64"`
	FatherID int64     `json:"father_id" gorm:"default:null"`
	Father   *Category `json:"father"`
}

// 文章详情
type ArticleDetail struct {
	ID      uint   `gorm:"primarykey"`
	Content string `json:"content" gorm:"type:longtext"`
}

// 文章
type Article struct {
	gorm.Model
	AuthorID       int64         `json:"author_id" gorm:"default:null"`
	Author         User          `json:"author"`
	Title          string        `json:"title" gorm:"size:128"`
	Summary        string        `json:"summary" gorm:"size:256"`
	ReadCount      uint          `json:"read_count" gorm:"default:0"`
	LikeCount      int           `json:"like_count" gorm:"default:0"`
	IsAllowComment bool          `json:"is_allow_comment" gorm:"default:true"`
	CommentCount   uint          `json:"comment_count" gorm:"default:0"`
	CategoryID     int64         `json:"category_id" gorm:"default:null"`
	Category       Category      `json:"category"`
	DetailID       int64         `json:"detail_id" gorm:"default:null"`
	Detail         ArticleDetail `json:"detail"`
}
