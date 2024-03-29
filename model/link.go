package model

type Link struct {
	BaseModel
	Name        string `json:"name" gorm:"size:255"`
	Description string `json:"description" gorm:"size:512"`
	URL         string `json:"url" gorm:"size:1024"`
	IconURL     string `json:"icon_url" gorm:"size:2048"`
	CategoryUID string `json:"category_uid" gorm:"size: 64"`
}
