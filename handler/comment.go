package handler

import (
	"myecho/config"
	"myecho/handler/rtype"
	"myecho/handler/validator"
	"myecho/model"

	"github.com/gofiber/fiber/v2"
)

func CommentCreate(c *fiber.Ctx) error {
	var res rtype.CommentRequest
	if err := c.BodyParser(&res); err != nil {
		return nil
	}
	if err := validator.ValidateCommentRequest(&res); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	var comment model.Comment
	structAssign(&comment, &res)
	config.Database.Save(&comment)
	return c.Status(fiber.StatusCreated).JSON(res)
}
