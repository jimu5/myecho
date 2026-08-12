package service

import (
	"os"
	"strings"

	"myecho/config"
	"myecho/config/static_config"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler/api/errors"
	"myecho/utils"
)

type SettingService struct {
}

type Setting[T int | string] struct {
	Key   string `json:"key"`
	Value T      `json:"value"`
}

func (s *SettingService) Create(model *mysql.SettingModel) error {
	model.IsSystem = false
	if model.IsPublic == nil {
		isPublic := false
		model.IsPublic = &isPublic
	}
	if *model.IsPublic && IsSensitiveSettingKey(model.Key) {
		return errors.ErrSettingKey
	}
	if err := validateSettingValue(model.Key, model.Value); err != nil {
		return err
	}
	err := dal.MySqlDB.Setting.Create(model)
	if err != nil {
		return err
	}
	cacheSetting(model)
	return nil
}

func (s *SettingService) GetAll() ([]*mysql.SettingModel, error) {
	return dal.MySqlDB.Setting.GetAll()
}

func (s *SettingService) GetByKey(key string) (mysql.SettingModel, error) {
	cacheValue, exist := config.MySqlSettingModelCache.Get(key)
	if exist {
		return cacheValue, nil
	}
	return dal.MySqlDB.Setting.GetByKey(key)
}
func (s *SettingService) UpdateValueAndDesc(key, value, desc string) (mysql.SettingModel, error) {
	return s.UpdateValueDescAndVisibility(key, value, desc, nil)
}

func (s *SettingService) UpdateValueDescAndVisibility(key, value, desc string, isPublic *bool) (mysql.SettingModel, error) {
	if isPublic != nil && *isPublic && IsSensitiveSettingKey(key) {
		return mysql.SettingModel{}, errors.ErrSettingKey
	}
	if err := validateSettingValue(key, value); err != nil {
		return mysql.SettingModel{}, err
	}
	result, err := dal.MySqlDB.Setting.UpdateValueDescAndVisibility(key, value, desc, isPublic)
	if err != nil {
		return result, err
	}
	// 这里采用的是更新后立马更新缓存
	cacheSetting(&result)
	go saveIcon(key, value)
	return result, nil
}

func validateSettingValue(key, value string) error {
	if key == "CommentNotificationWebhook" && strings.TrimSpace(value) != "" {
		return utils.ValidateRemoteFileURL(value)
	}
	return nil
}

func IsSettingPublic(setting *mysql.SettingModel) bool {
	if setting == nil || IsSensitiveSettingKey(setting.Key) {
		return false
	}
	if setting.IsPublic != nil {
		return *setting.IsPublic
	}
	return dal.MySqlDB.Setting.CheckIsInitKey(setting.Key) && setting.Key != "CommentNotificationWebhook"
}

func (s *SettingService) DeleteByKey(key string) error {
	if yes := dal.MySqlDB.Setting.CheckIsInitKey(key); yes {
		return errors.ErrDeleteSettingKeyIsDefault
	}
	if err := dal.MySqlDB.Setting.DeleteByKey(key); err != nil {
		return err
	}
	if config.MySqlSettingModelCache != nil {
		config.MySqlSettingModelCache.Delete(key)
	}
	return nil
}

func cacheSetting(model *mysql.SettingModel) {
	if config.MySqlSettingModelCache != nil {
		config.MySqlSettingModelCache.Set(model.Key, model)
	}
}

func saveIcon(key, value string) error {
	if key != "SiteFaviconIcon" {
		return nil
	}
	tmpPath := static_config.StorageIconPath + ".tmp"
	_ = os.Remove(tmpPath)
	if err := utils.DownloadRemoteFile(value, tmpPath, utils.DefaultMaxRemoteFileBytes); err != nil {
		return err
	}
	_ = os.Remove(static_config.StorageIconPath)
	return os.Rename(tmpPath, static_config.StorageIconPath)
}
