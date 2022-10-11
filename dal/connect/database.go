package connect

import (
	"myecho/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var Database *gorm.DB

// 连接数据库
func ConnectDB() {
	var err error
	Database, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		panic(err)
	}
	err = Database.AutoMigrate(
		&model.User{},
		&model.ArticleDetail{},
		&model.Category{},
		&model.Tag{},
		&model.Comment{},
		&model.File{},
		&model.Article{},
	)
	if err != nil {
		panic(err)
	}
}
