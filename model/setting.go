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

const (
	SettingModelTypeInt    = "int"
	SettingModelTypeString = "string"
	SettingModelTypeBool   = "bool"
)

func (s *Setting) checkExist(tx *gorm.DB) error {
	var count int64
	err := tx.Model(&Setting{}).Where("key = ?", s.Key).Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.ErrSettingKeyExist
	}
	return nil
}

func (s *Setting) setDefaultType() {
	if len(s.Type) == 0 {
		s.Type = SettingModelTypeString
	}
}

func (s *Setting) BeforeCreate(tx *gorm.DB) error {
	if err := s.checkExist(tx); err != nil {
		return err
	}
	s.setDefaultType()
	return nil
}

func (s *Setting) BeforeUpdate(tx *gorm.DB) error {
	if err := s.checkExist(tx); err != nil {
		return err
	}
	s.setDefaultType()
	return nil
}
