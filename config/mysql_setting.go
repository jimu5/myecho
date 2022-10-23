package config

import (
	"myecho/dal"
	"myecho/dal/cache"
)

var MySqlSettingModelCache *cache.MysqlSettingMap

func InitConfig() {
	initDefaultMysqlSetting()
	initSettingInMysql()
}

func initSettingInMysql() {
	MySqlSettingModelCache = cache.InitSettingCache()
}

func initDefaultMysqlSetting() {
	dal.MySqlDB.Setting.InitDefaultSetting()
}
