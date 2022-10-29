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

func (c *CategoryRepo) GetAllChildrenUID(father_uid string) ([]string, error) {
	children := make([]*CategoryModel, 0)
	err := db.Model(&CategoryModel{}).Where("father_uid = ?", father_uid).Find(&children).Error
	if err != nil {
		return nil, err
	}
	childrenUID := make([]string, 0)
	for len(children) != 0 {
		fathersUID := make([]string, 0, len(children))
		for _, category := range children {
			fathersUID = append(fathersUID, category.UID)
			childrenUID = append(childrenUID, category.UID)
		}
		err = db.Model(&CategoryModel{}).Where("father_uid in (?)", fathersUID).Find(&children).Error
		if err != nil {
			return nil, err
		}
	}
	return childrenUID, nil
}
