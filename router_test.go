package main

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSetupRoutersRegisterWithoutPanic(t *testing.T) {
	app := fiber.New()
	SetupApiRouter(app)
	SetupThemeRouter(app)
	setSwaggerRoute(app)
	SetupViewRouter(app)

	routes := app.GetRoutes()
	if len(routes) == 0 {
		t.Fatal("expected routes to be registered")
	}
}
