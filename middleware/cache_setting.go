package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"time"
)

var CacheConfig = cache.Config{
	Next: func(c *fiber.Ctx) bool {
		path := c.Path()
		if len(path) > 4 {
			if path[:4] == "/api" {
				return true
			}
		}
		if path == "/status" {
			return true
		}
		return false
	},
	Expiration: 5 * time.Second,
}
