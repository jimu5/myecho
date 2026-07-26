package service

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/model"
)

func TestReadThemeManifestFromPackageRoot(t *testing.T) {
	zipPath := writeThemeZip(t, map[string]string{
		"myecho/theme.json": `{
			"name": "clean_theme",
			"display_name": "Clean Theme",
			"author": "Myecho",
			"version": "1.2.0",
			"description": "A test theme",
			"css": "style.css",
			"js": "script.js",
			"preview": "preview.png"
		}`,
		"myecho/style.css":   "body { color: #111; }",
		"myecho/script.js":   "window.__themeLoaded = true;",
		"myecho/preview.png": "fake-image",
	})

	manifest, manifestDir, err := readThemeManifest(zipPath)
	if err != nil {
		t.Fatalf("readThemeManifest() error = %v", err)
	}
	if manifestDir != "myecho" {
		t.Fatalf("manifestDir = %q, want %q", manifestDir, "myecho")
	}
	if manifest.Name != "clean_theme" || manifest.DisplayName != "Clean Theme" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
}

func TestExtractThemePackageStripsManifestDirectory(t *testing.T) {
	zipPath := writeThemeZip(t, map[string]string{
		"theme-root/theme.json":      `{"name":"clean_theme","display_name":"Clean Theme"}`,
		"theme-root/style.css":       "body { color: #111; }",
		"theme-root/assets/icon.svg": "<svg></svg>",
		"other-root/ignored.txt":     "ignored",
	})
	destDir := t.TempDir()

	if err := extractThemePackage(zipPath, "theme-root", destDir); err != nil {
		t.Fatalf("extractThemePackage() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "style.css")); err != nil {
		t.Fatalf("style.css was not extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "assets", "icon.svg")); err != nil {
		t.Fatalf("nested asset was not extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "ignored.txt")); !os.IsNotExist(err) {
		t.Fatalf("file outside manifest directory should not be extracted")
	}
}

func TestValidateThemeManifestRejectsUnsafeName(t *testing.T) {
	err := validateThemeManifest(&ThemeManifest{Name: "../bad", DisplayName: "Bad"})
	if err == nil {
		t.Fatal("validateThemeManifest() expected an error")
	}
}

func TestThemeAssetURLRejectsTraversal(t *testing.T) {
	if got := themeAssetURL("clean_theme", "../secret.css"); got != "" {
		t.Fatalf("themeAssetURL() = %q, want empty string", got)
	}
	if got := themeAssetURL("clean_theme", "assets/main.css"); got != "/themes/clean_theme/assets/main.css" {
		t.Fatalf("themeAssetURL() = %q, want valid asset URL", got)
	}
}

func TestInitPresetThemesIsIdempotentAndPreservesActiveTheme(t *testing.T) {
	setupServiceTestDB(t)
	svc := &ThemeService{}
	active := &mysql.ThemeModel{Name: "custom", DisplayName: "Custom", IsActive: true}
	if err := svc.CreateTheme(active); err != nil {
		t.Fatalf("CreateTheme(active) error = %v", err)
	}

	if err := svc.InitPresetThemes(); err != nil {
		t.Fatalf("InitPresetThemes() error = %v", err)
	}
	if err := svc.InitPresetThemes(); err != nil {
		t.Fatalf("InitPresetThemes(second) error = %v", err)
	}

	themes, err := svc.GetAllThemes()
	if err != nil {
		t.Fatalf("GetAllThemes() error = %v", err)
	}
	if len(themes) != 4 {
		t.Fatalf("theme count = %d, want custom plus three presets", len(themes))
	}
	presetCSS := map[string]string{
		"paper": `@import url("/static/css/presets/paper.css");`,
		"night": `@import url("/static/css/presets/night.css");`,
		"anime": `@import url("/static/css/presets/anime.css");`,
	}
	for name, wantCSS := range presetCSS {
		preset, err := svc.GetThemeByName(name)
		if err != nil {
			t.Fatalf("GetThemeByName(%s) error = %v", name, err)
		}
		config, err := (*model.Theme)(preset).GetConfig()
		if err != nil || config["bundled"] != true || config["supports_color_mode"] != true || preset.CSS != wantCSS {
			t.Fatalf("preset %s config=%+v css=%q err=%v", name, config, preset.CSS, err)
		}
	}
	gotActive, err := svc.GetActiveTheme()
	if err != nil || gotActive.ID != active.ID {
		t.Fatalf("active theme = %+v, err = %v", gotActive, err)
	}
}

func TestInitPresetThemesPreservesExistingTheme(t *testing.T) {
	setupServiceTestDB(t)
	svc := &ThemeService{}
	existing := &mysql.ThemeModel{Name: "anime", DisplayName: "Custom Anime", CSS: ".custom-anime {}"}
	if err := svc.CreateTheme(existing); err != nil {
		t.Fatalf("CreateTheme(existing) error = %v", err)
	}

	if err := svc.InitPresetThemes(); err != nil {
		t.Fatalf("InitPresetThemes() error = %v", err)
	}
	got, err := svc.GetThemeByName("anime")
	if err != nil {
		t.Fatalf("GetThemeByName(anime) error = %v", err)
	}
	if got.ID != existing.ID || got.DisplayName != existing.DisplayName || got.CSS != existing.CSS {
		t.Fatalf("existing theme was overwritten: %+v", got)
	}
}

func TestInitPresetThemesMarksLegacyBundledTheme(t *testing.T) {
	setupServiceTestDB(t)
	svc := &ThemeService{}
	legacy := &mysql.ThemeModel{
		Name:        "paper",
		DisplayName: "Legacy Paper",
		Author:      "Myecho",
		CSS:         `@import url("/static/css/presets/paper.css");`,
	}
	if err := (*model.Theme)(legacy).SetConfig(map[string]interface{}{"supports_color_mode": true}); err != nil {
		t.Fatalf("SetConfig(legacy) error = %v", err)
	}
	if err := svc.CreateTheme(legacy); err != nil {
		t.Fatalf("CreateTheme(legacy) error = %v", err)
	}

	if err := svc.InitPresetThemes(); err != nil {
		t.Fatalf("InitPresetThemes() error = %v", err)
	}
	got, err := svc.GetThemeByName("paper")
	if err != nil {
		t.Fatalf("GetThemeByName(paper) error = %v", err)
	}
	if got.DisplayName != legacy.DisplayName || !IsBundledTheme(got) {
		t.Fatalf("legacy preset was not marked in place: %+v", got)
	}
}

func TestBundledFlagIsServerOwned(t *testing.T) {
	setupServiceTestDB(t)
	svc := &ThemeService{}
	theme := &mysql.ThemeModel{
		Name:        "anime",
		DisplayName: "Custom Anime",
		CSS:         `@import url("/static/css/presets/anime.css");`,
	}
	if err := (*model.Theme)(theme).SetConfig(map[string]interface{}{"bundled": true}); err != nil {
		t.Fatalf("SetConfig(theme) error = %v", err)
	}
	if err := svc.CreateTheme(theme); err != nil {
		t.Fatalf("CreateTheme(theme) error = %v", err)
	}
	if IsBundledTheme(theme) {
		t.Fatal("CreateTheme accepted the internal bundled flag")
	}
}

func TestBundledThemeCanBeCustomizedAndOverwrittenButCannotBeDeleted(t *testing.T) {
	chdirServiceTestTemp(t)
	setupServiceTestDB(t)
	svc := &ThemeService{}
	if err := svc.InitPresetThemes(); err != nil {
		t.Fatalf("InitPresetThemes() error = %v", err)
	}
	anime, err := svc.GetThemeByName("anime")
	if err != nil {
		t.Fatalf("GetThemeByName(anime) error = %v", err)
	}

	anime.CSS += "\nbody { --accent: hotpink; }"
	anime.JS = "window.customized = true;"
	if err := (*model.Theme)(anime).SetConfig(map[string]interface{}{"accent": "hotpink"}); err != nil {
		t.Fatalf("SetConfig(bundled) error = %v", err)
	}
	if err := svc.UpdateTheme(anime); err != nil {
		t.Fatalf("UpdateTheme(bundled) error = %v", err)
	}
	updated, err := svc.GetThemeByName("anime")
	if err != nil {
		t.Fatalf("GetThemeByName(anime) error = %v", err)
	}
	config, err := (*model.Theme)(updated).GetConfig()
	if err != nil || config["accent"] != "hotpink" || config["bundled"] != true || !IsBundledTheme(updated) {
		t.Fatalf("updated bundled theme = %+v config=%+v err=%v", updated, config, err)
	}
	overrideZip := writeThemeZip(t, map[string]string{
		"theme/theme.json":             `{"name":"anime","display_name":"Override","config":{"accent":"purple"}}`,
		"theme/templates/404.jet.html": "custom not found",
	})
	overridden, err := svc.InstallThemePackage(overrideZip)
	if err != nil {
		t.Fatalf("InstallThemePackage(bundled) error = %v", err)
	}
	config, err = (*model.Theme)(overridden).GetConfig()
	if err != nil || overridden.ID != anime.ID || overridden.DisplayName != "Override" || !overridden.HasTemplates || config["bundled"] != true {
		t.Fatalf("overridden bundled theme = %+v config=%+v err=%v", overridden, config, err)
	}
	if err := svc.DeleteTheme(int64(anime.ID)); !errors.Is(err, ErrBundledThemeImmutable) {
		t.Fatalf("DeleteTheme(bundled) error = %v, want ErrBundledThemeImmutable", err)
	}
}

func TestDefaultThemeCanBeOverwrittenByPackage(t *testing.T) {
	chdirServiceTestTemp(t)
	setupServiceTestDB(t)
	svc := &ThemeService{}
	defaultTheme := &mysql.ThemeModel{
		Name:        "default",
		DisplayName: "Default",
		IsDefault:   true,
		IsActive:    true,
	}
	if err := svc.CreateTheme(defaultTheme); err != nil {
		t.Fatalf("CreateTheme(default) error = %v", err)
	}

	zipPath := writeThemeZip(t, map[string]string{
		"theme/theme.json":                 `{"name":"default","display_name":"Customized Default"}`,
		"theme/templates/article.jet.html": "custom article",
	})
	overridden, err := svc.InstallThemePackage(zipPath)
	if err != nil {
		t.Fatalf("InstallThemePackage(default) error = %v", err)
	}
	if overridden.ID != defaultTheme.ID || !overridden.IsDefault || !overridden.IsActive || !overridden.HasTemplates {
		t.Fatalf("overridden default theme = %+v", overridden)
	}
}

func TestInstallThemePackageCreatesAndUpdatesTheme(t *testing.T) {
	setupServiceTestDB(t)
	t.Cleanup(func() { _ = os.RemoveAll(themeStorageDir) })
	zipPath := writeThemeZip(t, map[string]string{
		"theme/theme.json": `{
			"name": "upload_theme",
			"display_name": "Upload Theme",
			"author": "Myecho",
			"version": "1.0.0",
			"description": "A theme",
			"css": "style.css",
			"js": "script.js",
			"preview": "preview.png",
			"config": {"color": "blue"},
			"config_schema": [{"key":"font","default":"serif"}]
		}`,
		"theme/style.css":                "body { color: blue; }",
		"theme/script.js":                "window.theme = true;",
		"theme/preview.png":              "preview",
		"theme/templates/index.jet.html": "theme template",
	})

	theme, err := (&ThemeService{}).InstallThemePackage(zipPath)
	if err != nil {
		t.Fatalf("InstallThemePackage() error = %v", err)
	}
	if theme.Name != "upload_theme" || theme.DisplayName != "Upload Theme" {
		t.Fatalf("theme = %+v", theme)
	}
	if theme.CSS == "" || theme.JS != "window.theme = true;" || theme.Preview != "/themes/upload_theme/preview.png" || !theme.HasTemplates {
		t.Fatalf("unexpected theme assets: %+v", theme)
	}
	config, err := (*model.Theme)(theme).GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}
	if config["color"] != "blue" || config["font"] != "serif" {
		t.Fatalf("theme config defaults = %+v", config)
	}
	if _, err := os.Stat(filepath.Join(themeStorageDir, "upload_theme", "style.css")); err != nil {
		t.Fatalf("theme asset not extracted: %v", err)
	}

	updatedZip := writeThemeZip(t, map[string]string{
		"theme/theme.json": `{"name":"upload_theme","display_name":"Updated","version":"2.0.0","js":"script.js"}`,
		"theme/script.js":  "window.theme = 'updated';",
	})
	updated, err := (&ThemeService{}).InstallThemePackage(updatedZip)
	if err != nil {
		t.Fatalf("second InstallThemePackage() error = %v", err)
	}
	if updated.ID != theme.ID || updated.DisplayName != "Updated" || updated.JS != "window.theme = 'updated';" {
		t.Fatalf("updated theme = %+v, original id %d", updated, theme.ID)
	}
}

func TestThemeHelpersRejectInvalidInputs(t *testing.T) {
	if _, _, err := readThemeManifest(filepath.Join(t.TempDir(), "missing.zip")); err == nil {
		t.Fatal("readThemeManifest() expected missing file error")
	}
	if err := validateThemeManifest(nil); err == nil {
		t.Fatal("validateThemeManifest(nil) expected error")
	}
	manifest := &ThemeManifest{Name: "valid_name"}
	if err := validateThemeManifest(manifest); err != nil {
		t.Fatalf("validateThemeManifest() error = %v", err)
	}
	if manifest.DisplayName != "valid_name" || manifest.Version != "1.0.0" || manifest.Config == nil {
		t.Fatalf("manifest defaults not applied: %+v", manifest)
	}
	if _, ok := cleanZipName("../bad"); ok {
		t.Fatal("cleanZipName() should reject traversal")
	}
	if isUnderZipDir("other/file", "theme") {
		t.Fatal("isUnderZipDir() accepted outside path")
	}
	if isUnderDir(t.TempDir(), filepath.Join(t.TempDir(), "..", "outside")) {
		t.Fatal("isUnderDir() accepted outside target")
	}
	if got := themeCSS("theme", ""); got != "" {
		t.Fatalf("themeCSS(empty) = %q, want empty", got)
	}
	if got := themeJS(t.TempDir(), "../bad.js"); got != "" {
		t.Fatalf("themeJS(traversal) = %q, want empty", got)
	}
	if _, err := ValidateThemeName("../bad"); err == nil {
		t.Fatal("ValidateThemeName(traversal) expected error")
	}
	if _, err := themeStoragePath(".."); err == nil {
		t.Fatal("themeStoragePath(traversal) expected error")
	}

	largeManifest := writeThemeZip(t, map[string]string{
		"theme/theme.json": `{"name":"large","description":"` + strings.Repeat("x", maxThemeManifestBytes) + `"}`,
	})
	if _, _, err := readThemeManifest(largeManifest); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("readThemeManifest(large) error = %v, want size error", err)
	}
}

func TestThemePackageValidationAndTemplateHelpers(t *testing.T) {
	manifest := &ThemeManifest{
		Name: "schema_theme",
		Config: map[string]interface{}{
			"accent": "blue",
		},
		ConfigSchema: []map[string]interface{}{
			{"key": "accent", "default": "red"},
			{"key": "font", "default": "serif"},
		},
	}
	if err := validateThemeManifest(manifest); err != nil {
		t.Fatalf("validateThemeManifest() error = %v", err)
	}
	applyThemeConfigDefaults(manifest)
	if manifest.Config["accent"] != "blue" || manifest.Config["font"] != "serif" {
		t.Fatalf("applyThemeConfigDefaults() = %+v", manifest.Config)
	}
	if err := validateThemeManifest(&ThemeManifest{Name: "bad_schema", ConfigSchema: []map[string]interface{}{{"label": "Missing key"}}}); err == nil {
		t.Fatal("validateThemeManifest() expected config_schema key error")
	}

	unsupportedZip := writeThemeZip(t, map[string]string{
		"theme/theme.json": `{"name":"bad_file"}`,
		"theme/shell.php":  "<?php echo 'nope';",
	})
	if err := validateThemePackage(unsupportedZip, "theme"); err == nil || !strings.Contains(err.Error(), "unsupported file type") {
		t.Fatalf("validateThemePackage() error = %v, want unsupported file type", err)
	}
	if err := validateThemeAssets(t.TempDir(), &ThemeManifest{CSS: "missing.css"}); err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("validateThemeAssets(missing asset) error = %v, want missing file", err)
	}

	themeDir := t.TempDir()
	if packageHasTemplates(themeDir) {
		t.Fatal("packageHasTemplates(empty) = true, want false")
	}
	if err := os.MkdirAll(filepath.Join(themeDir, "templates", "posts"), 0755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "templates", "posts", "show.jet.html"), []byte("template"), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	if !packageHasTemplates(themeDir) {
		t.Fatal("packageHasTemplates() = false, want true")
	}

	allowed := []string{"theme.json", "assets/app.css", "templates/index.jet.html", "fonts/site.woff2"}
	for _, name := range allowed {
		if !isAllowedThemeFile(name) {
			t.Fatalf("isAllowedThemeFile(%q) = false, want true", name)
		}
	}
	if isAllowedThemeFile("bin/run.sh") || isAllowedThemeFile("") {
		t.Fatal("isAllowedThemeFile() accepted unsupported path")
	}
}

func TestThemePreviewTokenLifecycleAndHelpers(t *testing.T) {
	setupServiceTestDB(t)
	svc := &ThemeService{}
	theme := &mysql.ThemeModel{Name: "preview_theme", DisplayName: "Preview Theme"}
	if err := svc.CreateTheme(theme); err != nil {
		t.Fatalf("CreateTheme() error = %v", err)
	}

	token, expiresAt, err := svc.CreatePreviewToken(int64(theme.ID), time.Minute)
	if err != nil {
		t.Fatalf("CreatePreviewToken() error = %v", err)
	}
	if token == "" || !expiresAt.After(time.Now()) {
		t.Fatalf("token = %q expiresAt = %v", token, expiresAt)
	}
	validated, err := svc.ValidatePreviewToken(token)
	if err != nil {
		t.Fatalf("ValidatePreviewToken() error = %v", err)
	}
	if validated.ID != theme.ID {
		t.Fatalf("validated theme id = %d, want %d", validated.ID, theme.ID)
	}
	if _, _, err := svc.CreatePreviewToken(99999, time.Minute); err == nil {
		t.Fatal("CreatePreviewToken(missing) expected error")
	}
	if _, err := svc.ValidatePreviewToken(token + "x"); err == nil {
		t.Fatal("ValidatePreviewToken(tampered) expected error")
	}
	if _, err := svc.parsePreviewToken("not-a-token"); err == nil {
		t.Fatal("parsePreviewToken(malformed) expected error")
	}

	secret, err := svc.getPreviewSecret()
	if err != nil {
		t.Fatalf("getPreviewSecret() error = %v", err)
	}
	payloadJSON, err := json.Marshal(ThemePreviewPayload{
		ThemeID: theme.ID,
		Expires: time.Now().Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(payloadJSON)
	expiredToken := payloadPart + "." + signPreviewPayload([]byte(payloadPart), []byte(secret))
	if _, err := svc.ValidatePreviewToken(expiredToken); err == nil {
		t.Fatal("ValidatePreviewToken(expired) expected error")
	}

	random, err := randomSecret()
	if err != nil {
		t.Fatalf("randomSecret() error = %v", err)
	}
	if random == "" {
		t.Fatal("randomSecret() returned empty secret")
	}
	if !IsHiddenSettingKey(themePreviewSecretKey) || IsHiddenSettingKey("SiteTitle") {
		t.Fatal("IsHiddenSettingKey() mismatch")
	}
	if SafePreviewPath("") != "/" || SafePreviewPath("https://example.com") != "/" || SafePreviewPath("//example.com") != "/" {
		t.Fatal("SafePreviewPath() failed unsafe path normalization")
	}
	if SafePreviewPath("/articles/1") != "/articles/1" {
		t.Fatal("SafePreviewPath() rejected local path")
	}
	previewURL := PreviewURL("a+b&c", "/articles/1?draft=true")
	if !strings.Contains(previewURL, "token=a%2Bb%26c") || !strings.Contains(previewURL, "path=%2Farticles%2F1%3Fdraft%3Dtrue") {
		t.Fatalf("PreviewURL() = %q", previewURL)
	}
	if id, err := ParsePreviewID("42"); err != nil || id != 42 {
		t.Fatalf("ParsePreviewID() = %d, %v", id, err)
	}
	if _, err := ParsePreviewID("0"); err == nil {
		t.Fatal("ParsePreviewID(0) expected error")
	}
}

func TestDeleteThemeRemovesCustomThemeDirectory(t *testing.T) {
	setupServiceTestDB(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	theme := &mysql.ThemeModel{Name: "delete_me", DisplayName: "Delete Me"}
	if err := serviceTheme().CreateTheme(theme); err != nil {
		t.Fatalf("CreateTheme() error = %v", err)
	}
	themeDir := filepath.Join(themeStorageDir, theme.Name)
	if err := os.MkdirAll(themeDir, 0755); err != nil {
		t.Fatalf("mkdir theme dir: %v", err)
	}
	if err := serviceTheme().DeleteTheme(int64(theme.ID)); err != nil {
		t.Fatalf("DeleteTheme() error = %v", err)
	}
	if _, err := os.Stat(themeDir); !os.IsNotExist(err) {
		t.Fatalf("theme dir stat err = %v, want not exist", err)
	}
	reinstalled := &mysql.ThemeModel{Name: "delete_me", DisplayName: "Reinstalled"}
	if err := serviceTheme().CreateTheme(reinstalled); err != nil {
		t.Fatalf("CreateTheme(same name after delete) error = %v", err)
	}
}

func TestThemeNameIsImmutableAndUnsafeLegacyDeleteIsContained(t *testing.T) {
	setupServiceTestDB(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	theme := &mysql.ThemeModel{Name: "stable_name", DisplayName: "Stable"}
	if err := serviceTheme().CreateTheme(theme); err != nil {
		t.Fatalf("CreateTheme() error = %v", err)
	}
	theme.Name = "renamed"
	if err := serviceTheme().UpdateTheme(theme); !errors.Is(err, ErrThemeNameImmutable) {
		t.Fatalf("UpdateTheme(rename) error = %v, want ErrThemeNameImmutable", err)
	}

	legacy := &mysql.ThemeModel{Name: "..", DisplayName: "Unsafe legacy"}
	if err := dal.MySqlDB.Theme.Create(legacy); err != nil {
		t.Fatalf("create legacy theme: %v", err)
	}
	if err := os.MkdirAll("storage", 0755); err != nil {
		t.Fatalf("mkdir storage: %v", err)
	}
	if err := os.WriteFile("storage/sentinel", []byte("keep"), 0644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	if err := serviceTheme().DeleteTheme(int64(legacy.ID)); err == nil {
		t.Fatal("DeleteTheme(unsafe legacy name) expected error")
	}
	if _, err := os.Stat("storage/sentinel"); err != nil {
		t.Fatalf("unsafe delete touched storage root: %v", err)
	}
}

func serviceTheme() *ThemeService {
	return &ThemeService{}
}

func writeThemeZip(t *testing.T, files map[string]string) string {
	t.Helper()

	zipPath := filepath.Join(t.TempDir(), "theme.zip")
	out, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer out.Close()

	writer := zip.NewWriter(out)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return zipPath
}
