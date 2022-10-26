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
		ViewRoute.Get("", view.ArticleDisplayList)
		ViewRoute.Get("/articles/:id", view.ArticleRetrieve)
		ViewRoute.Get("/categories", view.CategoryAll)
	}
}
