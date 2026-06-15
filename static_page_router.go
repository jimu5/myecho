package main

import (
	"myecho/service"

	"github.com/gofiber/fiber/v2"
)

func SetupStaticPageStaticRoute(app *fiber.App) {
	app.Static("/static-pages", service.StaticPageStorageDir, fiber.Static{
		Index: "index.html",
	})
}
