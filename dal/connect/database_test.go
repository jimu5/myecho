package connect

import (
	"path/filepath"
	"testing"

	"myecho/config/yaml_config"
)

func TestConnectDBWithSQLiteConfigMigratesModels(t *testing.T) {
	yaml_config.Yaml.Database = &yaml_config.Database{
		Type:   "sqlite",
		DBName: filepath.Join(t.TempDir(), "myecho-test"),
	}

	ConnectDB()
	if Database == nil {
		t.Fatal("Database is nil")
	}
	if !Database.Migrator().HasTable("settings") || !Database.Migrator().HasTable("articles") {
		t.Fatal("expected core tables to be migrated")
	}
	sqlDB, err := Database.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}

func TestGetDialectorFromYamlConfigFallbacks(t *testing.T) {
	cases := []yaml_config.Database{
		{Type: "", DBName: filepath.Join(t.TempDir(), "default")},
		{Type: "mysql", DBName: filepath.Join(t.TempDir(), "mysql-fallback")},
		{Type: "postgresql", Host: "localhost", Port: "5432", User: "user", Password: "pass", DBName: "blog"},
	}
	for _, tc := range cases {
		yaml_config.Yaml.Database = &tc
		if got := getDialectorFromYamlConfig(); got == nil {
			t.Fatalf("getDialectorFromYamlConfig(%q) = nil", tc.Type)
		}
	}
}
