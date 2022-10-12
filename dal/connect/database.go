package connect

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"myecho/config"
	"myecho/model"
)

var Database *gorm.DB

// 连接数据库
func ConnectDB() {
	var err error
	Database, err = gorm.Open(getDialectorFromYamlConfig(), &gorm.Config{
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

func getDialectorFromYamlConfig() gorm.Dialector {
	dbConfig := config.Yaml.Database
	var dsn string
	switch config.Yaml.Database.Type {
	case "sqlite":
		dsn = dbConfig.DBName + ".db"
		return sqlite.Open(dsn)
	case "mysql":
		//	TODO: 待补充
		return nil
	case "postgresql":
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Shanghai", dbConfig.Host, dbConfig.User, dbConfig.Password, dbConfig.DBName, dbConfig.Port)
		return postgres.Open(dsn)
	default:
		// 未配置情况下使用 sqlite
		dsn = dbConfig.DBName + ".db"
		return sqlite.Open(dsn)
	}
}
