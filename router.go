package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"myecho/handler/view"
)

func SetupViewRouter(app *fiber.App) {
	ViewRoute := app.Group("")
	{
		app.Get("/status", monitor.New()) // 监控
	}
	{
		ViewRoute.Get("favicon.ico", view.GetFavicon)
		ViewRoute.Get("/theme-preview", view.ThemePreview)
		ViewRoute.Get("/theme-preview/clear", view.ClearThemePreview)
		ViewRoute.Get("", view.ArticleDisplayList)
		ViewRoute.Get("/rss.xml", view.RSS)
		ViewRoute.Get("/sitemap.xml", view.Sitemap)
		ViewRoute.Get("/articles/:id", view.ArticleRetrieve)
		ViewRoute.Get("/article/categories", view.CategoryArticleAll)
		ViewRoute.Get("/links", view.LinkAll)
	}
}
