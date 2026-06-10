package yaml_config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFillDefaultConfigInitializesMissingSections(t *testing.T) {
	oldYaml := Yaml
	t.Cleanup(func() { Yaml = oldYaml })

	Yaml = YAMLConfig{}
	fillDefaultConfig()

	if Yaml.Database == nil {
		t.Fatal("Database is nil")
	}
	if Yaml.Database.DBName != "myecho" {
		t.Fatalf("DBName = %q, want myecho", Yaml.Database.DBName)
	}
	if Yaml.APPConfig == nil {
		t.Fatal("APPConfig is nil")
	}
}

func TestFillDefaultConfigPreservesConfiguredDatabaseName(t *testing.T) {
	oldYaml := Yaml
	t.Cleanup(func() { Yaml = oldYaml })

	Yaml = YAMLConfig{Database: &Database{DBName: "custom"}}
	fillDefaultConfig()

	if Yaml.Database.DBName != "custom" {
		t.Fatalf("DBName = %q, want custom", Yaml.Database.DBName)
	}
	if Yaml.APPConfig == nil {
		t.Fatal("APPConfig is nil")
	}
}

func TestReadYAMLConfigLoadsFileAndFillsDefaults(t *testing.T) {
	oldYaml := Yaml
	t.Cleanup(func() { Yaml = oldYaml })

	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})
	content := []byte("database:\n  type: sqlite\napp_config:\n  allow_register: true\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	Yaml = YAMLConfig{}
	ReadYAMLConfig()

	if Yaml.Database == nil || Yaml.Database.Type != "sqlite" || Yaml.Database.DBName != "myecho" {
		t.Fatalf("Database = %+v, want sqlite with default db name", Yaml.Database)
	}
	if Yaml.APPConfig == nil || !Yaml.APPConfig.AllowRegister {
		t.Fatalf("APPConfig = %+v, want allow_register true", Yaml.APPConfig)
	}
}
