package service

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/model"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type ThemeService struct{}

const themeStorageDir = "storage/themes"

var themeNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

type ThemeManifest struct {
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Author      string                 `json:"author"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Preview     string                 `json:"preview"`
	CSS         string                 `json:"css"`
	JS          string                 `json:"js"`
	Config      map[string]interface{} `json:"config"`
}

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

// InstallThemePackage installs a zip theme package and creates or updates the theme record.
func (s *ThemeService) InstallThemePackage(zipPath string) (*mysql.ThemeModel, error) {
	manifest, manifestDir, err := readThemeManifest(zipPath)
	if err != nil {
		return nil, err
	}
	if err := validateThemeManifest(manifest); err != nil {
		return nil, err
	}

	existing, err := s.GetThemeByName(manifest.Name)
	if err != nil && err != mysql.ErrThemeNotExist {
		return nil, err
	}
	if existing != nil && existing.IsDefault {
		return nil, fmt.Errorf("default theme cannot be overwritten by package upload")
	}

	themeDir := filepath.Join(themeStorageDir, manifest.Name)
	if err := os.RemoveAll(themeDir); err != nil {
		return nil, err
	}
	if err := extractThemePackage(zipPath, manifestDir, themeDir); err != nil {
		return nil, err
	}

	theme := &mysql.ThemeModel{
		Name:        manifest.Name,
		DisplayName: manifest.DisplayName,
		Author:      manifest.Author,
		Version:     manifest.Version,
		Description: manifest.Description,
		Preview:     themeAssetURL(manifest.Name, manifest.Preview),
		CSS:         themeCSS(manifest.Name, manifest.CSS),
		JS:          themeJS(themeDir, manifest.JS),
		IsDefault:   false,
		IsActive:    false,
	}
	if err := (*model.Theme)(theme).SetConfig(manifest.Config); err != nil {
		return nil, err
	}

	if existing != nil {
		theme.ID = existing.ID
		theme.CreatedAt = existing.CreatedAt
		theme.IsDefault = existing.IsDefault
		theme.IsActive = existing.IsActive
		if err := s.UpdateTheme(theme); err != nil {
			return nil, err
		}
		return theme, nil
	}
	if err := s.CreateTheme(theme); err != nil {
		return nil, err
	}
	return theme, nil
}

func readThemeManifest(zipPath string) (*ThemeManifest, string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	for _, file := range reader.File {
		cleanName, ok := cleanZipName(file.Name)
		if !ok || path.Base(cleanName) != "theme.json" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()

		var manifest ThemeManifest
		if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
			return nil, "", err
		}
		return &manifest, path.Dir(cleanName), nil
	}
	return nil, "", fmt.Errorf("theme.json not found in package")
}

func validateThemeManifest(manifest *ThemeManifest) error {
	if manifest == nil {
		return fmt.Errorf("theme manifest is empty")
	}
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.DisplayName = strings.TrimSpace(manifest.DisplayName)
	if manifest.Name == "" {
		return fmt.Errorf("theme name is required")
	}
	if !themeNamePattern.MatchString(manifest.Name) {
		return fmt.Errorf("theme name can only contain lowercase letters, numbers, hyphens and underscores")
	}
	if manifest.DisplayName == "" {
		manifest.DisplayName = manifest.Name
	}
	if manifest.Version == "" {
		manifest.Version = "1.0.0"
	}
	if manifest.Config == nil {
		manifest.Config = make(map[string]interface{})
	}
	return nil
}

func extractThemePackage(zipPath, manifestDir, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	destRoot, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destRoot, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		cleanName, ok := cleanZipName(file.Name)
		if !ok || !isUnderZipDir(cleanName, manifestDir) {
			continue
		}
		relName := strings.TrimPrefix(cleanName, manifestDir)
		relName = strings.TrimPrefix(relName, "/")
		if relName == "" {
			continue
		}

		target := filepath.Join(destRoot, filepath.FromSlash(relName))
		if !isUnderDir(destRoot, target) {
			return fmt.Errorf("invalid file path in theme package: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := unzipFile(file, target); err != nil {
			return err
		}
	}
	return nil
}

func unzipFile(file *zip.File, target string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func cleanZipName(name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	cleanName := path.Clean(name)
	if cleanName == "." || strings.HasPrefix(cleanName, "../") || strings.HasPrefix(cleanName, "/") {
		return "", false
	}
	return cleanName, true
}

func isUnderZipDir(name, dir string) bool {
	if dir == "." {
		return true
	}
	return name == dir || strings.HasPrefix(name, dir+"/")
}

func isUnderDir(root, target string) bool {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, absTarget)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func themeAssetURL(themeName, assetPath string) string {
	cleanPath, ok := cleanThemeAssetPath(assetPath)
	if !ok {
		return ""
	}
	return "/themes/" + themeName + "/" + cleanPath
}

func themeCSS(themeName, cssPath string) string {
	cssURL := themeAssetURL(themeName, cssPath)
	if cssURL == "" {
		return ""
	}
	return fmt.Sprintf("@import url(%q);", cssURL)
}

func themeJS(themeDir, jsPath string) string {
	cleanPath, ok := cleanThemeAssetPath(jsPath)
	if !ok {
		return ""
	}
	target := filepath.Join(themeDir, filepath.FromSlash(cleanPath))
	content, err := os.ReadFile(target)
	if err != nil {
		return ""
	}
	return string(content)
}

func cleanThemeAssetPath(assetPath string) (string, bool) {
	assetPath = strings.TrimSpace(strings.ReplaceAll(assetPath, "\\", "/"))
	if assetPath == "" {
		return "", false
	}
	cleanPath := path.Clean(assetPath)
	if cleanPath == "." || strings.HasPrefix(cleanPath, "../") || strings.HasPrefix(cleanPath, "/") {
		return "", false
	}
	return cleanPath, true
}
