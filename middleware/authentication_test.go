package middleware

import (
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

func TestAuthenticationRejectsMalformedAuthorization(t *testing.T) {
	app := fiber.New()
	app.Get("/private", Authentication, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	testCases := []string{
		"",
		"token",
		"Bearer abc",
		"token abc extra",
	}
	for _, auth := range testCases {
		req := httptest.NewRequest(fiber.MethodGet, "/private", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test() error = %v", err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("Authorization %q status = %d, want %d", auth, resp.StatusCode, fiber.StatusUnauthorized)
		}
	}
}

func TestAuthenticationRejectsDisabledAndExpiredUsers(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	connect.Database = db
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	users := []model.User{
		{Name: "disabled", Token: "disabled-token", Disabled: true, TokenExpiresAt: time.Now().Add(time.Hour)},
		{Name: "expired", Token: "expired-token", TokenExpiresAt: time.Now().Add(-time.Hour)},
	}
	if err := db.Select("*").Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}

	app := fiber.New()
	app.Get("/private", Authentication, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	for _, token := range []string{"disabled-token", "expired-token"} {
		req := httptest.NewRequest(fiber.MethodGet, "/private", nil)
		req.Header.Set("Authorization", "token "+token)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test(%s) error = %v", token, err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Errorf("token %q status = %d, want %d", token, resp.StatusCode, fiber.StatusUnauthorized)
		}
	}
}
