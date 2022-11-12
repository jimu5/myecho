package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"strings"
	"time"
)

var passCacheRoutePathPrefix = []string{
	"/api",
	"/mos",
	"/status",
}

var CacheConfig = cache.Config{
	Next: func(c *fiber.Ctx) bool {
		return isPathSkipCache(c.Path())
	},
	Expiration: 5 * time.Second,
	KeyGenerator: func(ctx *fiber.Ctx) string {
		return ctx.OriginalURL()
	},
}

func isPathSkipCache(path string) bool {
	for i := range passCacheRoutePathPrefix {
		if strings.HasPrefix(path, passCacheRoutePathPrefix[i]) {
			return true
		}
	}
	return false
}
