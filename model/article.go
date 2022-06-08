package model

import (
	"time"

	"gorm.io/gorm"
)

// 分类
type Category struct {
	gorm.Model
	Name     string    `json:"name" gorm:"size:64"`
	FatherID *uint     `json:"father_id" gorm:"default:null"`
	Father   *Category `json:"father"`
	Count    uint      `json:"count"`
}

// 文章详情
type ArticleDetail struct {
	ID      uint   `gorm:"primarykey"`
	Content string `json:"content" gorm:"type:longtext"`
}

// 文章
type Article struct {
	BaseModel
	AuthorID       uint           `json:"author_id" gorm:"default:null"`
	Author         *User          `json:"author"`
	Title          string         `json:"title" gorm:"size:128"`
	Summary        string         `json:"summary" gorm:"size:256"`
	ReadCount      uint           `json:"read_count" gorm:"default:0"`
	LikeCount      int            `json:"like_count" gorm:"default:0"`
	IsAllowComment *bool          `json:"is_allow_comment" gorm:"default:true"`
	CommentCount   uint           `json:"comment_count" gorm:"default:0"`
	CategoryID     uint           `json:"category_id" gorm:"default:null"`
	Category       *Category      `json:"category"`
	DetailID       uint           `json:"detail_id" gorm:"default:null"`
	Detail         *ArticleDetail `json:"detail"`
	PostTime       time.Time      `json:"post_time"`
	Status         *int8          `json:"status" gorm:"default:0"` // 0: 已发布 1: 置顶 2: 草稿 3: 等待复审 4: 仅自己可见 5: 回收站
	Password       string         `json:"-" gorm:"default:null"`
	Tags           []Tag          `gorm:"many2many:article_tags;"`
}

func (article *Article) AfterCreate(tx *gorm.DB) {
	if article.CategoryID != 0 {
		tx.Model(&Category{}).Where("id = ?", article.CategoryID).Update("count", gorm.Expr("count + 1"))
	}
}

func (article *Article) BeforeUpdate(tx *gorm.DB) {
	if article.CategoryID != 0 {
		tx.Model(&Category{}).Where("id = ?", article.CategoryID).Update("count", gorm.Expr("count - 1"))
	}
}

func (article *Article) AfterUpdate(tx *gorm.DB) {
	if article.CategoryID != 0 {
		tx.Model(&Category{}).Where("id = ?", article.CategoryID).Update("count", gorm.Expr("count + 1"))
	}
}

func (article *Article) AfterDelete(tx *gorm.DB) {
	if article.CategoryID != 0 {
		tx.Model(&Category{}).Where("id = ?", article.CategoryID).Update("count", gorm.Expr("count - 1"))
	}
}
