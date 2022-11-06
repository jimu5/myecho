package service

import (
	"myecho/dal"
	"myecho/dal/mysql"
)

type LinkService struct {
}

func (l *LinkService) Create(link *mysql.LinkModel) error {
	return dal.MySqlDB.Link.Create(link)
}

func (l *LinkService) UpdateByID(id uint, link *mysql.LinkModel) error {
	return dal.MySqlDB.Link.UpdateByID(id, link)
}

func (l *LinkService) DeleteByID(id uint) error {
	return dal.MySqlDB.Link.DeleteByID(id)
}

func (l *LinkService) All(param *mysql.LinkCommonQueryParam) ([]*mysql.LinkModel, error) {
	if param == nil {
		param = &mysql.LinkCommonQueryParam{}
	}
	return dal.MySqlDB.Link.All(param)
}
