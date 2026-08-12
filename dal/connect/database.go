package connect

import (
	"database/sql"
	"fmt"
	"myecho/config/yaml_config"
	"myecho/model"
	"myecho/utils"
	"regexp"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormMigrator "gorm.io/gorm/migrator"
)

var Database *gorm.DB

// 连接数据库
func ConnectDB() {
	var err error
	Database, err = gorm.Open(getDialectorFromYamlConfig(), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		panic(err)
	}

	// 设置连接池
	sqlDB, err := Database.DB()
	if err != nil {
		panic(err)
	}

	// 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(10)
	// 设置打开数据库连接的最大数量
	sqlDB.SetMaxOpenConns(100)
	// 设置连接可复用的最大时间
	sqlDB.SetConnMaxLifetime(time.Hour)
	err = Database.AutoMigrate(
		&model.Setting{},
		&model.Category{},
		&model.User{},
		&model.Tag{},
		&model.ArticleDetail{},
		&model.Comment{},
		&model.File{},
		&model.Article{},
		&model.ArticleRevision{},
		&model.ArticleSlugRedirect{},
		&model.ArticleDailyStat{},
		&model.Link{},
		&model.Theme{},
	)
	if err != nil {
		panic(err)
	}
	if err := repairEmptyCategoryUIDs(Database); err != nil {
		panic(err)
	}
	if err := repairOrphanArticleRelations(Database); err != nil {
		panic(err)
	}
}

func repairEmptyCategoryUIDs(db *gorm.DB) error {
	var categories []model.Category
	if err := db.Where("uid IS NULL OR uid = ?", "").Find(&categories).Error; err != nil {
		return err
	}
	for _, category := range categories {
		if err := db.Model(&model.Category{}).
			Where("id = ?", category.ID).
			Update("uid", utils.GenUID20()).Error; err != nil {
			return err
		}
	}
	return nil
}

func repairOrphanArticleRelations(db *gorm.DB) error {
	for _, deletion := range []struct {
		query *gorm.DB
		value interface{}
	}{
		{db.Where("article_id NOT IN (?)", db.Model(&model.Article{}).Select("id")), &model.ArticleRevision{}},
		{db.Where("article_uid NOT IN (?)", db.Model(&model.Article{}).Select("uid")), &model.ArticleSlugRedirect{}},
		{db.Where("article_uid NOT IN (?)", db.Model(&model.Article{}).Select("uid")), &model.ArticleDailyStat{}},
		{db.Unscoped().Where("article_uid NOT IN (?)", db.Model(&model.Article{}).Select("uid")), &model.Comment{}},
	} {
		if err := deletion.query.Delete(deletion.value).Error; err != nil {
			return err
		}
	}
	return db.Exec("DELETE FROM article_tags WHERE article_uid NOT IN (SELECT uid FROM articles WHERE deleted_at IS NULL)").Error
}

func getDialectorFromYamlConfig() gorm.Dialector {
	dbConfig := yaml_config.Yaml.Database
	var dsn string
	switch dbConfig.Type {
	case "sqlite":
		dsn = dbConfig.DBName + ".db"
		return sqlite.Open(dsn)
	case "mysql":
		// mysql is not wired yet; keep the documented local fallback instead of returning a nil dialector.
		dsn = dbConfig.DBName + ".db"
		return sqlite.Open(dsn)
	case "postgresql":
		dsn = buildPostgresDSN(dbConfig)
		return newPostgresDialector(dsn)
	default:
		// 未配置情况下使用 sqlite
		dsn = dbConfig.DBName + ".db"
		return sqlite.Open(dsn)
	}
}

func buildPostgresDSN(dbConfig *yaml_config.Database) string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Shanghai",
		postgresKeywordValue(dbConfig.Host),
		postgresKeywordValue(dbConfig.User),
		postgresKeywordValue(dbConfig.Password),
		postgresKeywordValue(dbConfig.DBName),
		postgresKeywordValue(dbConfig.Port),
	)
}

func postgresKeywordValue(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return "'" + escaped + "'"
}

type postgresDialector struct {
	gorm.Dialector
}

func newPostgresDialector(dsn string) gorm.Dialector {
	return postgresDialector{Dialector: postgres.Open(dsn)}
}

func (d postgresDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return postgresMigrator{Migrator: postgres.Migrator{Migrator: gormMigrator.Migrator{Config: gormMigrator.Config{
		DB:                          db,
		Dialector:                   d,
		CreateIndexAfterCreateTable: true,
	}}}}
}

type postgresMigrator struct {
	postgres.Migrator
}

// ColumnTypes mirrors the postgres driver migrator, but avoids a parameterized LIMIT in the metadata probe.
func (m postgresMigrator) ColumnTypes(value interface{}) ([]gorm.ColumnType, error) {
	columnTypes := make([]gorm.ColumnType, 0)
	execErr := m.RunWithValue(value, func(stmt *gorm.Statement) (err error) {
		currentDatabase := m.DB.Migrator().CurrentDatabase()
		currentSchema, table := m.CurrentSchema(stmt, stmt.Table)
		columns, err := m.DB.Raw(
			"SELECT c.column_name, c.is_nullable = 'YES', c.udt_name, c.character_maximum_length, c.numeric_precision, c.numeric_precision_radix, c.numeric_scale, c.datetime_precision, 8 * typlen, c.column_default, pd.description, c.identity_increment FROM information_schema.columns AS c JOIN pg_type AS pgt ON c.udt_name = pgt.typname LEFT JOIN pg_catalog.pg_description as pd ON pd.objsubid = c.ordinal_position AND pd.objoid = (SELECT oid FROM pg_catalog.pg_class WHERE relname = c.table_name AND relnamespace = (SELECT oid FROM pg_catalog.pg_namespace WHERE nspname = c.table_schema)) where table_catalog = ? AND table_schema = ? AND table_name = ?",
			currentDatabase, currentSchema, table,
		).Rows()
		if err != nil {
			return err
		}
		defer columns.Close()

		for columns.Next() {
			column := &gormMigrator.ColumnType{
				PrimaryKeyValue: sql.NullBool{Valid: true},
				UniqueValue:     sql.NullBool{Valid: true},
			}
			var (
				datetimePrecision sql.NullInt64
				radixValue        sql.NullInt64
				typeLenValue      sql.NullInt64
				identityIncrement sql.NullString
			)

			err = columns.Scan(
				&column.NameValue, &column.NullableValue, &column.DataTypeValue, &column.LengthValue, &column.DecimalSizeValue,
				&radixValue, &column.ScaleValue, &datetimePrecision, &typeLenValue, &column.DefaultValueValue, &column.CommentValue, &identityIncrement,
			)
			if err != nil {
				return err
			}

			if typeLenValue.Valid && typeLenValue.Int64 > 0 {
				column.LengthValue = typeLenValue
			}

			if (strings.HasPrefix(column.DefaultValueValue.String, "nextval('") &&
				strings.HasSuffix(column.DefaultValueValue.String, "seq'::regclass)")) || (identityIncrement.Valid && identityIncrement.String != "") {
				column.AutoIncrementValue = sql.NullBool{Bool: true, Valid: true}
				column.DefaultValueValue = sql.NullString{}
			}

			if column.DefaultValueValue.Valid {
				column.DefaultValueValue.String = regexp.MustCompile(`'?(.*)\b'?:+[\w\s]+$`).ReplaceAllString(column.DefaultValueValue.String, "$1")
			}

			if datetimePrecision.Valid {
				column.DecimalSizeValue = datetimePrecision
			}

			columnTypes = append(columnTypes, column)
		}

		rows, err := postgresColumnTypesProbe(m.DB, m.CurrentTable(stmt)).Rows()
		if err != nil {
			return err
		}
		defer rows.Close()

		rawColumnTypes, err := rows.ColumnTypes()
		if err != nil {
			return err
		}
		for _, columnType := range columnTypes {
			for _, c := range rawColumnTypes {
				if c.Name() == columnType.Name() {
					columnType.(*gormMigrator.ColumnType).SQLColumnType = c
					break
				}
			}
		}

		columnTypeRows, err := m.DB.Raw("SELECT constraint_name FROM information_schema.table_constraints tc JOIN information_schema.constraint_column_usage AS ccu USING (constraint_schema, constraint_name) JOIN information_schema.columns AS c ON c.table_schema = tc.constraint_schema AND tc.table_name = c.table_name AND ccu.column_name = c.column_name WHERE constraint_type IN ('PRIMARY KEY', 'UNIQUE') AND c.table_catalog = ? AND c.table_schema = ? AND c.table_name = ? AND constraint_type = ?", currentDatabase, currentSchema, table, "UNIQUE").Rows()
		if err != nil {
			return err
		}
		uniqueContraints := map[string]int{}
		for columnTypeRows.Next() {
			var constraintName string
			columnTypeRows.Scan(&constraintName)
			uniqueContraints[constraintName]++
		}
		columnTypeRows.Close()

		columnTypeRows, err = m.DB.Raw("SELECT c.column_name, constraint_name, constraint_type FROM information_schema.table_constraints tc JOIN information_schema.constraint_column_usage AS ccu USING (constraint_schema, constraint_name) JOIN information_schema.columns AS c ON c.table_schema = tc.constraint_schema AND tc.table_name = c.table_name AND ccu.column_name = c.column_name WHERE constraint_type IN ('PRIMARY KEY', 'UNIQUE') AND c.table_catalog = ? AND c.table_schema = ? AND c.table_name = ?", currentDatabase, currentSchema, table).Rows()
		if err != nil {
			return err
		}
		for columnTypeRows.Next() {
			var name, constraintName, columnType string
			columnTypeRows.Scan(&name, &constraintName, &columnType)
			for _, c := range columnTypes {
				mc := c.(*gormMigrator.ColumnType)
				if mc.NameValue.String == name {
					switch columnType {
					case "PRIMARY KEY":
						mc.PrimaryKeyValue = sql.NullBool{Bool: true, Valid: true}
					case "UNIQUE":
						if uniqueContraints[constraintName] == 1 {
							mc.UniqueValue = sql.NullBool{Bool: true, Valid: true}
						}
					}
					break
				}
			}
		}
		columnTypeRows.Close()

		dataTypeRows, err := m.DB.Raw(`SELECT a.attname as column_name, format_type(a.atttypid, a.atttypmod) AS data_type
		FROM pg_attribute a JOIN pg_class b ON a.attrelid = b.oid AND relnamespace = (SELECT oid FROM pg_catalog.pg_namespace WHERE nspname = ?)
		WHERE a.attnum > 0
		AND NOT a.attisdropped
		AND b.relname = ?`, currentSchema, table).Rows()
		if err != nil {
			return err
		}
		for dataTypeRows.Next() {
			var name, dataType string
			dataTypeRows.Scan(&name, &dataType)
			for _, c := range columnTypes {
				mc := c.(*gormMigrator.ColumnType)
				if mc.NameValue.String == name {
					mc.ColumnTypeValue = sql.NullString{String: dataType, Valid: true}
					if strings.HasPrefix(mc.DataTypeValue.String, "_") {
						mc.DataTypeValue = sql.NullString{String: dataType, Valid: true}
					}
					break
				}
			}
		}
		dataTypeRows.Close()

		return
	})

	return columnTypes, execErr
}

func postgresColumnTypesProbe(db *gorm.DB, table interface{}) *gorm.DB {
	return db.Session(&gorm.Session{}).Raw("SELECT * FROM ? LIMIT 1", table)
}
