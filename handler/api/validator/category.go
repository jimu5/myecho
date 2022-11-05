package validator

import (
	"myecho/dal/connect"
	"myecho/handler/api/errors"
	"myecho/handler/rtype"
	"myecho/model"
)

func ValidateCategoryID(categoryID uint) error {
	if categoryID == 0 {
		return nil
	}
	err := connect.Database.Where("id = ?", categoryID).First(&model.Category{}).Error
	if err != nil {
		return errors.ErrCategoryNotFound
	}
	return nil
}

func ValidateCategoryUID(uid string) error {
	if len(uid) == 0 {
		return nil
	}
	err := connect.Database.Where("uid = ?", uid).First(&model.Category{}).Error
	if err != nil {
		return errors.ErrCategoryNotFound
	}
	return nil
}

func ValidateCategoryCreate(req *rtype.CategoryCreateRequest) error {
	if req.Name == "" {
		return errors.ErrCategoryNameEmpty
	}
	if len(req.FatherUID) != 0 {
		if err := ValidateCategoryUID(req.FatherUID); err != nil {
			return err
		}
	}
	if !req.Type.IsCategoryTypeValid() {
		return errors.ErrCategoryType
	}
	return nil
}

func ValidateCategoryUpdate(req *rtype.CategoryUpdateRequest) error {
	if req.Name != nil && *req.Name == "" {
		return errors.ErrCategoryNameEmpty
	}
	if req.FatherID != nil {
		if err := ValidateCategoryID(*req.FatherID); err != nil {
			return err
		}
	}
	return nil
}
