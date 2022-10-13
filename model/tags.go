package model

import (
	"gorm.io/gorm"
	"myecho/utils"
)

type Tag struct {
	BaseModel
	Name string `json:"name" gorm:"size:64"`
	UID  string `json:"uid" gorm:"size:20"`
}

func (tag *Tag) BeforeCreate(tx *gorm.DB) error {
	if len(tag.UID) == 0 {
		tag.UID = utils.GenUID20()
	}
	return nil
}
