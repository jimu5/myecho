package config

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var Database *gorm.DB

// 连接数据库
func Connect() error {
	var err error
	Database, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	Database.AutoMigrate()
	return nil
}
