package api

import (
	"github.com/gofiber/fiber/v2"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/rtype"
	"myecho/service"
)

func LinkCreate(c *fiber.Ctx) error {
	var link mysql.LinkModel
	if err := c.BodyParser(&link); err != nil {
		return err
	}
	if err := service.S.Link.Create(&link); err != nil {
		return err
	}
	return handler.SuccessWithStatus(c, fiber.StatusCreated, &link)
}

func LinkUpdate(c *fiber.Ctx) error {
	var link mysql.LinkModel
	if err := c.BodyParser(&link); err != nil {
		return err
	}
	id, err := handler.GetIDByParam(c, &mysql.LinkModel{})
	if err != nil {
		return err
	}
	link.ID = id
	err = service.S.Link.UpdateByID(id, &link)
	if err != nil {
		return err
	}
	return handler.Success(c, &link)
}

func LinkDelete(c *fiber.Ctx) error {
	id, err := handler.GetIDByParam(c, &mysql.LinkModel{})
	if err != nil {
		return err
	}
	err = service.S.Link.DeleteByID(id)
	if err != nil {
		return err
	}
	return handler.SuccessWithStatus(c, fiber.StatusOK, nil)
}

func LinkAll(c *fiber.Ctx) error {
	var param rtype.LinkQueryParam
	err := c.QueryParser(&param)
	if err != nil {
		return err
	}
	dalParam := param.ToDALParam()
	result, err := service.S.Link.All(&dalParam)
	if err != nil {
		return err
	}
	return handler.Success(c, &result)
}
