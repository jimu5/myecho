package dal

import "myecho/dal/mysql"

var (
	MySqlDB = NewMysqlDBRepo()
)

type MysqlDBRepo struct {
	Article *mysql.ArticleDBRepo
	File    *mysql.FileRepo
}

func NewMysqlDBRepo() MysqlDBRepo {
	return MysqlDBRepo{
		Article: &mysql.ArticleDBRepo{},
		File:    &mysql.FileRepo{},
	}
}
