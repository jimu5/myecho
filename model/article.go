package model

import (
	"myecho/utils"
	"time"

	"gorm.io/gorm"
)

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
	Category       *Category      `json:"category" gorm:"foreignKey:category_uid;references:uid"`
	DetailUID      string         `json:"detail_uid" gorm:"size:64"`
	Detail         *ArticleDetail `json:"detail" gorm:"foreignKey:detail_uid;references:uid"`
	PostTime       time.Time      `json:"post_time"`
	Status         int8           `json:"status" gorm:"default:1"` //  1:公开 2: 置顶 3: 私密 4: 草稿 5: 等待复审 6: 回收站
	Password       string         `json:"-" gorm:"default:null"`
	Tags           []*Tag         `gorm:"many2many:article_tags;foreignKey:UID;references:UID;"`
}

func (articleDetail *ArticleDetail) BeforeCreate(tx *gorm.DB) error {
	if len(articleDetail.UID) == 0 {
		articleDetail.UID = utils.GenUID20()
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
	if err := article.AddCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (article *Article) BeforeUpdate(tx *gorm.DB) error {
	if err := article.ReduceCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (article *Article) AfterUpdate(tx *gorm.DB) error {
	if err := article.AddCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (article *Article) AfterDelete(tx *gorm.DB) error {
	if err := article.ReduceCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (article *Article) AddCategoryCount(tx *gorm.DB) error {
	if article.Status == 1 && len(article.CategoryUID) != 0 {
		return tx.Model(&Category{}).Where("uid = ?", article.CategoryUID).Update("count", gorm.Expr("count + 1")).Error
	}
	return nil
}

func (article *Article) ReduceCategoryCount(tx *gorm.DB) error {
	oldArticle, err := getArticle(tx, article.ID)
	if err != nil {
		return err
	}
	if oldArticle.Status == 1 && len(oldArticle.CategoryUID) != 0 {
		return tx.Model(&Category{}).Where("uid = ?", oldArticle.CategoryUID).Update("count", gorm.Expr("count - 1")).Error
	}
	return nil
}

func getArticle(tx *gorm.DB, id uint) (Article, error) {
	var oldArticle Article
	err := tx.Model(&Article{}).Where("id = ?", id).First(&oldArticle).Error
	return oldArticle, err
}
