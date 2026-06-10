package rtype

import (
	"myecho/model"
	"time"
)

type CommentRequest struct {
	AuthorName  string `json:"author_name" gorm:"size:64"`
	AuthorEmail string `json:"author_email" gorm:"size:64"`
	AuthorUrl   string `json:"author_url" gorm:"size:256"`
	Content     string `json:"content" gorm:"type:text"`
	ParentID    uint   `json:"parent_id" gorm:"default:0"`
}

type CommentResponse struct {
	ID          uint      `json:"id"`
	AuthorName  string    `json:"author_name" gorm:"size:64"`
	AuthorEmail string    `json:"author_email" gorm:"size:64"`
	AuthorUrl   string    `json:"author_url" gorm:"size:256"`
	Content     string    `json:"content" gorm:"type:text"`
	ParentID    uint      `json:"parent_id" gorm:"default:0"`
	PostTime    time.Time `json:"post_time" gorm:"default:null"`
}

type CommentAdminResponse struct {
	ID           uint      `json:"id"`
	ArticleUID   string    `json:"article_uid"`
	ArticleID    uint      `json:"article_id"`
	ArticleTitle string    `json:"article_title"`
	AuthorName   string    `json:"author_name"`
	AuthorEmail  string    `json:"author_email"`
	AuthorIP     string    `json:"author_ip"`
	AuthorUrl    string    `json:"author_url"`
	AuthorAgent  string    `json:"author_agent"`
	Content      string    `json:"content"`
	Status       int8      `json:"status"`
	LikeCount    int       `json:"like_count"`
	ParentID     uint      `json:"parent_id"`
	UserID       uint      `json:"user_id"`
	PostTime     time.Time `json:"post_time"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CommentListQueryParam struct {
	Status     *model.CommentStatus `query:"status"`
	ArticleID  *uint                `query:"article_id"`
	ArticleUID *string              `query:"article_uid"`
}

type CommentUpdateReq struct {
	AuthorName  string               `json:"author_name"`
	AuthorEmail string               `json:"author_email"`
	AuthorUrl   string               `json:"author_url"`
	Content     string               `json:"content"`
	ParentID    uint                 `json:"parent_id"`
	Status      *model.CommentStatus `json:"status"`
}

type CommentBatchReq struct {
	IDs    []uint              `json:"ids"`
	Action string              `json:"action"`
	Status model.CommentStatus `json:"status"`
}
