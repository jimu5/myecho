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
			api.Post("/articles", middleware.Authentication, handler.ArticleCreate)
			api.Patch("/articles/:id", middleware.Authentication, handler.ArticleUpdate)
			api.Delete("/articles/:id", middleware.Authentication, handler.ArticleDelete)

			api.Patch("/comments/:id", middleware.Authentication, handler.CommentUpdate)

			api.Post("/articles/categories", middleware.Authentication, handler.CategoryCreate)
			api.Patch("/articles/categories/:id", middleware.Authentication, handler.CategoryUpdate)
			api.Delete("/articles/categories/:id", middleware.Authentication, handler.CategoryDelete)

			api.Post("/tags", middleware.Authentication, handler.TagCreate)
			api.Patch("/tags/:id", middleware.Authentication, handler.TagUpdate)
			api.Delete("/tags/:id", middleware.Authentication, handler.TagDelete)
		}
		// 不需要权限的
		{
			// 登录相关
			noNeedAuth := api.Group("")
			noNeedAuth.Post("/login", handler.Login)
			noNeedAuth.Post("/register", handler.Register)

			// 文章相关
			noNeedAuth.Get("/articles", handler.ArticleDisplayList)
			noNeedAuth.Get("/articles/:id", handler.ArticleRetrieve)
			noNeedAuth.Get("/articles/:id/comments", handler.ArticleCommentList)
			noNeedAuth.Post("/articles/:id/comments", handler.CommentCreate)

			noNeedAuth.Get("/articles/categories/all", handler.CategoryAll)

			noNeedAuth.Get("/tags/all", handler.TagListAll)
		}
	}
}
