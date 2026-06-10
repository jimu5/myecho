package api

import (
	"myecho/dal/connect"
	"myecho/handler"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"myecho/model"

	"github.com/gofiber/fiber/v2"
)

func TagListAll(c *fiber.Ctx) error {
	var tags []model.Tag
	if err := connect.Database.Table("tags").Find(&tags).Error; err != nil {
		return err
	}
	return handler.Success(c, &tags)
}

func TagCreate(c *fiber.Ctx) error {
	var req rtype.TagRequest
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if err := validator.ValidateTagRequest(&req); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	res := model.Tag{
		Name: req.Name,
	}
	if err := connect.Database.Table("tags").Create(&res).Error; err != nil {
		return err
	}
	return handler.SuccessWithStatus(c, fiber.StatusCreated, &res)
}

func TagUpdate(c *fiber.Ctx) error {
	var req rtype.TagRequest
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if err := validator.ValidateTagRequest(&req); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	var tag model.Tag
	if err := handler.DetailPreHandleByParam(c, &tag); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	tag.Name = req.Name
	if err := connect.Database.Table("tags").Save(&tag).Error; err != nil {
		return err
	}
	return handler.Success(c, &tag)
}

func TagDelete(c *fiber.Ctx) error {
	var tag model.Tag
	if err := handler.DetailPreHandleByParam(c, &tag); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if err := connect.Database.Table("tags").Delete(&tag).Error; err != nil {
		return err
	}
	if err := deleteAlterDelete(&tag); err != nil {
		return err
	}
	return handler.SuccessWithStatus(c, fiber.StatusOK, nil)
}

func deleteAlterDelete(tag *model.Tag) error {
	return connect.Database.Exec("DELETE FROM article_tags WHERE tag_uid = ?", tag.UID).Error
}

func FindTags(tags []*model.Tag) {
	_ = connect.Database.Table("tags").Find(&tags).Error
}

func FindTagsByUID(uids []string) ([]*model.Tag, error) {
	result := make([]*model.Tag, 0)
	err := connect.Database.Model(&model.Tag{}).Where("uid in (?)", uids).Find(&result).Error
	return result, err
}
