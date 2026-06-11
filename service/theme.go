package service

import (
	"archive/zip"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"myecho/config"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/model"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ThemeService struct{}

const (
	ThemeStorageDir              = "storage/themes"
	themeStorageDir              = ThemeStorageDir
	MaxThemePackageBytes   int64 = 20 << 20
	maxThemeExtractedBytes       = 50 << 20
	maxThemePackageFiles         = 500
	ThemePreviewCookieName       = "myecho_theme_preview"
	ThemePreviewTokenTTL         = 15 * time.Minute
	themePreviewSecretKey        = "ThemePreviewTokenSecret"
)

var themeNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

type ThemeManifest struct {
	SchemaVersion int                      `json:"schema_version"`
	Name          string                   `json:"name"`
	DisplayName   string                   `json:"display_name"`
	Author        string                   `json:"author"`
	Version       string                   `json:"version"`
	Description   string                   `json:"description"`
	Preview       string                   `json:"preview"`
	CSS           string                   `json:"css"`
	JS            string                   `json:"js"`
	Config        map[string]interface{}   `json:"config"`
	ConfigSchema  []map[string]interface{} `json:"config_schema"`
}

type ThemePreviewPayload struct {
	ThemeID uint  `json:"theme_id"`
	Expires int64 `json:"expires"`
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
	theme, err := s.GetThemeByID(id)
	if err != nil {
		return err
	}
	if err := dal.MySqlDB.Theme.Delete(id); err != nil {
		return err
	}
	if !theme.IsDefault && theme.Name != "" {
		return os.RemoveAll(filepath.Join(themeStorageDir, theme.Name))
	}
	return nil
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
	if err := validateThemePackage(zipPath, manifestDir); err != nil {
		return nil, err
	}

	existing, err := s.GetThemeByName(manifest.Name)
	if err != nil && err != mysql.ErrThemeNotExist {
		return nil, err
	}
	if existing != nil && existing.IsDefault {
		return nil, fmt.Errorf("default theme cannot be overwritten by package upload")
	}

	if err := os.MkdirAll(themeStorageDir, 0755); err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp(themeStorageDir, "."+manifest.Name+"-*")
	if err != nil {
		return nil, err
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(tempDir)
		}
	}()

	if err := extractThemePackage(zipPath, manifestDir, tempDir); err != nil {
		return nil, err
	}
	hasTemplates := packageHasTemplates(tempDir)
	applyThemeConfigDefaults(manifest)

	theme := &mysql.ThemeModel{
		Name:         manifest.Name,
		DisplayName:  manifest.DisplayName,
		Author:       manifest.Author,
		Version:      manifest.Version,
		Description:  manifest.Description,
		Preview:      themeAssetURL(manifest.Name, manifest.Preview),
		CSS:          themeCSS(manifest.Name, manifest.CSS),
		JS:           themeJS(tempDir, manifest.JS),
		IsDefault:    false,
		IsActive:     false,
		HasTemplates: hasTemplates,
	}
	if err := (*model.Theme)(theme).SetConfig(manifest.Config); err != nil {
		return nil, err
	}
	if err := (*model.Theme)(theme).SetConfigSchema(manifest.ConfigSchema); err != nil {
		return nil, err
	}

	themeDir := filepath.Join(themeStorageDir, manifest.Name)
	backupDir := ""
	if _, err := os.Stat(themeDir); err == nil {
		backupDir = filepath.Join(themeStorageDir, fmt.Sprintf(".%s-backup-%d", manifest.Name, time.Now().UnixNano()))
		if err := os.Rename(themeDir, backupDir); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	restoreThemeDir := func() {
		_ = os.RemoveAll(themeDir)
		if backupDir != "" {
			_ = os.Rename(backupDir, themeDir)
		}
	}
	if err := os.Rename(tempDir, themeDir); err != nil {
		restoreThemeDir()
		return nil, err
	}
	cleanupTemp = false

	if existing != nil {
		theme.ID = existing.ID
		theme.CreatedAt = existing.CreatedAt
		theme.IsDefault = existing.IsDefault
		theme.IsActive = existing.IsActive
		if err := s.UpdateTheme(theme); err != nil {
			restoreThemeDir()
			return nil, err
		}
		_ = os.RemoveAll(backupDir)
		return theme, nil
	}
	if err := s.CreateTheme(theme); err != nil {
		restoreThemeDir()
		return nil, err
	}
	_ = os.RemoveAll(backupDir)
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
	if manifest.ConfigSchema == nil {
		manifest.ConfigSchema = []map[string]interface{}{}
	}
	for _, field := range manifest.ConfigSchema {
		key, _ := field["key"].(string)
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("theme config_schema field key is required")
		}
	}
	return nil
}

func applyThemeConfigDefaults(manifest *ThemeManifest) {
	if manifest.Config == nil {
		manifest.Config = make(map[string]interface{})
	}
	for _, field := range manifest.ConfigSchema {
		key, _ := field["key"].(string)
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := manifest.Config[key]; exists {
			continue
		}
		if defaultValue, ok := field["default"]; ok {
			manifest.Config[key] = defaultValue
		}
	}
}

func validateThemePackage(zipPath, manifestDir string) error {
	stat, err := os.Stat(zipPath)
	if err != nil {
		return err
	}
	if stat.Size() > MaxThemePackageBytes {
		return fmt.Errorf("theme package is too large")
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	var fileCount int
	var totalSize uint64
	for _, file := range reader.File {
		cleanName, ok := cleanZipName(file.Name)
		if !ok {
			return fmt.Errorf("invalid file path in theme package: %s", file.Name)
		}
		if !isUnderZipDir(cleanName, manifestDir) {
			continue
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed in theme packages")
		}
		fileCount++
		if fileCount > maxThemePackageFiles {
			return fmt.Errorf("theme package contains too many files")
		}
		totalSize += file.UncompressedSize64
		if totalSize > maxThemeExtractedBytes {
			return fmt.Errorf("theme package extracted size is too large")
		}
		relName := strings.TrimPrefix(cleanName, manifestDir)
		relName = strings.TrimPrefix(relName, "/")
		if !isAllowedThemeFile(relName) {
			return fmt.Errorf("theme package contains unsupported file type: %s", relName)
		}
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
		if !isAllowedThemeFile(relName) {
			return fmt.Errorf("theme package contains unsupported file type: %s", relName)
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

	perm := file.FileInfo().Mode().Perm()
	if perm == 0 {
		perm = 0644
	}
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
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
	return ThemeAssetBaseURL(themeName) + cleanPath
}

func ThemeAssetBaseURL(themeName string) string {
	themeName = strings.TrimSpace(themeName)
	if themeName == "" || !themeNamePattern.MatchString(themeName) {
		return ""
	}
	return "/themes/" + themeName + "/"
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

func isAllowedThemeFile(relName string) bool {
	relName = strings.TrimSpace(strings.ReplaceAll(relName, "\\", "/"))
	if relName == "" {
		return false
	}
	if path.Clean(relName) == "theme.json" {
		return true
	}
	if strings.HasSuffix(relName, ".jet.html") {
		return true
	}
	switch strings.ToLower(path.Ext(relName)) {
	case ".css", ".js", ".json", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".otf", ".eot", ".txt", ".md":
		return true
	default:
		return false
	}
}

func packageHasTemplates(themeDir string) bool {
	templateRoot := filepath.Join(themeDir, "templates")
	found := false
	_ = filepath.WalkDir(templateRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".jet.html") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func (s *ThemeService) CreatePreviewToken(id int64, ttl time.Duration) (string, time.Time, error) {
	theme, err := s.GetThemeByID(id)
	if err != nil {
		return "", time.Time{}, err
	}
	if ttl <= 0 {
		ttl = ThemePreviewTokenTTL
	}
	expiresAt := time.Now().Add(ttl)
	payload := ThemePreviewPayload{
		ThemeID: theme.ID,
		Expires: expiresAt.Unix(),
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", time.Time{}, err
	}
	secret, err := s.getPreviewSecret()
	if err != nil {
		return "", time.Time{}, err
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(payloadJSON)
	sig := signPreviewPayload([]byte(payloadPart), []byte(secret))
	return payloadPart + "." + sig, expiresAt, nil
}

func (s *ThemeService) ValidatePreviewToken(token string) (*mysql.ThemeModel, error) {
	payload, err := s.parsePreviewToken(token)
	if err != nil {
		return nil, err
	}
	if payload.Expires < time.Now().Unix() {
		return nil, fmt.Errorf("theme preview token expired")
	}
	return s.GetThemeByID(int64(payload.ThemeID))
}

func (s *ThemeService) parsePreviewToken(token string) (*ThemePreviewPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid theme preview token")
	}
	secret, err := s.getPreviewSecret()
	if err != nil {
		return nil, err
	}
	expected := signPreviewPayload([]byte(parts[0]), []byte(secret))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return nil, fmt.Errorf("invalid theme preview token")
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var payload ThemePreviewPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, err
	}
	if payload.ThemeID == 0 || payload.Expires == 0 {
		return nil, fmt.Errorf("invalid theme preview token")
	}
	return &payload, nil
}

func signPreviewPayload(payload, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *ThemeService) getPreviewSecret() (string, error) {
	setting, err := dal.MySqlDB.Setting.GetByKey(themePreviewSecretKey)
	if err == nil && setting.Value != "" {
		return setting.Value, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	secret, err := randomSecret()
	if err != nil {
		return "", err
	}
	setting = mysql.SettingModel{
		Key:         themePreviewSecretKey,
		Value:       secret,
		Type:        model.SettingModelTypeString,
		Description: "Theme preview token signing secret",
		IsSystem:    true,
	}
	if err := dal.MySqlDB.Setting.Create(&setting); err != nil {
		latest, latestErr := dal.MySqlDB.Setting.GetByKey(themePreviewSecretKey)
		if latestErr == nil && latest.Value != "" {
			return latest.Value, nil
		}
		return "", err
	}
	if config.MySqlSettingModelCache != nil {
		config.MySqlSettingModelCache.Set(setting.Key, &setting)
	}
	return secret, nil
}

func randomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func IsHiddenSettingKey(key string) bool {
	return key == themePreviewSecretKey
}

func SafePreviewPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.ContainsRune(value, 0) {
		return "/"
	}
	return value
}

func PreviewURL(token, targetPath string) string {
	return "/theme-preview?token=" + url.QueryEscape(token) + "&path=" + url.QueryEscape(SafePreviewPath(targetPath))
}

func ParsePreviewID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid theme id")
	}
	return id, nil
}
