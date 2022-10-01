package main

import (
	"myecho/config"
	"myecho/handler"
	mw "myecho/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupApiRouter(app *fiber.App) {
	api := app.Group("/api")
	mos := app.Group(config.StorageRootUrl)
	{
		// 需要权限的, TODO: 改造
		{
			api.Get("/all_articles", mw.Authentication, handler.ArticleAllList)
			api.Post("/articles", mw.Authentication, handler.ArticleCreate)
			api.Patch("/articles/:id", mw.Authentication, handler.ArticleUpdate)
			api.Delete("/articles/:id", mw.Authentication, handler.ArticleDelete)

			api.Patch("/comments/:id", mw.Authentication, handler.CommentUpdate)

			api.Post("/articles/categories", mw.Authentication, handler.CategoryCreate)
			api.Patch("/articles/categories/:id", mw.Authentication, handler.CategoryUpdate)
			api.Delete("/articles/categories/:id", mw.Authentication, handler.CategoryDelete)

			api.Post("/tags", mw.Authentication, handler.TagCreate)
			api.Patch("/tags/:id", mw.Authentication, handler.TagUpdate)
			api.Delete("/tags/:id", mw.Authentication, handler.TagDelete)

			mos.Post("upload", mw.Authentication, handler.UploadFile)
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
