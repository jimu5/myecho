package router

import (
	"github.com/Kimiato/myecho/handler"
	"github.com/Kimiato/myecho/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupApiRouter(app *fiber.App) {
	api := app.Group("/api")
	{
		// 需要权限的, TODO: 改造
		{
			needAuth := api.Group("")
			needAuth.Post("/article", middleware.Authentication, handler.ArticleCreate)
			needAuth.Patch("/articles/:id", middleware.Authentication, handler.ArticleUpdate)
			needAuth.Delete("/articles/:id", middleware.Authentication, handler.ArticleDelete)
		}
		// 不需要权限的
		{
			noNeedAuth := api.Group("")
			noNeedAuth.Post("/login", handler.Login)
			noNeedAuth.Post("/register", handler.Register)

			noNeedAuth.Get("/articles", handler.ArticleList)
			noNeedAuth.Get("/articles/:id", handler.ArticleRetrieve)
		}
	}
}
