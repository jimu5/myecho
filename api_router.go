package main

import (
	"github.com/gofiber/swagger"
	"myecho/config/static_config"
	"myecho/handler/api"
	mw "myecho/middleware"

	"github.com/gofiber/fiber/v2"
	"myecho/docs"
)

func SetupApiRouter(app *fiber.App) {
	apiRoute := app.Group("/api")
	apiRoute.Get("/openapi.json", func(c *fiber.Ctx) error {
		return c.Type("json").Send(docs.OpenAPI)
	})
	mos := app.Group(static_config.StorageRootUrl)
	{
		// 需要权限的, TODO: 改造
		{
			apiRoute.Get("/all_articles", mw.Authentication, mw.AuthorRequired, api.ArticleAllList)
			apiRoute.Post("/articles", mw.Authentication, mw.AuthorRequired, api.ArticleCreate)
			apiRoute.Post("/articles/:id/duplicate", mw.Authentication, mw.AuthorRequired, api.ArticleDuplicate)
			apiRoute.Post("/articles/:id/previews", mw.Authentication, mw.AuthorRequired, api.ArticlePreviewCreate)
			apiRoute.Delete("/articles/:id/previews", mw.Authentication, mw.AuthorRequired, api.ArticlePreviewRevoke)
			apiRoute.Post("/articles/batch", mw.Authentication, mw.EditorRequired, api.ArticleBatch)
			apiRoute.Get("/articles/:id/revisions", mw.Authentication, mw.AuthorRequired, api.ArticleRevisionList)
			apiRoute.Get("/articles/:id/revisions/:revision_id", mw.Authentication, mw.AuthorRequired, api.ArticleRevisionRetrieve)
			apiRoute.Post("/articles/:id/revisions/:revision_id/restore", mw.Authentication, mw.AuthorRequired, api.ArticleRevisionRestore)
			apiRoute.Get("/articles/:id/reviews", mw.Authentication, mw.AuthorRequired, api.ArticleReviewList)
			apiRoute.Post("/articles/:id/review", mw.Authentication, mw.EditorRequired, api.ArticleReview)
			apiRoute.Get("/submissions", mw.Authentication, mw.EditorRequired, api.ArticleSubmissionList)
			apiRoute.Get("/submissions/authors", mw.Authentication, mw.EditorRequired, api.ArticleSubmissionAuthorList)
			apiRoute.Get("/submissions/:id", mw.Authentication, mw.AuthorRequired, api.ArticleSubmissionRetrieve)
			apiRoute.Post("/submissions/:id/review", mw.Authentication, mw.EditorRequired, api.ArticleSubmissionReview)
			apiRoute.Post("/submissions/:id/withdraw", mw.Authentication, mw.AuthorRequired, api.ArticleSubmissionWithdraw)
			apiRoute.Patch("/articles/:id", mw.Authentication, mw.AuthorRequired, api.ArticleUpdate)
			apiRoute.Delete("/articles/:id", mw.Authentication, mw.AuthorRequired, api.ArticleDelete)
			apiRoute.Post("/articles/:id/restore", mw.Authentication, mw.AuthorRequired, api.ArticleTrashRestore)
			apiRoute.Delete("/articles/:id/permanent", mw.Authentication, mw.AuthorRequired, api.ArticlePermanentlyDelete)
			apiRoute.Get("/dashboard", mw.Authentication, mw.AdminRequired, api.Dashboard)
			apiRoute.Get("/export", mw.Authentication, mw.AdminRequired, api.Export)
			apiRoute.Get("/content/export", mw.Authentication, mw.AdminRequired, api.ContentExport)
			apiRoute.Post("/content/import", mw.Authentication, mw.AdminRequired, api.ContentImport)

			apiRoute.Get("/comments", mw.Authentication, mw.EditorRequired, api.CommentAllList)
			apiRoute.Patch("/comments/:id", mw.Authentication, mw.EditorRequired, api.CommentUpdate)
			apiRoute.Post("/comments/:id/reply", mw.Authentication, mw.EditorRequired, api.CommentReply)
			apiRoute.Post("/comments/batch", mw.Authentication, mw.EditorRequired, api.CommentBatch)
			apiRoute.Delete("/comments/:id", mw.Authentication, mw.EditorRequired, api.CommentDelete)

			apiRoute.Get("/account/profile", mw.Authentication, api.Profile)
			apiRoute.Get("/account/tokens", mw.Authentication, api.AgentTokenList)
			apiRoute.Post("/account/tokens", mw.Authentication, api.AgentTokenCreate)
			apiRoute.Delete("/account/tokens/:id", mw.Authentication, api.AgentTokenRevoke)
			apiRoute.Patch("/account/profile", mw.Authentication, api.ProfileUpdate)
			apiRoute.Patch("/account/password", mw.Authentication, api.PasswordUpdate)
			apiRoute.Post("/logout", mw.Authentication, api.Logout)

			apiRoute.Post("/article/categories", mw.Authentication, mw.EditorRequired, api.ArticleCategoryCreate)
			apiRoute.Post("/link/categories", mw.Authentication, mw.EditorRequired, api.LinkCategoryCreate)
			apiRoute.Patch("/categories/:id", mw.Authentication, mw.EditorRequired, api.CategoryUpdate)
			apiRoute.Delete("/categories/:id", mw.Authentication, mw.EditorRequired, api.CategoryDelete)

			apiRoute.Post("/tags", mw.Authentication, mw.EditorRequired, api.TagCreate)
			apiRoute.Patch("/tags/:id", mw.Authentication, mw.EditorRequired, api.TagUpdate)
			apiRoute.Delete("/tags/:id", mw.Authentication, mw.EditorRequired, api.TagDelete)

			apiRoute.Post("/settings", mw.Authentication, mw.AdminRequired, api.SettingCreate)
			apiRoute.Get("/settings/admin", mw.Authentication, mw.AdminRequired, api.SettingAdminAll)
			apiRoute.Patch("/settings/:key", mw.Authentication, mw.AdminRequired, api.SettingUpdate)
			apiRoute.Delete("/settings/:key", mw.Authentication, mw.AdminRequired, api.SettingDelete)
			apiRoute.Post("/import", mw.Authentication, mw.AdminRequired, api.Import)

			apiRoute.Post("/links", mw.Authentication, mw.AdminRequired, api.LinkCreate)
			apiRoute.Put("/links/:id", mw.Authentication, mw.AdminRequired, api.LinkUpdate)
			apiRoute.Delete("/links/:id", mw.Authentication, mw.AdminRequired, api.LinkDelete)

			apiRoute.Get("/static-pages", mw.Authentication, mw.AdminRequired, api.StaticPageList)
			apiRoute.Post("/static-pages/upload", mw.Authentication, mw.AdminRequired, api.UploadStaticPage)
			apiRoute.Patch("/static-pages/:name", mw.Authentication, mw.AdminRequired, api.UpdateStaticPage)
			apiRoute.Delete("/static-pages/:name", mw.Authentication, mw.AdminRequired, api.DeleteStaticPage)

			mos.Post("/files/vditor_upload", mw.Authentication, mw.AuthorRequired, api.VditorFileUpload)
			mos.Post("/files/upload", mw.Authentication, mw.AuthorRequired, api.FileSingleUpload)
			mos.Post("/save_url_file", mw.Authentication, mw.AuthorRequired, api.FileSaveByLinkUrl)
			mos.Get("/files", mw.Authentication, mw.AuthorRequired, api.FilePageList)
			mos.Get("/files/:id/usages", mw.Authentication, mw.AuthorRequired, api.FileUsages)
			mos.Delete("/files/:id", mw.Authentication, mw.AuthorRequired, api.FileDelete)
			mos.Put("/files/:id", mw.Authentication, mw.AuthorRequired, api.FileInfoUpdate)
		}
		// 不需要权限的
		{
			// 登录相关
			noNeedAuth := app.Group("/api")
			noNeedAuth.Post("/login", api.Login)
			noNeedAuth.Post("/register", api.Register)
			noNeedAuth.Get("/setup/status", api.SetupStatus)
			noNeedAuth.Post("/setup", api.Setup)

			// 文章相关
			noNeedAuth.Get("/articles", mw.OptionalAuthentication, api.ArticleDisplayList)
			noNeedAuth.Get("/articles/:id", mw.OptionalAuthentication, api.ArticleRetrieve)
			noNeedAuth.Post("/articles/:id/password", api.ArticlePasswordUnlock)
			noNeedAuth.Get("/articles/:id/comments", api.ArticleCommentList)
			noNeedAuth.Post("/articles/:id/comments", api.CommentCreate)

			noNeedAuth.Get("/article/categories/all", mw.OptionalAuthentication, api.CategoryArticleAll)
			noNeedAuth.Get("/link/categories/all", mw.OptionalAuthentication, api.CategoryLinkAll)

			noNeedAuth.Get("/settings/:key", mw.OptionalAuthentication, api.SettingRetrieve)
			noNeedAuth.Get("/settings", mw.OptionalAuthentication, api.SettingAll)
			noNeedAuth.Get("/tags/all", mw.OptionalAuthentication, api.TagListAll)

			apiRoute.Get("/links", mw.Authentication, mw.AdminRequired, api.LinkAll)
		}
	}
}

func setSwaggerRoute(app *fiber.App) {
	if !*prod {
		app.Get("/api/swagger/*", swagger.New(swagger.Config{URL: "/api/openapi.json"}))
	}
}

// SetupThemeRouter 设置主题相关的API路由
func SetupThemeRouter(app *fiber.App) {
	api.SetupThemeRouter(app)
}
