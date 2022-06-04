package handler

import (
	"github.com/gofiber/fiber/v2"
	"myecho/config"
	"myecho/handler/rtype"
	"myecho/handler/validator"
	"myecho/model"
)

func CategoryAll(c *fiber.Ctx) error {
	var res []model.Category
	config.Database.Table("categories").Find(&res)
	return c.JSON(&res)
}

func CategoryCreate(c *fiber.Ctx) error {
	var req rtype.CategoryCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return nil
	}
	if err := validator.ValidateCategoryCreate(&req); err != nil {
		return nil
	}
	category := model.Category{
		Name:     req.Name,
		FatherID: req.FatherID}
	config.Database.Table("categories").Create(&category)
	return c.Status(fiber.StatusCreated).JSON(&category)
}

func CategoryUpdate(c *fiber.Ctx) error {
	var req rtype.CategoryUpdateRequest
	var category model.Category
	if err := c.BodyParser(&req); err != nil {
		return nil
	}
	if err := validator.ValidateCategoryUpdate(&req); err != nil {
		return nil
	}
	if err := DetailPreHandle(c, &category); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if result := config.Database.Table("categories").Model(&category).Updates(&req); result.Error != nil {
		return InternalErrorResponse(c, InternalSQLError, result.Error.Error())
	}
	return c.JSON(&category)
}

func CategoryDelete(c *fiber.Ctx) error {
	var category model.Category
	if err := DetailPreHandle(c, &category); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if result := config.Database.Table("categories").Delete(&category); result.Error != nil {
		return InternalErrorResponse(c, InternalSQLError, result.Error.Error())
	}
	if err := deleteAlterRelated(category.ID); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func deleteAlterRelated(deletedCategoryID uint) error {
	if tx := config.Database.Table("articles").Where("category_id = ?", deletedCategoryID).Update(
		"category_id", nil); tx.Error != nil {
		return tx.Error
	}
	if tx := config.Database.Table("categories").Where("father_id = ?", deletedCategoryID).Delete(
		&model.Category{}); tx.Error != nil {
		return tx.Error
	}
	return nil
}
