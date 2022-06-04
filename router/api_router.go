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

			needAuth.Post("/articles/categories", handler.CategoryCreate)
			needAuth.Patch("/articles/categories/:id", handler.CategoryUpdate)
			needAuth.Delete("/articles/categories/:id", handler.CategoryDelete)
		}
		// 不需要权限的
		{
			// 登录相关
			noNeedAuth := api.Group("")
			noNeedAuth.Post("/login", handler.Login)
			noNeedAuth.Post("/register", handler.Register)

			// 文章相关
			noNeedAuth.Get("/articles", handler.ArticleList)
			noNeedAuth.Get("/articles/:id", handler.ArticleRetrieve)
			noNeedAuth.Get("/articles/:id/comments", handler.ArticleCommentList)
			noNeedAuth.Post("/articles/:id/comments", handler.CommentCreate)

			noNeedAuth.Get("/articles/categories/all", handler.CategoryAll)
		}
	}
}
