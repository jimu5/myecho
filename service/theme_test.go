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
