package main

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSetupStaticPageStaticRouteServesPageDirectory(t *testing.T) {
	chdirMainTestTemp(t)
	pageDir := filepath.Join("storage", "static-pages", "landing", "assets")
	if err := os.MkdirAll(pageDir, 0755); err != nil {
		t.Fatalf("mkdir static page dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join("storage", "static-pages", "landing", "index.html"), []byte("<!doctype html><title>Landing</title>"), 0644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pageDir, "app.css"), []byte("body { color: #111; }"), 0644); err != nil {
		t.Fatalf("write css: %v", err)
	}

	app := fiber.New()
	SetupStaticPageStaticRoute(app)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/static-pages/landing/", nil))
	if err != nil {
		t.Fatalf("index app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("index status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Landing") {
		t.Fatalf("index body = %q", body)
	}

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static-pages/landing/assets/app.css", nil))
	if err != nil {
		t.Fatalf("asset app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("asset status = %d, want 200", resp.StatusCode)
	}
	body, _ = io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "color") {
		t.Fatalf("asset body = %q", body)
	}
}

func chdirMainTestTemp(t *testing.T) {
	t.Helper()
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
}
