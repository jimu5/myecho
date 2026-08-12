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
	public := true
	private := false
	settings := make([]SettingModel, 0)
	settings = append(settings, SettingModel{
		Key:         "SiteTitle",
		Value:       "Myecho 默认网站名",
		Description: "网站名",
		IsPublic:    &public,
	})
	settings = append(settings,
		SettingModel{Key: "SiteDescription", Description: "网站描述", IsPublic: &public},
		SettingModel{Key: "SiteLogo", Description: "网站 Logo", IsPublic: &public},
		SettingModel{Key: "SiteAuthor", Description: "作者名称", IsPublic: &public},
		SettingModel{Key: "SiteAuthorBio", Description: "作者简介", IsPublic: &public},
		SettingModel{Key: "SiteFooter", Description: "页脚文本", IsPublic: &public},
		SettingModel{Key: "SiteICP", Description: "备案号", IsPublic: &public},
		SettingModel{Key: "SiteSocialLinks", Value: "[]", Description: "社交链接", IsPublic: &public},
		SettingModel{Key: "SiteShareImage", Description: "默认分享图", IsPublic: &public},
		SettingModel{Key: "BaseURL", Description: "站点地址", IsPublic: &public},
	)
	settings = append(settings, SettingModel{
		Key:         "SiteIndexMetaKeyword",
		Value:       "myecho",
		Description: "站点主页关键词",
		IsPublic:    &public,
	})
	settings = append(settings, SettingModel{
		Key:         "SiteFaviconIcon",
		Value:       "",
		Description: "网站icon",
		IsPublic:    &public,
	})
	settings = append(settings, SettingModel{
		Key:         "CommentNotificationWebhook",
		Value:       "",
		Description: "新评论通知 Webhook",
		IsPublic:    &private,
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
	err := db.Model(&SettingModel{}).Where("key = ?", key).First(&result).Error
	return result, err
}

func (s *SettingRepo) UpdateValueAndDesc(key, value, desc string) (SettingModel, error) {
	result := db.Model(&SettingModel{}).Where("key = ?", key).Updates(map[string]interface{}{
		"value":       value,
		"description": desc,
	})
	if result.Error != nil {
		return SettingModel{}, result.Error
	}
	if result.RowsAffected == 0 {
		return SettingModel{}, gorm.ErrRecordNotFound
	}
	return s.GetByKey(key)
}

func (s *SettingRepo) UpdateValueDescAndVisibility(key, value, desc string, isPublic *bool) (SettingModel, error) {
	updates := map[string]interface{}{
		"value":       value,
		"description": desc,
	}
	if isPublic != nil {
		updates["is_public"] = *isPublic
	}
	result := db.Model(&SettingModel{}).Where("key = ?", key).Updates(updates)
	if result.Error != nil {
		return SettingModel{}, result.Error
	}
	if result.RowsAffected == 0 {
		return SettingModel{}, gorm.ErrRecordNotFound
	}
	return s.GetByKey(key)
}

func (s *SettingRepo) UpdateValueAndType(key, typeValue, value string) (SettingModel, error) {
	result := db.Model(&SettingModel{}).Where("key = ?", key).Updates(map[string]interface{}{"type": typeValue, "value": value})
	if result.Error != nil {
		return SettingModel{}, result.Error
	}
	if result.RowsAffected == 0 {
		return SettingModel{}, gorm.ErrRecordNotFound
	}
	return s.GetByKey(key)
}

func (s *SettingRepo) DeleteByKey(key string) error {
	result := db.Model(&SettingModel{}).Where("key = ?", key).Delete(&SettingModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
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
