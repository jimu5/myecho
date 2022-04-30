package config

import (
	"myecho/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var Database *gorm.DB

// 连接数据库
func ConnectDB() error {
	var err error
	Database, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		panic(err)
	}
	Database.AutoMigrate(
		&model.User{},
		&model.ArticleDetail{},
		&model.Category{},
		&model.Article{},
		&model.Comment{},
	)
	return nil
}
