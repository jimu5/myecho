package mysql

import (
	"gorm.io/gorm"
	"myecho/dal/connect"
)

var db *gorm.DB

func InitDB() {
	db = connect.Database
}

var (
	categoryRepo = CategoryRepo{}
	articleRepo  = ArticleDBRepo{}
	linkRepo     = LinkRepo{}
)
