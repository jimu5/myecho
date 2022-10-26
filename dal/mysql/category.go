package mysql

import (
	"myecho/model"
)

type CategoryRepo struct {
}

type CategoryModel = model.Category

func (c *CategoryRepo) All() ([]*CategoryModel, error) {
	res := make([]*CategoryModel, 0)
	err := db.Model(&CategoryModel{}).Order("id").Find(&res).Error
	return res, err
}

func (c *CategoryRepo) Create(categoryModel *CategoryModel) error {
	return db.Create(&categoryModel).Error
}
