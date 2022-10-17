package mysql

import "myecho/model"

type CategoryRepo struct {
}

type CategoryModel = model.Category

func (c *CategoryRepo) Create(categoryModel *CategoryModel) error {
	return db.Create(&categoryModel).Error
}
