package mysql

import (
	"myecho/handler/api/errors"
	"myecho/model"

	"gorm.io/gorm"
)

type SettingRepo struct {
}

type SettingModel model.Setting

func (SettingModel) TableName() string {
	return "settings"
}

func (s *SettingModel) BeforeCreate(tx *gorm.DB) error {
	if err := s.checkExist(tx); err != nil {
		return err
	}
	s.setDefaultType()
	return nil
}

func (s *SettingModel) BeforeUpdate(tx *gorm.DB) error {
	s.setDefaultType()
	return nil
}

func (s *SettingModel) setDefaultType() {
	if len(s.Type) == 0 {
		s.Type = model.SettingModelTypeString
	}
}

func (s *SettingModel) checkExist(tx *gorm.DB) error {
	var count int64
	err := tx.Model(&SettingModel{}).Where("key = ?", s.Key).Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.ErrSettingKeyExist
	}
	return nil
}

func getDefaultSettings() map[string]SettingModel {
	settings := make([]SettingModel, 0)
	settings = append(settings, SettingModel{
		Key:         "SiteTitle",
		Value:       "Myecho 默认网站名",
		Description: "网站名",
	})
	settings = append(settings, SettingModel{
		Key:         "SiteIndexMetaKeyword",
		Value:       "myecho",
		Description: "站点主页关键词",
	})
	settings = append(settings, SettingModel{
		Key:         "SiteFaviconIcon",
		Value:       "",
		Description: "网站icon",
	})
	result := make(map[string]SettingModel, len(settings))
	for i := range settings {
		settings[i].IsSystem = true
		result[settings[i].Key] = settings[i]
	}
	return result
}

func (s *SettingRepo) Create(setting *SettingModel) error {
	return db.Create(setting).Error
}

func (s *SettingRepo) MCreate(settings []*SettingModel) error {
	if len(settings) != 0 {
		return db.Create(settings).Error
	}
	return nil
}

func (s *SettingRepo) MUpdateIsSystem(settings []*SettingModel, isSystem bool) error {
	if len(settings) != 0 {
		return db.Model(settings).Update("is_system", isSystem).Error
	}
	return nil
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

func (s *SettingRepo) UpdateValueAndDesc(key, value, desc string) (SettingModel, error) {
	err := db.Model(&SettingModel{}).Where("key = ?", key).Save(map[string]interface{}{
		"value":       value,
		"description": desc,
		"key":         key,
	}).Error
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

func (s *SettingRepo) DeleteByKey(key string) error {
	return db.Model(&SettingModel{}).Where("key = ?", key).Delete(&SettingModel{}).Error
}

// 初始化默认设置
func (s *SettingRepo) InitDefaultSetting() {
	allSettings, err := s.GetAll()
	if err != nil {
		panic(err)
	}
	defaultSettingMap := getDefaultSettings()
	updateSystemSettings := make([]*SettingModel, 0)
	for _, setting := range allSettings {
		if _, ok := defaultSettingMap[setting.Key]; ok {
			if !setting.IsSystem {
				updateSystemSettings = append(updateSystemSettings, setting)
			}
			delete(defaultSettingMap, setting.Key)
		}
	}
	needInitSettings := make([]*SettingModel, 0, len(defaultSettingMap))
	for i := range defaultSettingMap {
		setting := defaultSettingMap[i]
		needInitSettings = append(needInitSettings, &setting)
	}
	err = s.MCreate(needInitSettings)
	if err != nil {
		panic(err)
	}
	err = s.MUpdateIsSystem(updateSystemSettings, true)
	if err != nil {
		panic(err)
	}
}

// 检查是否为默认key
func (s *SettingRepo) CheckIsInitKey(key string) bool {
	defaultSettingMap := getDefaultSettings()
	if _, ok := defaultSettingMap[key]; ok {
		return true
	}
	return false
}
