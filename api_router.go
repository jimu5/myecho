package main

import (
	"myecho/config/static_config"
	"myecho/handler/api"
	mw "myecho/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupApiRouter(app *fiber.App) {
	apiRoute := app.Group("/api")
	mos := app.Group(static_config.StorageRootUrl)
	{
		// 需要权限的, TODO: 改造
		{
			apiRoute.Get("/all_articles", mw.Authentication, api.ArticleAllList)
			apiRoute.Post("/articles", mw.Authentication, api.ArticleCreate)
			apiRoute.Patch("/articles/:id", mw.Authentication, api.ArticleUpdate)
			apiRoute.Delete("/articles/:id", mw.Authentication, api.ArticleDelete)

			apiRoute.Patch("/comments/:id", mw.Authentication, api.CommentUpdate)

			apiRoute.Post("/article/categories", mw.Authentication, api.ArticleCategoryCreate)
			apiRoute.Post("/link/categories", mw.Authentication, api.LinkCategoryCreate)
			apiRoute.Patch("/categories/:id", mw.Authentication, api.CategoryUpdate)
			apiRoute.Delete("/categories/:id", mw.Authentication, api.CategoryDelete)

			apiRoute.Post("/tags", mw.Authentication, api.TagCreate)
			apiRoute.Patch("/tags/:id", mw.Authentication, api.TagUpdate)
			apiRoute.Delete("/tags/:id", mw.Authentication, api.TagDelete)

			apiRoute.Post("/settings", mw.Authentication, api.SettingCreate)
			apiRoute.Patch("/settings/:key", mw.Authentication, api.SettingUpdate)
			apiRoute.Delete("settings/:key", mw.Authentication, api.SettingDelete)

			apiRoute.Post("/links", mw.Authentication, api.LinkCreate)
			apiRoute.Put("/links/:id", mw.Authentication, api.LinkUpdate)
			apiRoute.Delete("/links/:id", mw.Authentication, api.LinkDelete)

			mos.Post("/upload", mw.Authentication, api.FileUpload)
			mos.Post("/save_url_file", mw.Authentication, api.FileSaveByLinkUrl)
			mos.Get("/files", mw.Authentication, api.FilePageList)
			mos.Delete("/files/:id", mw.Authentication, api.FileDelete)
			mos.Put("/files/:id", mw.Authentication, api.FileInfoUpdate)
		}
		// 不需要权限的
		{
			// 登录相关
			noNeedAuth := apiRoute.Group("")
			noNeedAuth.Post("/login", api.Login)
			noNeedAuth.Post("/register", api.Register)

			// 文章相关
			noNeedAuth.Get("/articles", api.ArticleDisplayList)
			noNeedAuth.Get("/articles/:id", api.ArticleRetrieve)
			noNeedAuth.Get("/articles/:id/comments", api.ArticleCommentList)
			noNeedAuth.Post("/articles/:id/comments", api.CommentCreate)

			noNeedAuth.Get("/article/categories/all", api.CategoryArticleAll)
			noNeedAuth.Get("/link/categories/all", api.CategoryLinkAll)

			noNeedAuth.Get("/settings/:key", api.SettingRetrieve)
			noNeedAuth.Get("/settings", api.SettingAll)
			noNeedAuth.Get("/tags/all", api.TagListAll)

			apiRoute.Get("/links", mw.Authentication, api.LinkAll)
		}
	}
}
