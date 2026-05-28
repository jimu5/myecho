package service

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
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
			"config": {"color": "blue"}
		}`,
		"theme/style.css":   "body { color: blue; }",
		"theme/script.js":   "window.theme = true;",
		"theme/preview.png": "preview",
	})

	theme, err := (&ThemeService{}).InstallThemePackage(zipPath)
	if err != nil {
		t.Fatalf("InstallThemePackage() error = %v", err)
	}
	if theme.Name != "upload_theme" || theme.DisplayName != "Upload Theme" {
		t.Fatalf("theme = %+v", theme)
	}
	if theme.CSS == "" || theme.JS != "window.theme = true;" || theme.Preview != "/themes/upload_theme/preview.png" {
		t.Fatalf("unexpected theme assets: %+v", theme)
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
