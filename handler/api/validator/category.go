package validator

import (
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler/api/errors"
	"myecho/handler/rtype"
)

func ValidateCategoryID(categoryID uint) error {
	if categoryID == 0 {
		return nil
	}
	err := connect.Database.Where("id = ?", categoryID).First(&mysql.CategoryModel{}).Error
	if err != nil {
		return errors.ErrCategoryNotFound
	}
	return nil
}

func ValidateCategoryUID(uid string) error {
	if len(uid) == 0 {
		return nil
	}
	err := connect.Database.Where("uid = ?", uid).First(&mysql.CategoryModel{}).Error
	if err != nil {
		return errors.ErrCategoryNotFound
	}
	return nil
}

func ValidateCategoryUpdate(req *rtype.CategoryUpdateRequest) error {
	if req.Name != nil && *req.Name == "" {
		return errors.ErrCategoryNameEmpty
	}
	if req.FatherUID != nil {
		if err := ValidateCategoryUID(*req.FatherUID); err != nil {
			return err
		}
	}
	return nil
}
