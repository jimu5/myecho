package api

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSetupThemeRouterRegistersRoutes(t *testing.T) {
	app := fiber.New()
	SetupThemeRouter(app)
	if len(app.GetRoutes()) == 0 {
		t.Fatal("expected theme routes to be registered")
	}
}
