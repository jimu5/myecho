package service

import (
	"myecho/config"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler/api/errors"
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
	cacheValue, exist := config.MySqlSettingModelCache.Get("key")
	if exist {
		return cacheValue, nil
	}
	return dal.MySqlDB.Setting.GetByKey(key)
}
func (s *SettingService) UpdateValue(key, value string) (mysql.SettingModel, error) {
	result, err := dal.MySqlDB.Setting.UpdateValue(key, value)
	if err != nil {
		return result, err
	}
	// 这里采用的是更新后立马更新缓存
	cacheSetting(&result)
	return result, nil
}

func (s *SettingService) DeleteByKey(key string) error {
	if yes := dal.MySqlDB.Setting.CheckIsInitKey(key); yes {
		return errors.ErrDeleteSettingKeyIsDefault
	}
	return dal.MySqlDB.Setting.DeleteByKey(key)
}

func cacheSetting(model *mysql.SettingModel) {
	config.MySqlSettingModelCache.Set(model.Key, model)
}
