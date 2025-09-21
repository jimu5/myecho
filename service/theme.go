package service

import (
	"myecho/dal"
	"myecho/dal/mysql"
)

type ThemeService struct{}

// CreateTheme 创建主题
func (s *ThemeService) CreateTheme(theme *mysql.ThemeModel) error {
	return dal.MySqlDB.Theme.Create(theme)
}

// GetAllThemes 获取所有主题
func (s *ThemeService) GetAllThemes() ([]*mysql.ThemeModel, error) {
	return dal.MySqlDB.Theme.GetAll()
}

// GetThemeByID 根据ID获取主题
func (s *ThemeService) GetThemeByID(id int64) (*mysql.ThemeModel, error) {
	return dal.MySqlDB.Theme.GetByID(id)
}

// GetThemeByName 根据名称获取主题
func (s *ThemeService) GetThemeByName(name string) (*mysql.ThemeModel, error) {
	return dal.MySqlDB.Theme.GetByName(name)
}

// GetActiveTheme 获取当前激活的主题
func (s *ThemeService) GetActiveTheme() (*mysql.ThemeModel, error) {
	return dal.MySqlDB.Theme.GetActiveTheme()
}

// UpdateTheme 更新主题
func (s *ThemeService) UpdateTheme(theme *mysql.ThemeModel) error {
	return dal.MySqlDB.Theme.Update(theme)
}

// DeleteTheme 删除主题
func (s *ThemeService) DeleteTheme(id int64) error {
	return dal.MySqlDB.Theme.Delete(id)
}

// ActivateTheme 激活主题
func (s *ThemeService) ActivateTheme(id int64) error {
	return dal.MySqlDB.Theme.ActivateTheme(id)
}

// InitDefaultTheme 初始化默认主题
func (s *ThemeService) InitDefaultTheme() error {
	return dal.MySqlDB.Theme.InitDefaultTheme()
}
