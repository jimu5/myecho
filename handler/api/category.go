package api

import (
	"github.com/gofiber/fiber/v2"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"myecho/model"
)

func CategoryAll(c *fiber.Ctx) error {
	var res []mysql.CategoryModel
	err := connect.Database.Table("categories").Order("id").Find(&res).Error
	if err != nil {
		return err
	}
	return c.JSON(&res)
}

func ArticleCategoryCreate(c *fiber.Ctx) error {
	var req rtype.CategoryCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return nil
	}
	if err := validator.ValidateCategoryCreate(&req); err != nil {
		return nil
	}
	category := mysql.CategoryModel{
		Name:      req.Name,
		FatherUID: req.FatherUID,
	}
	category.Type = model.CategoryTypeArticle
	err := connect.Database.Table("categories").Create(&category).Error
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(&category)
}

func CategoryUpdate(c *fiber.Ctx) error {
	var req rtype.CategoryUpdateRequest
	var category mysql.CategoryModel
	if err := c.BodyParser(&req); err != nil {
		return nil
	}
	if err := validator.ValidateCategoryUpdate(&req); err != nil {
		return nil
	}
	if err := handler.DetailPreHandleByParam(c, &category); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if result := connect.Database.Table("categories").Model(&category).Updates(&req); result.Error != nil {
		return InternalErrorResponse(c, InternalSQLError, result.Error.Error())
	}
	return c.JSON(&category)
}

func CategoryDelete(c *fiber.Ctx) error {
	var category mysql.CategoryModel
	if err := handler.DetailPreHandleByParam(c, &category); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if result := connect.Database.Table("categories").Delete(&category); result.Error != nil {
		return InternalErrorResponse(c, InternalSQLError, result.Error.Error())
	}
	if err := deleteAlterRelated(category.ID); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func deleteAlterRelated(deletedCategoryID uint) error {
	if tx := connect.Database.Table("articles").Where("category_id = ?", deletedCategoryID).Update(
		"category_uid", nil); tx.Error != nil {
		return tx.Error
	}
	if tx := connect.Database.Table("categories").Where("father_id = ?", deletedCategoryID).Delete(
		&mysql.CategoryModel{}); tx.Error != nil {
		return tx.Error
	}
	return nil
}
