package api

import (
	"github.com/gofiber/fiber/v2"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"myecho/model"
	"myecho/service"
)

func CategoryAll(c *fiber.Ctx) error {
	var res []mysql.CategoryModel
	err := connect.Database.Table("categories").Order("id").Find(&res).Error
	if err != nil {
		return err
	}
	return handler.Success(c, &res)
}

func CategoryArticleAll(c *fiber.Ctx) error {
	result, err := service.S.Category.AllByType(model.CategoryTypeArticle)
	if err != nil {
		return err
	}
	return handler.Success(c, &result)
}

func CategoryLinkAll(c *fiber.Ctx) error {
	result, err := service.S.Category.AllByType(model.CategoryTypeLink)
	if err != nil {
		return err
	}
	return handler.Success(c, &result)
}

func getCategoryFromCategoryCreateRequest(c *fiber.Ctx) (mysql.CategoryModel, error) {
	var req rtype.CategoryCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return mysql.CategoryModel{}, err
	}
	if err := req.Validate(); err != nil {
		return mysql.CategoryModel{}, err
	}
	category := req.ToMysqlModel()
	return category, nil
}

func ArticleCategoryCreate(c *fiber.Ctx) error {
	category, err := getCategoryFromCategoryCreateRequest(c)
	if err != nil {
		return err
	}
	if err = service.S.Category.CreateByType(&category, model.CategoryTypeArticle); err != nil {
		return err
	}
	return handler.SuccessWithStatus(c, fiber.StatusCreated, &category)
}

func LinkCategoryCreate(c *fiber.Ctx) error {
	category, err := getCategoryFromCategoryCreateRequest(c)
	if err != nil {
		return err
	}
	if err = service.S.Category.CreateByType(&category, model.CategoryTypeLink); err != nil {
		return err
	}
	return handler.SuccessWithStatus(c, fiber.StatusCreated, &category)
}

func CategoryUpdate(c *fiber.Ctx) error {
	var req rtype.CategoryUpdateRequest
	var category mysql.CategoryModel
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	if err := validator.ValidateCategoryUpdate(&req); err != nil {
		return err
	}
	if err := handler.DetailPreHandleByParam(c, &category); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if result := connect.Database.Table("categories").Model(&category).Updates(&req); result.Error != nil {
		return InternalErrorResponse(c, InternalSQLError, result.Error.Error())
	}
	return handler.Success(c, &category)
}

func CategoryDelete(c *fiber.Ctx) error {
	var category mysql.CategoryModel
	if err := handler.DetailPreHandleByParam(c, &category); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if result := connect.Database.Table("categories").Delete(&category); result.Error != nil {
		return InternalErrorResponse(c, InternalSQLError, result.Error.Error())
	}
	if err := deleteAlterRelated(&category); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.SuccessWithStatus(c, fiber.StatusOK, nil)
}

func deleteAlterRelated(deletedCategory *mysql.CategoryModel) error {
	if tx := connect.Database.Table("articles").Where("category_uid = ?", deletedCategory.UID).Update(
		"category_uid", nil); tx.Error != nil {
		return tx.Error
	}
	if tx := connect.Database.Table("categories").Where("father_uid = ?", deletedCategory.UID).Delete(
		&mysql.CategoryModel{}); tx.Error != nil {
		return tx.Error
	}
	return nil
}
