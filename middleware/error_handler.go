package middleware

import (
	"github.com/gofiber/fiber/v2"
)

func Custom404ErrorHandler(ctx *fiber.Ctx) (err error) {
	// TODO: 先简单这样处理下，后面需要修改下，判断下是否在static文件中存在，以及api的路由不返回这个
	ctx.Status(fiber.StatusOK).SendFile("./static/admin/index.html")
	return nil
}
