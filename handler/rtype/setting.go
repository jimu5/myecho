package rtype

import (
	"myecho/handler/api/errors"
	"myecho/model"
)

type SettingCreateReq struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
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
}

type Setting struct {
	model.Setting
}
