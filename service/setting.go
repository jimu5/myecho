package service

import (
	"myecho/dal"
	"myecho/dal/mysql"
)

type Setting struct {
}

func (s *Setting) Create(model *mysql.SettingModel) error {
	return dal.MySqlDB.Setting.Create(model)
}

func (s *Setting) GetAll() ([]*mysql.SettingModel, error) {
	return dal.MySqlDB.Setting.GetAll()
}

func (s *Setting) GetByKey(key string) (mysql.SettingModel, error) {
	return dal.MySqlDB.Setting.GetByKey(key)
}
func (s *Setting) UpdateValue(key, value string) (mysql.SettingModel, error) {
	return dal.MySqlDB.Setting.UpdateValue(key, value)
}
