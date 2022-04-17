package router

import (
	"github.com/Kimiato/myecho/handler"
	"github.com/Kimiato/myecho/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupApiRouter(app *fiber.App) {
	api := app.Group("/api")
	{
		// 需要权限的
		{
			needAuth := api.Group("")
			needAuth.Use(middleware.Authentication)
		}
		// 不需要权限的
		{
			noNeedAuth := api.Group("")
			noNeedAuth.Post("/login", handler.Login)
			noNeedAuth.Post("/register", handler.Register)
		}
	}
}
