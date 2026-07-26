package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallStaticPagePackageCreatesAndListsPage(t *testing.T) {
	chdirServiceTestTemp(t)
	zipPath := writeThemeZip(t, map[string]string{
		"landing/static-page.json": `{
			"name": "landing",
			"display_name": "Landing Page",
			"author": "Codex",
			"version": "1.0.0",
			"description": "Campaign page"
		}`,
		"landing/index.html":     "<!doctype html><title>Landing</title>",
		"landing/assets/app.css": "body { color: #111; }",
	})

	page, err := (&StaticPageService{}).InstallStaticPagePackage(zipPath)
	if err != nil {
		t.Fatalf("InstallStaticPagePackage() error = %v", err)
	}
	if page.Name != "landing" || page.DisplayName != "Landing Page" || page.URL != "/static-pages/landing/" {
		t.Fatalf("page = %+v", page)
	}
	if page.AssetBaseURL != "/static-pages/landing/" || page.Entry != "index.html" {
		t.Fatalf("unexpected static page URLs: %+v", page)
	}
	if _, err := os.Stat(filepath.Join(StaticPageStorageDir, "landing", "assets", "app.css")); err != nil {
		t.Fatalf("static page asset not extracted: %v", err)
	}

	pages, err := (&StaticPageService{}).ListStaticPages()
	if err != nil {
		t.Fatalf("ListStaticPages() error = %v", err)
	}
	if len(pages) != 1 || pages[0].Name != "landing" || pages[0].URL != "/static-pages/landing/" {
		t.Fatalf("pages = %+v", pages)
	}

	updated, err := (&StaticPageService{}).SetNavigationVisibility("landing", true)
	if err != nil {
		t.Fatalf("SetNavigationVisibility() error = %v", err)
	}
	if !updated.ShowInNavigation {
		t.Fatalf("updated page = %+v, want navigation enabled", updated)
	}
	updatedZipPath := writeThemeZip(t, map[string]string{
		"landing/static-page.json": `{"name":"landing","display_name":"Landing Page","version":"2.0.0"}`,
		"landing/index.html":       "<!doctype html><title>Landing v2</title>",
	})
	reinstalled, err := (&StaticPageService{}).InstallStaticPagePackage(updatedZipPath)
	if err != nil {
		t.Fatalf("InstallStaticPagePackage(reinstall) error = %v", err)
	}
	if !reinstalled.ShowInNavigation || reinstalled.Version != "2.0.0" {
		t.Fatalf("reinstalled page = %+v, want navigation preference preserved", reinstalled)
	}
	content, err := os.ReadFile(filepath.Join(StaticPageStorageDir, "landing", "index.html"))
	if err != nil || !strings.Contains(string(content), "Landing v2") {
		t.Fatalf("reinstalled page content = %q, err = %v", content, err)
	}
	navigation, err := (&StaticPageService{}).ListNavigationPages()
	if err != nil {
		t.Fatalf("ListNavigationPages() error = %v", err)
	}
	if len(navigation) != 1 || navigation[0].Name != "landing" {
		t.Fatalf("navigation pages = %+v", navigation)
	}
	if _, err := (&StaticPageService{}).SetNavigationVisibility("landing", false); err != nil {
		t.Fatalf("SetNavigationVisibility(false) error = %v", err)
	}
	navigation, err = (&StaticPageService{}).ListNavigationPages()
	if err != nil || len(navigation) != 0 {
		t.Fatalf("navigation pages after disable = %+v, err = %v", navigation, err)
	}
}

func TestStaticPagePackageValidationRejectsUnsafeInputs(t *testing.T) {
	chdirServiceTestTemp(t)

	badNameZip := writeThemeZip(t, map[string]string{
		"page/static-page.json": `{"name":"../bad","display_name":"Bad"}`,
		"page/index.html":       "<!doctype html>",
	})
	if _, err := (&StaticPageService{}).InstallStaticPagePackage(badNameZip); err == nil {
		t.Fatal("InstallStaticPagePackage() expected bad name error")
	}

	unsupportedZip := writeThemeZip(t, map[string]string{
		"page/static-page.json": `{"name":"bad_file","display_name":"Bad File"}`,
		"page/index.html":       "<!doctype html>",
		"page/shell.php":        "<?php echo 'nope';",
	})
	if _, err := (&StaticPageService{}).InstallStaticPagePackage(unsupportedZip); err == nil || !strings.Contains(err.Error(), "unsupported file type") {
		t.Fatalf("InstallStaticPagePackage() error = %v, want unsupported file type", err)
	}

	missingEntryZip := writeThemeZip(t, map[string]string{
		"page/static-page.json": `{"name":"missing_entry","entry":"home.html"}`,
		"page/index.html":       "<!doctype html>",
	})
	if _, err := (&StaticPageService{}).InstallStaticPagePackage(missingEntryZip); err == nil || !strings.Contains(err.Error(), "entry file not found") {
		t.Fatalf("InstallStaticPagePackage() error = %v, want missing entry", err)
	}

	nonHTMLZip := writeThemeZip(t, map[string]string{
		"page/static-page.json": `{"name":"bad_entry","entry":"app.js"}`,
		"page/app.js":           "window.page = true;",
	})
	if _, err := (&StaticPageService{}).InstallStaticPagePackage(nonHTMLZip); err == nil || !strings.Contains(err.Error(), "entry path is invalid") {
		t.Fatalf("InstallStaticPagePackage() error = %v, want invalid entry", err)
	}
}

func TestStaticPagePublicURLRejectsUnsafeName(t *testing.T) {
	if got := StaticPagePublicBaseURL("../bad"); got != "" {
		t.Fatalf("StaticPagePublicBaseURL() = %q, want empty string", got)
	}
	if got := StaticPagePublicBaseURL("landing"); got != "/static-pages/landing/" {
		t.Fatalf("StaticPagePublicBaseURL() = %q, want landing URL", got)
	}
}

func TestDeleteStaticPageRejectsUnsafeName(t *testing.T) {
	if err := (&StaticPageService{}).DeleteStaticPage("../bad"); err == nil {
		t.Fatal("DeleteStaticPage() expected unsafe name error")
	}
	if _, err := (&StaticPageService{}).SetNavigationVisibility("../bad", true); err == nil {
		t.Fatal("SetNavigationVisibility() expected unsafe name error")
	}
}

func TestStaticPageMutationsRejectConcurrentWrites(t *testing.T) {
	chdirServiceTestTemp(t)
	lockPath := filepath.Join(staticPageMutationLockDir, "landing")
	if err := os.MkdirAll(lockPath, 0700); err != nil {
		t.Fatalf("create static page lock: %v", err)
	}
	if _, err := (&StaticPageService{}).SetNavigationVisibility("landing", true); !errors.Is(err, ErrStaticPageBusy) {
		t.Fatalf("SetNavigationVisibility() error = %v, want ErrStaticPageBusy", err)
	}
}

func chdirServiceTestTemp(t *testing.T) {
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
