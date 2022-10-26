package middleware

import (
	"github.com/gofiber/fiber/v2"
	"myecho/handler/api"
	"strings"
)

func Custom404ErrorHandler(c *fiber.Ctx) error {
	// TODO: 后面需要修改下，判断下是否在static文件中存在，
	if strings.HasPrefix(c.Path(), "/api") {
		return nil
	}
	return c.Status(fiber.StatusOK).SendFile("./static/admin/index.html")
}

func CommonErrorHandler(c *fiber.Ctx) error {
	err := c.Next()
	if err != nil && strings.HasPrefix(c.Path(), "/api") {
		return c.Status(fiber.StatusBadRequest).JSON(api.Error{Code: api.CommonBadError, Msg: err.Error()})
	}
	// TODO: 需要增加普通页面的错误返回
	return err
}
