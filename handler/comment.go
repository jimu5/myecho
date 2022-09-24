package handler

import (
	"myecho/dal/connect"
	"myecho/handler/rtype"
	"myecho/handler/validator"
	"myecho/model"

	"github.com/gofiber/fiber/v2"
)

func CommentCreate(c *fiber.Ctx) error {
	var res rtype.CommentRequest
	var article model.Article
	if err := c.BodyParser(&res); err != nil {
		return nil
	}
	if err := validator.ValidateCommentRequest(&res); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if err := DetailPreHandle(c, &article); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}

	var comment model.Comment
	comment.ArticleID = article.ID
	structAssign(&comment, &res)
	if c.Locals("user") != nil {
		comment.UserID = c.Locals("user").(*model.User).ID
		comment.AuthorName = c.Locals("user").(*model.User).NickName
		comment.AuthorEmail = c.Locals("user").(*model.User).Email
	}
	connect.Database.Save(&comment)
	return c.Status(fiber.StatusCreated).JSON(res)
}

// 更新评论
func CommentUpdate(c *fiber.Ctx) error {
	var r rtype.CommentRequest
	// 校验
	if err := c.BodyParser(&r); err != nil {
		return nil
	}

	var comment model.Comment
	if err := validator.ValidateCommentRequest(&r); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	structAssign(&comment, &r)
	connect.Database.Updates(&comment)
	return c.Status(fiber.StatusOK).JSON(comment)
}

// 获取文章评论
func ArticleCommentList(c *fiber.Ctx) error {
	var comments []rtype.CommentResponse
	var article model.Article
	// 校验
	if err := DetailPreHandle(c, &article); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	connect.Database.Table("comments").Where("article_id = ?", article.ID).Find(&comments)
	return c.Status(fiber.StatusOK).JSON(comments)
}
