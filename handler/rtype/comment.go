package rtype

type CommentRequest struct {
	ArticleID   uint   `json:"-" gorm:"default:null"`
	AuthorName  string `json:"author_name" gorm:"size:64"`
	AuthorEmail string `json:"author_email" gorm:"size:64"`
	AuthorUrl   string `json:"author_url" gorm:"size:256"`
	Content     string `json:"content" gorm:"type:text"`
	ParentID    uint   `json:"parent_id" gorm:"default:0"`
}
