package main

import (
	"github.com/gofiber/swagger"
	"myecho/config/static_config"
	"myecho/handler/api"
	mw "myecho/middleware"

	"github.com/gofiber/fiber/v2"
	_ "myecho/docs"
)

func SetupApiRouter(app *fiber.App) {
	apiRoute := app.Group("/api")
	mos := app.Group(static_config.StorageRootUrl)
	{
		// 需要权限的, TODO: 改造
		{
			apiRoute.Get("/all_articles", mw.Authentication, mw.AdminRequired, api.ArticleAllList)
			apiRoute.Post("/articles", mw.Authentication, mw.AdminRequired, api.ArticleCreate)
			apiRoute.Post("/articles/batch", mw.Authentication, mw.AdminRequired, api.ArticleBatch)
			apiRoute.Patch("/articles/:id", mw.Authentication, mw.AdminRequired, api.ArticleUpdate)
			apiRoute.Delete("/articles/:id", mw.Authentication, mw.AdminRequired, api.ArticleDelete)

			apiRoute.Get("/comments", mw.Authentication, mw.AdminRequired, api.CommentAllList)
			apiRoute.Patch("/comments/:id", mw.Authentication, mw.AdminRequired, api.CommentUpdate)
			apiRoute.Post("/comments/batch", mw.Authentication, mw.AdminRequired, api.CommentBatch)
			apiRoute.Delete("/comments/:id", mw.Authentication, mw.AdminRequired, api.CommentDelete)

			apiRoute.Post("/article/categories", mw.Authentication, mw.AdminRequired, api.ArticleCategoryCreate)
			apiRoute.Post("/link/categories", mw.Authentication, mw.AdminRequired, api.LinkCategoryCreate)
			apiRoute.Patch("/categories/:id", mw.Authentication, mw.AdminRequired, api.CategoryUpdate)
			apiRoute.Delete("/categories/:id", mw.Authentication, mw.AdminRequired, api.CategoryDelete)

			apiRoute.Post("/tags", mw.Authentication, mw.AdminRequired, api.TagCreate)
			apiRoute.Patch("/tags/:id", mw.Authentication, mw.AdminRequired, api.TagUpdate)
			apiRoute.Delete("/tags/:id", mw.Authentication, mw.AdminRequired, api.TagDelete)

			apiRoute.Post("/settings", mw.Authentication, mw.AdminRequired, api.SettingCreate)
			apiRoute.Patch("/settings/:key", mw.Authentication, mw.AdminRequired, api.SettingUpdate)
			apiRoute.Delete("/settings/:key", mw.Authentication, mw.AdminRequired, api.SettingDelete)

			apiRoute.Post("/links", mw.Authentication, mw.AdminRequired, api.LinkCreate)
			apiRoute.Put("/links/:id", mw.Authentication, mw.AdminRequired, api.LinkUpdate)
			apiRoute.Delete("/links/:id", mw.Authentication, mw.AdminRequired, api.LinkDelete)

			apiRoute.Get("/static-pages", mw.Authentication, mw.AdminRequired, api.StaticPageList)
			apiRoute.Post("/static-pages/upload", mw.Authentication, mw.AdminRequired, api.UploadStaticPage)
			apiRoute.Delete("/static-pages/:name", mw.Authentication, mw.AdminRequired, api.DeleteStaticPage)

			mos.Post("/files/vditor_upload", mw.Authentication, mw.AdminRequired, api.VditorFileUpload)
			mos.Post("/files/upload", mw.Authentication, mw.AdminRequired, api.FileSingleUpload)
			mos.Post("/save_url_file", mw.Authentication, mw.AdminRequired, api.FileSaveByLinkUrl)
			mos.Get("/files", mw.Authentication, mw.AdminRequired, api.FilePageList)
			mos.Delete("/files/:id", mw.Authentication, mw.AdminRequired, api.FileDelete)
			mos.Put("/files/:id", mw.Authentication, mw.AdminRequired, api.FileInfoUpdate)
		}
		// 不需要权限的
		{
			// 登录相关
			noNeedAuth := app.Group("/api")
			noNeedAuth.Post("/login", api.Login)
			noNeedAuth.Post("/register", api.Register)

			// 文章相关
			noNeedAuth.Get("/articles", api.ArticleDisplayList)
			noNeedAuth.Get("/articles/:id", api.ArticleRetrieve)
			noNeedAuth.Post("/articles/:id/password", api.ArticlePasswordUnlock)
			noNeedAuth.Get("/articles/:id/comments", api.ArticleCommentList)
			noNeedAuth.Post("/articles/:id/comments", api.CommentCreate)

			noNeedAuth.Get("/article/categories/all", api.CategoryArticleAll)
			noNeedAuth.Get("/link/categories/all", api.CategoryLinkAll)

			noNeedAuth.Get("/settings/:key", api.SettingRetrieve)
			noNeedAuth.Get("/settings", api.SettingAll)
			noNeedAuth.Get("/tags/all", api.TagListAll)

			apiRoute.Get("/links", mw.Authentication, mw.AdminRequired, api.LinkAll)
		}
	}
}

func setSwaggerRoute(app *fiber.App) {
	if *prod {
		app.Get("api/swagger/*", swagger.HandlerDefault)
	}
}

// SetupThemeRouter 设置主题相关的API路由
func SetupThemeRouter(app *fiber.App) {
	api.SetupThemeRouter(app)
}
