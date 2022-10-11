package model

import (
	"gorm.io/gorm"
	"myecho/utils"
)

type Tag struct {
	BaseModel
	Name string `json:"name" gorm:"size:64"`
	UUID string `json:"uuid" gorm:"size:20"`
}

func (tag *Tag) BeforeCreate(tx *gorm.DB) error {
	if len(tag.UUID) == 0 {
		tag.UUID = utils.GenUID20()
	}
	return nil
}
