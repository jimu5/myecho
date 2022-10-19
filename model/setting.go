package model

import (
	"gorm.io/gorm"
	"myecho/handler/api/errors"
)

type Setting struct {
	BaseModel
	Key   string `json:"key" gorm:"size:255"`
	Value string `json:"value" gorm:"type:text"`
	Type  string `json:"type" gorm:"size:20"`
}

func (s *Setting) BeforeCreate(tx *gorm.DB) error {
	var count int64
	err := tx.Model(&Setting{}).Where("key = ?", s.Key).Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.ErrSettingCreateExist
	}
	if len(s.Type) == 0 {
		s.Type = "string"
	}
	return nil
}
