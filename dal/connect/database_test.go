package connect

import (
	"path/filepath"
	"testing"

	"myecho/config/yaml_config"
	"myecho/model"

	"github.com/glebarez/sqlite"
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

func TestRepairEmptyCategoryUIDs_BitsUT(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "repair.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&model.Category{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	categories := []model.Category{
		{Name: "missing UID", Type: model.CategoryTypeArticle},
		{Name: "existing UID", UID: "existing-category-uid", Type: model.CategoryTypeLink},
	}
	if err := db.Create(&categories).Error; err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := repairEmptyCategoryUIDs(db); err != nil {
		t.Fatalf("repairEmptyCategoryUIDs() error = %v", err)
	}

	var got []model.Category
	if err := db.Order("id").Find(&got).Error; err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("category count = %d, want 2", len(got))
	}
	if len(got[0].UID) != 20 {
		t.Fatalf("repaired UID length = %d, want 20", len(got[0].UID))
	}
	if got[1].UID != "existing-category-uid" {
		t.Fatalf("existing UID = %q, want unchanged", got[1].UID)
	}
}

func TestRepairOrphanArticleRelations_BitsUT(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "repair-relations.db")), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(
		&model.Article{}, &model.ArticleDetail{}, &model.ArticleRevision{}, &model.ArticleSlugRedirect{},
		&model.ArticleDailyStat{}, &model.Comment{}, &model.Tag{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	live := model.Article{UID: "live", Slug: "live", Type: model.ArticleTypePost}
	deleted := model.Article{UID: "deleted", Slug: "deleted", Type: model.ArticleTypePost}
	if err := db.Create(&[]*model.Article{&live, &deleted}).Error; err != nil {
		t.Fatalf("create articles: %v", err)
	}
	if err := db.Delete(&deleted).Error; err != nil {
		t.Fatalf("soft-delete article: %v", err)
	}
	for _, article := range []model.Article{live, deleted} {
		if err := db.Create(&model.ArticleRevision{ArticleID: article.ID}).Error; err != nil {
			t.Fatalf("create revision: %v", err)
		}
		if err := db.Create(&model.ArticleSlugRedirect{ArticleUID: article.UID, Slug: "old-" + article.UID, Type: model.ArticleTypePost}).Error; err != nil {
			t.Fatalf("create redirect: %v", err)
		}
		if err := db.Create(&model.ArticleDailyStat{ArticleUID: article.UID, Date: "2026-07-27"}).Error; err != nil {
			t.Fatalf("create stat: %v", err)
		}
		if err := db.Create(&model.Comment{ArticleUID: article.UID}).Error; err != nil {
			t.Fatalf("create comment: %v", err)
		}
		if err := db.Table("article_tags").Create(map[string]interface{}{"article_uid": article.UID, "tag_uid": "tag"}).Error; err != nil {
			t.Fatalf("create article tag: %v", err)
		}
	}

	if err := repairOrphanArticleRelations(db); err != nil {
		t.Fatalf("repairOrphanArticleRelations() error = %v", err)
	}
	for name, query := range map[string]*gorm.DB{
		"revisions": db.Model(&model.ArticleRevision{}),
		"redirects": db.Model(&model.ArticleSlugRedirect{}),
		"stats":     db.Model(&model.ArticleDailyStat{}),
		"comments":  db.Unscoped().Model(&model.Comment{}),
		"tags":      db.Table("article_tags"),
	} {
		var count int64
		if err := query.Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if count != 1 {
			t.Fatalf("%s count = %d, want only the live article relation", name, count)
		}
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
