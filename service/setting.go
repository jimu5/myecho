package service

import (
	"myecho/config"
	"myecho/dal"
	"myecho/dal/mysql"
)

type SettingService struct {
}

type Setting[T int | string] struct {
	Key   string `json:"key"`
	Value T      `json:"value"`
}

func (s *SettingService) Create(model *mysql.SettingModel) error {
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
	return dal.MySqlDB.Setting.GetByKey(key)
}
func (s *SettingService) UpdateValue(key, value string) (mysql.SettingModel, error) {
	result, err := dal.MySqlDB.Setting.UpdateValue(key, value)
	if err != nil {
		return result, err
	}
	cacheSetting(&result)
	return result, nil
}

func cacheSetting(model *mysql.SettingModel) {
	if model.Cached {
		config.MySqlSettingCache.Set(model.Key, model)
	}
}
