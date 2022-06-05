package rtype

import (
	"myecho/model"
	"time"
)

type ArticleRequest struct {
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	Content        string    `json:"content"`
	CategoryID     uint      `json:"category_id"`
	IsAllowComment *bool     `json:"is_allow_comment"`
	PostTime       time.Time `json:"post_time"`
	Status         *int8     `json:"status"`
	Password       string    `json:"password"`
	TagIDs         []uint    `json:"tag_ids"`
}

type User struct {
	ID       uint   `json:"id"`
	NickName string `json:"nick_name"`
}

type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ArticleResponse struct {
	model.BaseModel
	AuthorID       uint                 `json:"-"`
	Author         *User                `json:"author"`
	Title          string               `json:"title"`
	Summary        string               `json:"summary"`
	DetailID       uint                 `json:"-"`
	Detail         *model.ArticleDetail `json:"detail"`
	CategoryID     uint                 `json:"-"`
	Category       *Category            `json:"category"`
	IsAllowComment *bool                `json:"is_allow_comment"`
	ReadCount      uint                 `json:"read_count"`
	LikeCount      int                  `json:"like_count"`
	CommentCount   uint                 `json:"comment_count"`
	PostTime       time.Time            `json:"post_time"`
	Status         *int8                `json:"status"`
	Tags           []model.Tag          `json:"tags" gorm:"many2many:article_tags;joinForeignKey:ArticleID"`
}
