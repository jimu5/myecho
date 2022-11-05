package dal

import "myecho/dal/mysql"

var (
	MySqlDB = NewMysqlDBRepo()
)

type MysqlDBRepo struct {
	Article  *mysql.ArticleDBRepo
	File     *mysql.FileRepo
	Category *mysql.CategoryRepo
	Setting  *mysql.SettingRepo
	Link     *mysql.LinkRepo
}

func NewMysqlDBRepo() MysqlDBRepo {
	return MysqlDBRepo{
		Article:  &mysql.ArticleDBRepo{},
		File:     &mysql.FileRepo{},
		Category: &mysql.CategoryRepo{},
		Setting:  &mysql.SettingRepo{},
		Link:     &mysql.LinkRepo{},
	}
}
