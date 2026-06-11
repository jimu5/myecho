package connect

import (
	"path/filepath"
	"testing"

	"myecho/config/yaml_config"

	"github.com/jackc/pgx/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func TestBuildPostgresDSNEscapesKeywordValues(t *testing.T) {
	dsn := buildPostgresDSN(&yaml_config.Database{
		Type:     "postgresql",
		Host:     "localhost",
		Port:     "5432",
		User:     "blog user",
		Password: `pa ss'word\trail`,
		DBName:   "my echo",
	})

	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}
	if cfg.User != "blog user" {
		t.Fatalf("User = %q, want %q", cfg.User, "blog user")
	}
	if cfg.Password != `pa ss'word\trail` {
		t.Fatalf("Password = %q, want %q", cfg.Password, `pa ss'word\trail`)
	}
	if cfg.Database != "my echo" {
		t.Fatalf("Database = %q, want %q", cfg.Database, "my echo")
	}
}

func TestBuildPostgresDSNDoesNotAllowPasswordParameterInjection(t *testing.T) {
	dsn := buildPostgresDSN(&yaml_config.Database{
		Type:     "postgresql",
		Host:     "localhost",
		Port:     "5432",
		User:     "blog",
		Password: "secret default_query_exec_mode=simple_protocol",
		DBName:   "myecho",
	})

	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}
	if cfg.Password != "secret default_query_exec_mode=simple_protocol" {
		t.Fatalf("Password = %q, want full literal password", cfg.Password)
	}
	if cfg.DefaultQueryExecMode == pgx.QueryExecModeSimpleProtocol {
		t.Fatal("password content changed pgx default_query_exec_mode")
	}
}

func TestBuildPostgresDSNKeepsDefaultQueryMode(t *testing.T) {
	dsn := buildPostgresDSN(&yaml_config.Database{
		Type:     "postgresql",
		Host:     "127.0.0.1",
		Port:     "5432",
		User:     "myecho",
		Password: "blog-password",
		DBName:   "myecho",
	})

	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}
	if cfg.DefaultQueryExecMode == pgx.QueryExecModeSimpleProtocol {
		t.Fatal("DefaultQueryExecMode unexpectedly uses simple protocol")
	}
}

func TestPostgresColumnTypesProbeUsesParameterizedLimit(t *testing.T) {
	db, err := gorm.Open(postgres.Open(buildPostgresDSN(&yaml_config.Database{
		Type:     "postgresql",
		Host:     "127.0.0.1",
		Port:     "5432",
		User:     "myecho",
		Password: "blog-password",
		DBName:   "myecho",
	})), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("gorm.Open returned error: %v", err)
	}

	tx := db.Table("settings").Limit(1).Find(&[]map[string]interface{}{})
	if got, want := tx.Statement.SQL.String(), `SELECT * FROM "settings" LIMIT $1`; got != want {
		t.Fatalf("SQL = %q, want %q", got, want)
	}
	if len(tx.Statement.Vars) != 1 || tx.Statement.Vars[0] != 1 {
		t.Fatalf("Vars = %#v, want []interface{}{1}", tx.Statement.Vars)
	}
}

func TestPostgresColumnTypesProbeUsesLiteralLimit(t *testing.T) {
	db, err := gorm.Open(postgres.Open(buildPostgresDSN(&yaml_config.Database{
		Type:     "postgresql",
		Host:     "127.0.0.1",
		Port:     "5432",
		User:     "myecho",
		Password: "blog-password",
		DBName:   "myecho",
	})), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("gorm.Open returned error: %v", err)
	}

	tx := postgresColumnTypesProbe(db, clause.Table{Name: "settings"})
	if got, want := tx.Statement.SQL.String(), `SELECT * FROM "settings" LIMIT 1`; got != want {
		t.Fatalf("SQL = %q, want %q", got, want)
	}
	if len(tx.Statement.Vars) != 0 {
		t.Fatalf("Vars = %#v, want none", tx.Statement.Vars)
	}
}
