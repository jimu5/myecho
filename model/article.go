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
	Tags           []*Tag         `gorm:"many2many:article_tags;foreignKey:UID;joinforeignKey:ArticleUID;references:UID;joinReferences:TagUID"`
}

func (articleDetail *ArticleDetail) BeforeCreate(tx *gorm.DB) error {
	if len(articleDetail.UID) == 0 {
		articleDetail.UID = utils.GenUID20()
	}
	return nil
}
