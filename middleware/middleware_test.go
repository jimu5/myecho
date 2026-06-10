package middleware

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"myecho/dal/connect"
	"myecho/model"
)

func TestIsPathSkipCache(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/api/articles", want: true},
		{path: "/mos/file.png", want: true},
		{path: "/status", want: true},
		{path: "/articles", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isPathSkipCache(tt.path); got != tt.want {
				t.Fatalf("isPathSkipCache() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheConfigKeyGeneratorUsesOriginalURL(t *testing.T) {
	app := fiber.New()
	app.Get("/api/items", func(c *fiber.Ctx) error {
		if CacheConfig.KeyGenerator(c) != "/api/items?page=1" {
			t.Fatalf("KeyGenerator() = %q, want original URL", CacheConfig.KeyGenerator(c))
		}
		if !CacheConfig.Next(c) {
			t.Fatal("CacheConfig.Next() should skip /api prefix")
		}
		return nil
	})
	req := httptest.NewRequest("GET", "/api/items?page=1", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
}

func TestMWRequestTimeCostSetsLocal(t *testing.T) {
	app := fiber.New()
	app.Use(MWRequestTimeCost)
	app.Get("/", func(c *fiber.Ctx) error {
		cost, ok := c.Locals("RequestTimeDuration").(*RequestTimeDuration)
		if !ok || cost == nil {
			t.Fatal("RequestTimeDuration local missing")
		}
		if cost.GetTimeCost() < 0 {
			t.Fatal("time cost should not be negative")
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusNoContent)
	}
}

func TestMWViewChangeAlwaysContinues(t *testing.T) {
	app := fiber.New()
	app.Use(MWViewChange)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	for _, mode := range []string{"", "json"} {
		req := httptest.NewRequest("GET", "/", nil)
		if mode != "" {
			req.Header.Set("M-Mode", mode)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test(%q) error = %v", mode, err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
	}
}

func TestCustom404ErrorHandlerAPIRoute(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return Custom404ErrorHandler(c)
	}})
	resp, err := app.Test(httptest.NewRequest("GET", "/api/missing", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestCommonErrorHandlerForAPIAndPageRoutes(t *testing.T) {
	app := fiber.New()
	app.Use(CommonErrorHandler)
	app.Get("/api/fail", func(c *fiber.Ctx) error { return errors.New("boom") })
	app.Get("/page/fail", func(c *fiber.Ctx) error { return errors.New("page boom") })

	resp, err := app.Test(httptest.NewRequest("GET", "/api/fail", nil))
	if err != nil {
		t.Fatalf("api app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("api status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
	}

	resp, err = app.Test(httptest.NewRequest("GET", "/page/fail", nil))
	if err != nil {
		t.Fatalf("page app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("page status = %d, want %d", resp.StatusCode, fiber.StatusInternalServerError)
	}
}

func TestRequestTimeDuration(t *testing.T) {
	cost := &RequestTimeDuration{StartTime: time.Now().Add(-time.Millisecond)}
	if cost.GetTimeCost() <= 0 {
		t.Fatal("GetTimeCost() should be positive for past start time")
	}
}

func TestGetUserByTokenAndAuthenticationSuccess(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	connect.Database = db
	user := &model.User{Name: "admin", Token: "token-123"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	got, err := GetUserByToken("token-123")
	if err != nil {
		t.Fatalf("GetUserByToken() error = %v", err)
	}
	if got.ID != user.ID {
		t.Fatalf("GetUserByToken() = %+v, want ID %d", got, user.ID)
	}
	if _, err := GetUserByToken("missing"); err == nil {
		t.Fatal("GetUserByToken(missing) expected error")
	}

	app := fiber.New()
	app.Use(Authentication)
	app.Get("/", func(c *fiber.Ctx) error {
		localUser, ok := c.Locals("user").(*model.User)
		if !ok || localUser.ID != user.ID {
			t.Fatalf("user local = %#v", c.Locals("user"))
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "token token-123")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusNoContent)
	}
}
