package rtype

import (
	"myecho/handler/api/errors"
	"myecho/model"
)

type SettingCreateReq struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Description string `json:"description"`
	IsPublic    *bool  `json:"is_public"`
}

func (sq *SettingCreateReq) Validate() error {
	if len(sq.Key) == 0 {
		return errors.ErrSettingKey
	}
	return nil
}

type SettingUpdateReq struct {
	Value       string `json:"value"`
	Description string `json:"description"`
	IsPublic    *bool  `json:"is_public"`
}

type Setting struct {
	ID          uint   `json:"id"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Description string `json:"description"`
	IsSystem    bool   `json:"is_system"`
	IsPublic    bool   `json:"is_public"`
}

func NewSetting(setting model.Setting, isPublic bool) Setting {
	return Setting{
		ID:          setting.ID,
		Key:         setting.Key,
		Value:       setting.Value,
		Type:        setting.Type,
		Description: setting.Description,
		IsSystem:    setting.IsSystem,
		IsPublic:    isPublic,
	}
}
