package mysql

import "myecho/model"

type SettingRepo struct {
}

type SettingModel = model.Setting

const (
	SettingModelTypeInt    = "int"
	SettingModelTypeString = "string"
	SettingModelTypeBool   = "bool"
)

func (s *SettingRepo) Create(setting *SettingModel) error {
	return db.Create(setting).Error
}

func (s *SettingRepo) GetAll() ([]*SettingModel, error) {
	var result []*SettingModel
	err := db.Model(&SettingModel{}).Find(&result).Error
	if err != nil {
		return nil, err
	}
	return result, err
}

func (s *SettingRepo) GetByKey(key string) (SettingModel, error) {
	var result SettingModel
	err := db.Model(&SettingModel{}).Where("key = ?", key).Find(&result).Error
	return result, err
}

func (s *SettingRepo) UpdateValue(key, value string) (SettingModel, error) {
	err := db.Model(&SettingModel{}).Where("key = ?", key).Update("value", value).Error
	if err != nil {
		return SettingModel{}, err
	}
	return s.GetByKey(key)
}

func (s *SettingRepo) UpdateValueAndType(key, typeValue, value string) (SettingModel, error) {
	err := db.Model(&SettingModel{}).Where("key = ?", key).Updates(map[string]interface{}{"type": typeValue, "value": value}).Error
	if err != nil {
		return SettingModel{}, nil
	}
	return s.GetByKey(key)
}
