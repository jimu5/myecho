package model

import (
	"myecho/utils"
	"time"

	"gorm.io/gorm"
)

// 分类
type Category struct {
	BaseModel
	Name      string  `json:"name" gorm:"size:64"`
	UID       string  `json:"uid" gorm:"size:20;"`
	FatherUID *string `json:"father_uid" gorm:"default:null"`
	Count     uint    `json:"count"`
}

// 文章详情
type ArticleDetail struct {
	ID      uint   `gorm:"primarykey"`
	UID     string `json:"uid" gorm:"size:64;index"`
	Content string `json:"content" gorm:"type:text"`
}

// 文章
type Article struct {
	BaseModel
	UID            string         `json:"uid" gorm:"size:20;index"`
	AuthorID       uint           `json:"author_id" gorm:"default:null"`
	Author         *User          `json:"author"`
	Title          string         `json:"title" gorm:"size:128"`
	Summary        string         `json:"summary" gorm:"size:255"`
	ReadCount      uint           `json:"read_count" gorm:"default:0"`
	LikeCount      int            `json:"like_count" gorm:"default:0"`
	IsAllowComment *bool          `json:"is_allow_comment" gorm:"default:true"`
	CommentCount   uint           `json:"comment_count" gorm:"default:0"`
	CategoryUID    string         `json:"category_uid" gorm:"size:20"`
	Category       *Category      `json:"category" gorm:"foreignKey:UID"`
	DetailUID      string         `json:"detail_uid" gorm:"size:64"`
	Detail         *ArticleDetail `json:"detail" gorm:"foreignKey:UID"`
	PostTime       time.Time      `json:"post_time"`
	Status         int8           `json:"status" gorm:"default:1"` //  1:公开 2: 置顶 3: 私密 4: 草稿 5: 等待复审 6: 回收站
	Password       string         `json:"-" gorm:"default:null"`
	Tags           []*Tag         `gorm:"many2many:article_tags;foreignKey:UID;References:UID;"`
}

func (articleDetail *ArticleDetail) BeforeCreate(tx *gorm.DB) error {
	if len(articleDetail.UID) == 0 {
		articleDetail.UID = utils.GenUID20()
	}
	return nil
}

func (category *Category) BeforeCreate(tx *gorm.DB) error {
	if len(category.UID) == 0 {
		category.UID = utils.GenUID20()
	}
	return nil
}

func (article *Article) BeforeCreate(tx *gorm.DB) error {
	if len(article.UID) == 0 {
		article.UID = utils.GenUID20()
	}
	if article.Detail != nil {
		if len(article.Detail.UID) == 0 {
			uid := utils.GenUID20()
			article.Detail.UID = article.UID + "_" + uid
		}
	}
	// TODO: 根据文章内容生成统计信息 https://github.com/mdigger/goldmark-stats info.Chars, info.Duration(400), 使用协程加版本锁
	return nil
}

func (article *Article) AfterCreate(tx *gorm.DB) error {
	if len(article.CategoryUID) != 0 {
		tx.Model(&Category{}).Where("uid = ?", article.CategoryUID).Update("count", gorm.Expr("count + 1"))
	}
	return nil
}

func (article *Article) BeforeUpdate(tx *gorm.DB) error {
	if len(article.CategoryUID) != 0 {
		tx.Model(&Category{}).Where("uid = ?", article.CategoryUID).Update("count", gorm.Expr("count - 1"))
	}
	return nil
}

func (article *Article) AfterUpdate(tx *gorm.DB) error {
	if len(article.CategoryUID) != 0 {
		tx.Model(&Category{}).Where("uid = ?", article.CategoryUID).Update("count", gorm.Expr("count + 1"))
	}
	return nil
}

func (article *Article) AfterDelete(tx *gorm.DB) error {
	if len(article.CategoryUID) != 0 {
		tx.Model(&Category{}).Where("uid = ?", article.CategoryUID).Update("count", gorm.Expr("count - 1"))
	}
	return nil
}
