package mysql

import "myecho/model"

type SettingRepo struct {
}

type SettingModel = model.Setting

func getDefaultSettings() map[string]SettingModel {
	settings := make([]SettingModel, 0)
	settings = append(settings, SettingModel{
		Key:    "SiteTitle",
		Value:  "Myecho 默认网站名",
		Cached: true,
	})
	settings = append(settings, SettingModel{
		Key:    "SiteIndexMetaKeyword",
		Value:  "myecho",
		Cached: true,
	})
	result := make(map[string]SettingModel, len(settings))
	for i := range settings {
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

// 初始化默认设置
func (s *SettingRepo) InitDefaultSetting() {
	allSettings, err := s.GetAll()
	if err != nil {
		panic(err)
	}
	defaultSettingMap := getDefaultSettings()
	for _, setting := range allSettings {
		if _, ok := defaultSettingMap[setting.Key]; ok {
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
}
