package handler

import (
	"myecho/config"
	"myecho/handler/rtype"
	"myecho/handler/validator"
	"myecho/model"

	"github.com/gofiber/fiber/v2"
)

func TagListAll(c *fiber.Ctx) error {
	var tags []model.Tag
	config.Database.Table("tags").Find(&tags)
	return c.JSON(&tags)
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
	config.Database.Table("tags").Create(&res)
	return c.Status(fiber.StatusCreated).JSON(&res)
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
	if err := DetailPreHandle(c, &tag); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	tag.Name = req.Name
	config.Database.Table("tags").Save(&tag)
	return c.JSON(&tag)
}

func TagDelete(c *fiber.Ctx) error {
	var tag model.Tag
	if err := DetailPreHandle(c, &tag); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	config.Database.Table("tags").Delete(&tag)
	deleteAlterDelete(&tag)
	return c.SendStatus(fiber.StatusNoContent)
}

func deleteAlterDelete(tag *model.Tag) {
	config.Database.Table("articles").Association("Tags").Delete(tag)
}

func FindTags(tags *[]model.Tag) {
	config.Database.Table("tags").Where("id in (?)", tags).Find(&tags)
}
