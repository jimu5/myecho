package main

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSetupApiRouterRegistersStaticPageAdminRoutes(t *testing.T) {
	app := fiber.New()
	SetupApiRouter(app)

	want := map[string]bool{
		fiber.MethodGet + " /api/static-pages":          false,
		fiber.MethodPost + " /api/static-pages/upload":  false,
		fiber.MethodDelete + " /api/static-pages/:name": false,
	}
	for _, route := range app.GetRoutes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Fatalf("static page route %s was not registered", route)
		}
	}
}
