package main

import (
	"github.com/gofiber/fiber/v2"
	"myecho/handler/view"
)

func SetupViewRouter(app *fiber.App) {
	ViewRoute := app.Group("")
	{
		ViewRoute.Get("", view.ArticleDisplayList)
		ViewRoute.Get("/articles/:id", view.ArticleRetrieve)
	}
}
