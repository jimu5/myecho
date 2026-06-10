package service

import (
	"myecho/config"
	"myecho/config/static_config"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler/api/errors"
	"myecho/utils"
	"os"
)

type SettingService struct {
}

type Setting[T int | string] struct {
	Key   string `json:"key"`
	Value T      `json:"value"`
}

func (s *SettingService) Create(model *mysql.SettingModel) error {
	model.IsSystem = false
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
	result, err := dal.MySqlDB.Setting.UpdateValueAndDesc(key, value, desc)
	if err != nil {
		return result, err
	}
	// 这里采用的是更新后立马更新缓存
	cacheSetting(&result)
	go saveIcon(key, value)
	return result, nil
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
