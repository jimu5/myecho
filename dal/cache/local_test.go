package cache

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

func TestMysqlSettingMapSetGetDelete(t *testing.T) {
	var settingMap MysqlSettingMap
	setting := &mysql.SettingModel{Key: "SiteTitle", Value: "Myecho"}

	settingMap.Set(setting.Key, setting)
	setting.Value = "Changed"

	got, ok := settingMap.Get("SiteTitle")
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	if got.Value != "Myecho" {
		t.Fatalf("Get() Value = %q, want stored copy", got.Value)
	}
	if gotString := settingMap.GetStringValue("SiteTitle"); gotString != "Myecho" {
		t.Fatalf("GetStringValue() = %q, want Myecho", gotString)
	}

	settingMap.Delete("SiteTitle")
	if _, ok := settingMap.Get("SiteTitle"); ok {
		t.Fatal("Get() ok = true after Delete()")
	}
	if gotString := settingMap.GetStringValue("SiteTitle"); gotString != "" {
		t.Fatalf("GetStringValue(missing) = %q, want empty", gotString)
	}
}

func TestInitSettingCacheLoadsSettings(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Setting{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	connect.Database = db
	mysql.InitDB()
	if err := (&mysql.SettingRepo{}).Create(&mysql.SettingModel{Key: "SiteTitle", Value: "Myecho"}); err != nil {
		t.Fatalf("create setting: %v", err)
	}

	settingMap := InitSettingCache()
	if got := settingMap.GetStringValue("SiteTitle"); got != "Myecho" {
		t.Fatalf("GetStringValue(SiteTitle) = %q, want Myecho", got)
	}
}
