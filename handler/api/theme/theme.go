package theme

import (
	"myecho/dal/mysql"
	"myecho/handler/api/errors"
	"myecho/model"
	"myecho/service"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// CreateTheme 创建主题
func CreateTheme(c *fiber.Ctx) error {
	theme := mysql.ThemeModel{}
	if err := c.BodyParser(&theme); err != nil {
		return err
	}
	if err := service.S.Theme.CreateTheme(&theme); err != nil {
		return err
	}
	return c.JSON(&theme)
}

// GetAllThemes 获取所有主题
func GetAllThemes(c *fiber.Ctx) error {
	themes, err := service.S.Theme.GetAllThemes()
	if err != nil {
		return err
	}

	// 创建一个新的响应数组，包含解析后的配置
	respThemes := make([]map[string]interface{}, 0, len(themes))
	for _, theme := range themes {
		// 转换为 model.Theme 类型以使用 GetConfig 方法
		modelTheme := (*model.Theme)(theme)
		config, err := modelTheme.GetConfig()
		if err != nil {
			config = make(map[string]interface{})
		}
		
		// 创建响应对象
		respTheme := map[string]interface{}{
			"id":           theme.ID,
			"name":         theme.Name,
			"display_name": theme.DisplayName,
			"author":       theme.Author,
			"version":      theme.Version,
			"description":  theme.Description,
			"preview":      theme.Preview,
			"css":          theme.CSS,
			"js":           theme.JS,
			"is_default":   theme.IsDefault,
			"is_active":    theme.IsActive,
			"config":       config,
			"created_at":   theme.CreatedAt,
			"updated_at":   theme.UpdatedAt,
		}
		respThemes = append(respThemes, respTheme)
	}

	return c.JSON(respThemes)
}

// GetTheme 获取单个主题
func GetTheme(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return errors.ErrInvalidParams
	}
	theme, err := service.S.Theme.GetThemeByID(id)
	if err != nil {
		return err
	}

	// 转换为 model.Theme 类型以使用 GetConfig 方法
	modelTheme := (*model.Theme)(theme)
	config, err := modelTheme.GetConfig()
	if err == nil {
		// 创建一个新的响应对象，包含解析后的配置
		respTheme := map[string]interface{}{
			"id":           theme.ID,
			"name":         theme.Name,
			"display_name": theme.DisplayName,
			"author":       theme.Author,
			"version":      theme.Version,
			"description":  theme.Description,
			"preview":      theme.Preview,
			"css":          theme.CSS,
			"js":           theme.JS,
			"is_default":   theme.IsDefault,
			"is_active":    theme.IsActive,
			"config":       config,
			"created_at":   theme.CreatedAt,
			"updated_at":   theme.UpdatedAt,
		}
		return c.JSON(respTheme)
	}

	return c.JSON(&theme)
}

// UpdateTheme 更新主题
func UpdateTheme(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return errors.ErrInvalidParams
	}
	theme, err := service.S.Theme.GetThemeByID(id)
	if err != nil {
		return err
	}

	// 解析请求体
	updateData := make(map[string]interface{})
	if err := c.BodyParser(&updateData); err != nil {
		return err
	}

	// 更新主题信息
	if name, ok := updateData["name"].(string); ok {
		theme.Name = name
	}
	if displayName, ok := updateData["display_name"].(string); ok {
		theme.DisplayName = displayName
	}
	if author, ok := updateData["author"].(string); ok {
		theme.Author = author
	}
	if version, ok := updateData["version"].(string); ok {
		theme.Version = version
	}
	if description, ok := updateData["description"].(string); ok {
		theme.Description = description
	}
	if preview, ok := updateData["preview"].(string); ok {
		theme.Preview = preview
	}
	if css, ok := updateData["css"].(string); ok {
		theme.CSS = css
	}
	if js, ok := updateData["js"].(string); ok {
		theme.JS = js
	}
	if config, ok := updateData["config"]; ok {
		// 确保 config 是 map[string]interface{} 类型
				if configMap, ok := config.(map[string]interface{}); ok {
					if err := (*model.Theme)(theme).SetConfig(configMap); err != nil {
						return err
					}
				}
	}

	if err := service.S.Theme.UpdateTheme(theme); err != nil {
		return err
	}
	return c.JSON(&theme)
}

// DeleteTheme 删除主题
func DeleteTheme(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return errors.ErrInvalidParams
	}
	if err := service.S.Theme.DeleteTheme(id); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "删除成功"})
}

// ActivateTheme 激活主题
func ActivateTheme(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return errors.ErrInvalidParams
	}
	if err := service.S.Theme.ActivateTheme(id); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "激活成功"})
}

// GetActiveTheme 获取当前激活的主题
func GetActiveTheme(c *fiber.Ctx) error {
	theme, err := service.S.Theme.GetActiveTheme()
	if err != nil {
		return err
	}

	// 转换为 model.Theme 类型以使用 GetConfig 方法
	modelTheme := (*model.Theme)(theme)
	config, err := modelTheme.GetConfig()
	if err == nil {
		// 创建一个新的响应对象，包含解析后的配置
		respTheme := map[string]interface{}{
			"id":           theme.ID,
			"name":         theme.Name,
			"display_name": theme.DisplayName,
			"author":       theme.Author,
			"version":      theme.Version,
			"description":  theme.Description,
			"preview":      theme.Preview,
			"css":          theme.CSS,
			"js":           theme.JS,
			"is_default":   theme.IsDefault,
			"is_active":    theme.IsActive,
			"config":       config,
			"created_at":   theme.CreatedAt,
			"updated_at":   theme.UpdatedAt,
		}
		return c.JSON(respTheme)
	}

	return c.JSON(&theme)
}

// UpdateThemeConfig 更新主题配置
func UpdateThemeConfig(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return errors.ErrInvalidParams
	}
	theme, err := service.S.Theme.GetThemeByID(id)
	if err != nil {
		return err
	}

	var configData map[string]interface{}
	if err := c.BodyParser(&configData); err != nil {
		return err
	}

	// 获取当前配置
	currentConfig, err := (*model.Theme)(theme).GetConfig()
	if err != nil {
		return err
	}

	// 合并新配置
	for k, v := range configData {
		currentConfig[k] = v
	}

	// 设置更新后的配置
	if err := (*model.Theme)(theme).SetConfig(currentConfig); err != nil {
		return err
	}

	if err := service.S.Theme.UpdateTheme(theme); err != nil {
		return err
	}
	return c.JSON(&theme)
}
