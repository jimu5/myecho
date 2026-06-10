package cache

import (
	"testing"

	"myecho/dal/mysql"
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
