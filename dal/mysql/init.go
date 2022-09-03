package mysql

import (
	"gorm.io/gorm"
	"myecho/config"
)

var db *gorm.DB

func InitDB() {
	db = config.Database
}
