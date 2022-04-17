package model

import (
	"gorm.io/gorm"
)

// 分类
type Category struct {
	gorm.Model
	Name           string    `json:"name" gorm:"size:64"`
	FatherCategory *Category `json:"father_category"`
}

// 文章
type Article struct {
	gorm.Model
	Author         User     `json:"author"`
	Title          string   `json:"title" gorm:"size:128"`
	Summary        string   `json:"summary" gorm:"size:256"`
	ReadCount      uint     `json:"read_count" gorm:"default:0"`
	LikeCount      int      `json:"like_count" gorm:"default:0"`
	IsAllowComment bool     `json:"is_allow_comment" gorm:"default:true"`
	CommentCount   uint     `json:"comment_count" gorm:"default:0"`
	Category       Category `json:"category"`
}

// 文章详情
type ArticleDetail struct {
	Article Article `json:"article"`
	Content string  `json:"content" gorm:"type:longtext"`
}
