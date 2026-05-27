package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAuthenticationRejectsMalformedAuthorization(t *testing.T) {
	app := fiber.New()
	app.Get("/private", Authentication, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	testCases := []string{
		"",
		"token",
		"Bearer abc",
		"token abc extra",
	}
	for _, auth := range testCases {
		req := httptest.NewRequest(fiber.MethodGet, "/private", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test() error = %v", err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("Authorization %q status = %d, want %d", auth, resp.StatusCode, fiber.StatusUnauthorized)
		}
	}
}
