package model

type Tag struct {
	BaseModel
	Name string `json:"name" gorm:"size:64"`
}
