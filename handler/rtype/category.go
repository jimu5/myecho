package rtype

import (
	"fmt"
	"myecho/dal/mysql"
	"myecho/handler/api/errors"
	"myecho/service"
)

type CategoryCreateRequest struct {
	Name      string `json:"name" gorm:"size:64"`
	FatherUID string `json:"father_uid" gorm:"default:null"`
}

func (c *CategoryCreateRequest) ToMysqlModel() mysql.CategoryModel {
	return mysql.CategoryModel{
		Name:      c.Name,
		FatherUID: c.FatherUID,
	}
}
func (c *CategoryCreateRequest) Validate() error {
	if c.Name == "" {
		return errors.ErrCategoryNameEmpty
	}
	if err := service.S.Category.ValidateUIDExist(c.FatherUID); err != nil {
		return fmt.Errorf("父级分类有误: %s", err)
	}
	return nil
}

type CategoryUpdateRequest struct {
	Name     *string `json:"name" gorm:"size:64"`
	FatherID *uint   `json:"father_id" gorm:"default:null"`
}
