package config

import (
	"myecho/config/static_config"
	"myecho/dal"
	"myecho/dal/cache"
	"os"
)

var MySqlSettingModelCache *cache.MysqlSettingMap

func InitConfig() {
	initCreateFile()
	initDefaultMysqlSetting()
	initSettingInMysql()
}

func initSettingInMysql() {
	MySqlSettingModelCache = cache.InitSettingCache()
}

func initDefaultMysqlSetting() {
	dal.MySqlDB.Setting.InitDefaultSetting()
}

func initCreateFile() {
	_, err := os.Stat(static_config.StorageRootPath)
	if os.IsNotExist(err) {
		err = os.Mkdir(static_config.StorageRootPath, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}
}
