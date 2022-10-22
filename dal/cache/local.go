package cache

import (
	"myecho/dal/connect"
	"myecho/model"
	"sync"
)

var db = connect.Database

type MysqlSettingMap struct {
	sync.Map
}

func (m *MysqlSettingMap) Get(key string) (model.Setting, bool) {
	v, ok := m.Load(key)
	if !ok {
		return model.Setting{}, false
	}
	return v.(model.Setting), true
}

func (m *MysqlSettingMap) Set(key string, value *model.Setting) {
	m.Store(key, value)
}

func InitSettingCache() MysqlSettingMap {
	var allSettings []*model.Setting
	err := db.Model(&model.Setting{}).Find(&allSettings).Error
	if err != nil {
		panic(err)
	}
	var resultMap MysqlSettingMap
	for _, setting := range allSettings {
		if setting.Cached {
			resultMap.Store(setting.Key, *setting)
		}
	}
	return resultMap
}
