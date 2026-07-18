package middleware

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"myecho/service"
	"strings"
	"time"
)

var passCacheRoutePathPrefix = []string{
	"/api",
	"/mos",
	"/status",
	"/theme-preview",
	"/articles/",
	"/posts/",
	"/pages/",
}

var themeIndependentCachePathPrefix = []string{
	"/admin",
	"/static",
	"/themes",
	"/favicon.ico",
}

var CacheConfig = cache.Config{
	Next: func(c *fiber.Ctx) bool {
		return isPathSkipCache(c.Path()) || c.Cookies(service.ThemePreviewCookieName) != ""
	},
	Expiration: 5 * time.Second,
	KeyGenerator: func(ctx *fiber.Ctx) string {
		if isPathSkipCache(ctx.Path()) || isThemeIndependentCachePath(ctx.Path()) {
			return ctx.OriginalURL()
		}
		if ctx.Cookies(service.ThemePreviewCookieName) != "" {
			return ctx.OriginalURL() + "|theme-preview"
		}
		theme, err := service.S.Theme.GetActiveTheme()
		if err != nil || theme == nil {
			return ctx.OriginalURL()
		}
		return fmt.Sprintf("%s|theme:%d:%d", ctx.OriginalURL(), theme.ID, theme.UpdatedAt.UnixNano())
	},
}

func isThemeIndependentCachePath(path string) bool {
	for i := range themeIndependentCachePathPrefix {
		if strings.HasPrefix(path, themeIndependentCachePathPrefix[i]) {
			return true
		}
	}
	return false
}

func isPathSkipCache(path string) bool {
	for i := range passCacheRoutePathPrefix {
		if strings.HasPrefix(path, passCacheRoutePathPrefix[i]) {
			return true
		}
	}
	return false
}
