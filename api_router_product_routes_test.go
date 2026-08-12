package main

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSetupApiRouter_ProductRoutes_BitsUT(t *testing.T) {
	app := fiber.New()
	SetupApiRouter(app)

	routeCounts := make(map[string]int)
	for _, route := range app.GetRoutes() {
		routeCounts[route.Method+" "+route.Path]++
	}

	for _, route := range []string{
		fiber.MethodGet + " /api/articles/:id/revisions",
		fiber.MethodPost + " /api/articles/:id/revisions/:revision_id/restore",
		fiber.MethodPost + " /api/comments/:id/reply",
		fiber.MethodGet + " /api/account/profile",
		fiber.MethodPatch + " /api/account/profile",
		fiber.MethodPatch + " /api/account/password",
		fiber.MethodPost + " /api/logout",
		fiber.MethodGet + " /api/settings/admin",
		fiber.MethodPost + " /api/import",
	} {
		t.Run(route, func(t *testing.T) {
			if got := routeCounts[route]; got != 1 {
				t.Fatalf("route registration count = %d, want 1", got)
			}
		})
	}
}
