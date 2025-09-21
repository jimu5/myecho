package mysql

import (
	"errors"
	"myecho/model"

	"gorm.io/gorm"
)

var (
	ErrThemeNotExist       = errors.New("主题不存在")
	ErrThemeActiveExists   = errors.New("已有激活的主题")
	ErrThemeCantDeleteActive = errors.New("无法删除激活状态的主题")
	ErrThemeCantDeleteDefault = errors.New("无法删除默认主题")
)

type ThemeRepo struct {
}

type ThemeModel model.Theme

func (ThemeModel) TableName() string {
	return "themes"
}

// Create 创建主题
func (s *ThemeRepo) Create(theme *ThemeModel) error {
	return db.Create(theme).Error
}

// GetAll 获取所有主题
func (s *ThemeRepo) GetAll() ([]*ThemeModel, error) {
	var themes []*ThemeModel
	err := db.Find(&themes).Error
	return themes, err
}

// GetByID 根据ID获取主题
func (s *ThemeRepo) GetByID(id int64) (*ThemeModel, error) {
	var theme ThemeModel
	err := db.Where("id = ?", id).First(&theme).Error
	if err == gorm.ErrRecordNotFound {
		return nil, ErrThemeNotExist
	}
	return &theme, err
}

// GetByName 根据名称获取主题
func (s *ThemeRepo) GetByName(name string) (*ThemeModel, error) {
	var theme ThemeModel
	err := db.Where("name = ?", name).First(&theme).Error
	if err == gorm.ErrRecordNotFound {
		return nil, ErrThemeNotExist
	}
	return &theme, err
}

// GetActiveTheme 获取当前激活的主题
func (s *ThemeRepo) GetActiveTheme() (*ThemeModel, error) {
	var theme ThemeModel
	err := db.Where("is_active = ?", true).First(&theme).Error
	if err == gorm.ErrRecordNotFound {
		return nil, ErrThemeNotExist
	}
	return &theme, err
}

// Update 更新主题
func (s *ThemeRepo) Update(theme *ThemeModel) error {
	return db.Save(theme).Error
}

// Delete 删除主题
func (s *ThemeRepo) Delete(id int64) error {
	theme, err := s.GetByID(id)
	if err != nil {
		return err
	}
	if theme.IsActive {
		return ErrThemeCantDeleteActive
	}
	if theme.IsDefault {
		return ErrThemeCantDeleteDefault
	}
	return db.Where("id = ?", id).Delete(&ThemeModel{}).Error
}

// ActivateTheme 激活主题
func (s *ThemeRepo) ActivateTheme(id int64) error {
	// 先禁用当前激活的主题
	err := db.Model(&ThemeModel{}).Where("is_active = ?", true).Update("is_active", false).Error
	if err != nil {
		return err
	}
	// 激活指定主题
	return db.Model(&ThemeModel{}).Where("id = ?", id).Update("is_active", true).Error
}

// InitDefaultTheme 初始化默认主题
func (s *ThemeRepo) InitDefaultTheme() error {
	// 检查是否已经有主题
	var count int64
	db.Model(&ThemeModel{}).Count(&count)
	if count > 0 {
		return nil
	}

	// 创建默认主题
	defaultTheme := &ThemeModel{
		Name:        "default",
		DisplayName: "默认主题",
		Author:      "Myecho",
		Version:     "1.0.0",
		Description: "博客系统默认主题",
		Preview:     "",
		CSS:         ":root {\n  --font-color: #1a1a1a;\n  --bg-color: #ffffff;\n  --primary-color: #000000;\n  --secondary-color: #e9e9e9;\n}",
		JS:          "",
		IsDefault:   true,
		IsActive:    true,
	}

	// 设置默认配置 - 需要转换为 model.Theme 类型以调用方法
	if err := (*model.Theme)(defaultTheme).SetConfig(make(map[string]interface{})); err != nil {
		return err
	}

	return s.Create(defaultTheme)
}