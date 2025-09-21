package api

import (
	"myecho/handler/api/theme"
	"myecho/middleware"

	"github.com/gofiber/fiber/v2"
)

// SetupThemeRouter 设置主题相关的API路由
func SetupThemeRouter(app *fiber.App) {
	// 创建主题API分组，需要认证

	themeGroup := app.Group("/api/themes", middleware.Authentication)
	{
		// 创建主题
		themeGroup.Post("", theme.CreateTheme)
		// 获取所有主题
		themeGroup.Get("", theme.GetAllThemes)
		// 获取单个主题
		themeGroup.Get("/:id", theme.GetTheme)
		// 更新主题
		themeGroup.Patch("/:id", theme.UpdateTheme)
		// 删除主题
		themeGroup.Delete("/:id", theme.DeleteTheme)
		// 激活主题
		themeGroup.Post("/:id/activate", theme.ActivateTheme)
		// 更新主题配置
		themeGroup.Patch("/:id/config", theme.UpdateThemeConfig)
	}

	// 无需认证的主题API
	publicThemeGroup := app.Group("/api/themes")
	{
		// 获取当前激活的主题
		publicThemeGroup.Get("/active", theme.GetActiveTheme)
	}
}
