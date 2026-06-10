package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"myecho/model"
)

func TestAdminRequired(t *testing.T) {
	tests := []struct {
		name       string
		user       *model.User
		wantStatus int
		wantCode   int
	}{
		{name: "missing user", wantStatus: fiber.StatusUnauthorized, wantCode: Unauthorized},
		{name: "normal user", user: &model.User{PermissionType: model.Normal}, wantStatus: fiber.StatusForbidden, wantCode: Forbidden},
		{name: "admin user", user: &model.User{PermissionType: model.Admin}, wantStatus: fiber.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			if tt.user != nil {
				app.Use(func(c *fiber.Ctx) error {
					c.Locals("user", tt.user)
					return c.Next()
				})
			}
			app.Use(AdminRequired)
			app.Get("/", func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusNoContent)
			})

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if tt.wantCode == 0 {
				return
			}
			var body Error
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d", body.Code, tt.wantCode)
			}
		})
	}
}
