package cache

import (
	"myecho/dal"
	"myecho/dal/mysql"
	"sync"
)

type MysqlSettingMap struct {
	sync.Map
}

func (m *MysqlSettingMap) Get(key string) (mysql.SettingModel, bool) {
	v, ok := m.Load(key)
	if !ok {
		return mysql.SettingModel{}, false
	}
	return v.(mysql.SettingModel), true
}

func (m *MysqlSettingMap) Set(key string, value *mysql.SettingModel) {
	m.Store(key, *value)
}

func (m *MysqlSettingMap) GetStringValue(key string) string {
	v, ok := m.Get(key)
	if !ok {
		return ""
	}
	return v.Value
}

func InitSettingCache() *MysqlSettingMap {
	allSettings, _ := dal.MySqlDB.Setting.GetAll()
	var resultMap MysqlSettingMap
	for _, setting := range allSettings {
		resultMap.Store(setting.Key, *setting)
	}
	return &resultMap
}
