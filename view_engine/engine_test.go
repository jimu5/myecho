package view_engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"myecho/dal/mysql"
	"myecho/model"
)

func TestRenderUsesBaseTemplatesByDefault(t *testing.T) {
	viewDir := t.TempDir()
	writeTemplate(t, viewDir, "page.jet.html", "base {{ Title }}")

	engine := newTestEngine(viewDir)
	var out strings.Builder
	if err := engine.Render(&out, "page.jet.html", fiber.Map{"Title": "Page"}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if out.String() != "base Page" {
		t.Fatalf("Render() = %q, want base template", out.String())
	}

	if got := themeFromData("not a map"); got != nil {
		t.Fatalf("themeFromData(non-map) = %+v, want nil", got)
	}
	if got := engine.setForData(fiber.Map{}); got != engine.Set {
		t.Fatal("setForData() without theme should return default set")
	}
	if err := engine.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestRenderUsesThemeTemplatesWhenEnabled(t *testing.T) {
	workingDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workingDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	viewDir := filepath.Join(workingDir, "views")
	writeTemplate(t, viewDir, "page.jet.html", "base {{ Title }}")
	writeTemplate(t, filepath.Join(workingDir, "storage", "themes", "clean", "templates"), "page.jet.html", "theme {{ Title }}")

	engine := newTestEngine(viewDir)
	theme := &mysql.ThemeModel{
		BaseModel:    model.BaseModel{UpdatedAt: time.Unix(100, 0)},
		Name:         "clean",
		HasTemplates: true,
	}

	var out strings.Builder
	if err := engine.Render(&out, "page.jet.html", fiber.Map{"Title": "Page", "Theme": theme}); err != nil {
		t.Fatalf("Render() with theme error = %v", err)
	}
	if out.String() != "theme Page" {
		t.Fatalf("Render() = %q, want theme template", out.String())
	}
	if len(engine.themeSets) != 1 {
		t.Fatalf("theme cache size = %d, want 1", len(engine.themeSets))
	}

	out.Reset()
	theme.HasTemplates = false
	if err := engine.Render(&out, "page.jet.html", fiber.Map{"Title": "Page", "Theme": theme}); err != nil {
		t.Fatalf("Render() without theme templates error = %v", err)
	}
	if out.String() != "base Page" {
		t.Fatalf("Render() = %q, want base template after templates disabled", out.String())
	}
}

func TestThemeSetFallsBackWhenThemeDirectoryMissing(t *testing.T) {
	viewDir := t.TempDir()
	writeTemplate(t, viewDir, "page.jet.html", "base")
	engine := newTestEngine(viewDir)
	theme := &mysql.ThemeModel{
		BaseModel:    model.BaseModel{UpdatedAt: time.Unix(200, 0)},
		Name:         "missing",
		HasTemplates: true,
	}

	if got := engine.setForData(fiber.Map{"Theme": theme}); got != engine.Set {
		t.Fatal("setForData() should return default set when theme directory is missing")
	}
}

func TestThemeNameValidation(t *testing.T) {
	if !isSafeThemeName("clean-theme_1") {
		t.Fatal("isSafeThemeName(valid) = false")
	}
	for _, name := range []string{"", " ../bad", "Bad", "theme.name"} {
		if isSafeThemeName(name) {
			t.Fatalf("isSafeThemeName(%q) = true, want false", name)
		}
	}
}

func newTestEngine(viewDir string) *HotReloadEngine {
	engine := &HotReloadEngine{
		viewDir:   viewDir,
		ext:       ".jet.html",
		themeSets: make(map[string]cachedThemeSet),
	}
	engine.Reload()
	return engine
}

func writeTemplate(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir template dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}
}
