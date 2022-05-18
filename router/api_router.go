package router

import (
	"myecho/handler"
	"myecho/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupApiRouter(app *fiber.App) {
	api := app.Group("/api")
	{
		// 需要权限的, TODO: 改造
		{
			needAuth := api.Group("")
			needAuth.Post("/articles", middleware.Authentication, handler.ArticleCreate)
			needAuth.Patch("/articles/:id", middleware.Authentication, handler.ArticleUpdate)
			needAuth.Delete("/articles/:id", middleware.Authentication, handler.ArticleDelete)

			needAuth.Patch("/comments/:id", middleware.Authentication, handler.CommentUpdate)
		}
		// 不需要权限的
		{
			noNeedAuth := api.Group("")
			noNeedAuth.Post("/login", handler.Login)
			noNeedAuth.Post("/register", handler.Register)

			noNeedAuth.Get("/articles", handler.ArticleList)
			noNeedAuth.Get("/articles/:id", handler.ArticleRetrieve)
			noNeedAuth.Get("/articles/:id/comments", handler.ArticleCommentList)
			noNeedAuth.Post("/articles/:id/comments", handler.CommentCreate)
		}
	}
}
