package middleware

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
)

func TestCacheMiddlewareDoesNotHideRevokedOrExpiredArticlePreview(t *testing.T) {
	for _, reason := range []string{"revoked", "expired"} {
		for _, path := range []string{"/article-preview/opaque-token", "/ARTICLE-PREVIEW/opaque-token"} {
			t.Run(reason+path, func(t *testing.T) {
				app := fiber.New()
				app.Use(cache.New(CacheConfig))
				valid := true
				calls := 0
				app.Get("/article-preview/:token", func(c *fiber.Ctx) error {
					calls++
					if !valid {
						return c.SendStatus(fiber.StatusNotFound)
					}
					return c.SendString("preview snapshot")
				})

				request := func() (int, string, error) {
					req := httptest.NewRequest(fiber.MethodGet, path, nil)
					resp, err := app.Test(req)
					if err != nil {
						return 0, "", err
					}
					defer resp.Body.Close()
					body, err := io.ReadAll(resp.Body)
					return resp.StatusCode, string(body), err
				}

				firstStatus, firstBody, err := request()
				if err != nil {
					t.Fatalf("first app.Test() error = %v", err)
				}
				if firstStatus != fiber.StatusOK || firstBody != "preview snapshot" {
					t.Fatalf("first response = %d %q, want 200 preview", firstStatus, firstBody)
				}

				valid = false
				secondStatus, _, err := request()
				if err != nil {
					t.Fatalf("second app.Test() error = %v", err)
				}
				if secondStatus != fiber.StatusNotFound {
					t.Fatalf("response after preview became %s = %d, want 404", reason, secondStatus)
				}
				if calls != 2 {
					t.Fatalf("preview handler calls = %d, want a fresh validation on both requests", calls)
				}
			})
		}
	}
}
