package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"myecho/config/yaml_config"
	"myecho/dal/connect"
	"myecho/model"
)

func setupAuthTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	connect.Database = db
	yaml_config.Yaml.APPConfig = &yaml_config.APPConfig{AllowRegister: true}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestRegisterStoresBcryptPassword(t *testing.T) {
	setupAuthTestDB(t)
	app := fiber.New()
	app.Post("/register", Register)

	body := bytes.NewBufferString(`{"name":"admin","email":"admin@example.com","password":"secret"}`)
	req := httptest.NewRequest(fiber.MethodPost, "/register", body)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var user model.User
	if err := connect.Database.Where("name = ?", "admin").First(&user).Error; err != nil {
		t.Fatalf("find user: %v", err)
	}
	if !strings.HasPrefix(user.Password, "$2") {
		t.Fatalf("password was not stored as bcrypt: %q", user.Password)
	}
	ok, shouldUpgrade := CheckPassword(user.Password, "secret")
	if !ok || shouldUpgrade {
		t.Fatalf("CheckPassword() ok=%v shouldUpgrade=%v", ok, shouldUpgrade)
	}
}

func TestLoginMigratesLegacyPassword(t *testing.T) {
	setupAuthTestDB(t)
	legacyPassword := EncryptPassword("secret")
	user := &model.User{Name: "admin", Email: "admin@example.com", Password: legacyPassword}
	if err := connect.Database.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	app := fiber.New()
	app.Post("/login", Login)

	body := bytes.NewBufferString(`{"name":"admin","password":"secret"}`)
	req := httptest.NewRequest(fiber.MethodPost, "/login", body)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var wrapped struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapped); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if wrapped.Code != 0 || wrapped.Data.Token == "" {
		t.Fatalf("unexpected response: %+v", wrapped)
	}
	var updated model.User
	if err := connect.Database.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("find updated user: %v", err)
	}
	if updated.Password == legacyPassword || !strings.HasPrefix(updated.Password, "$2") {
		t.Fatalf("legacy password was not migrated: %q", updated.Password)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	setupAuthTestDB(t)
	hashedPassword, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	user := &model.User{Name: "admin", Email: "admin@example.com", Password: hashedPassword}
	if err := connect.Database.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	app := fiber.New()
	app.Post("/login", Login)

	body := bytes.NewBufferString(`{"name":"admin","password":"bad"}`)
	req := httptest.NewRequest(fiber.MethodPost, "/login", body)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusForbidden)
	}
}
