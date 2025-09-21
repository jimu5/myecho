package connect

import (
	"fmt"
	"myecho/config/yaml_config"
	"myecho/model"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var Database *gorm.DB

// 连接数据库
func ConnectDB() {
	var err error
	Database, err = gorm.Open(getDialectorFromYamlConfig(), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		panic(err)
	}

	// 设置连接池
	sqlDB, err := Database.DB()
	if err != nil {
		panic(err)
	}

	// 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(10)
	// 设置打开数据库连接的最大数量
	sqlDB.SetMaxOpenConns(100)
	// 设置连接可复用的最大时间
	sqlDB.SetConnMaxLifetime(time.Hour)
	err = Database.AutoMigrate(
		&model.Setting{},
		&model.Category{},
		&model.User{},
		&model.Tag{},
		&model.ArticleDetail{},
		&model.Comment{},
		&model.File{},
		&model.Article{},
		&model.Link{},
	)
	if err != nil {
		panic(err)
	}
}

func getDialectorFromYamlConfig() gorm.Dialector {
	dbConfig := yaml_config.Yaml.Database
	var dsn string
	switch yaml_config.Yaml.Database.Type {
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
