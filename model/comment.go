package model

import "time"

// 评论
type Comment struct {
	BaseModel
	ArticleID   uint      `json:"-" gorm:"default:null"`
	AuthorName  string    `json:"author_name" gorm:"size:64"`
	AuthorEmail string    `json:"author_email" gorm:"size:64"`
	AuthorIP    string    `json:"author_ip" gorm:"size:16"`
	AuthorUrl   string    `json:"author_url" gorm:"size:256"`
	AuthorAgent string    `json:"author_agent" gorm:"size:256"`
	Content     string    `json:"content" gorm:"type:text"`
	Status      *int8     `json:"status" gorm:"default:0"` // 0:未审批 1:审批通过 2:审批不通过 3:垃圾
	LikeCount   int       `json:"like_count" gorm:"default:0"`
	ParentID    uint      `json:"parent_id" gorm:"default:0"`
	UserID      uint      `json:"user_id" gorm:"default:0"`
	PostTime    time.Time `json:"post_time" gorm:"default:null"`
}
