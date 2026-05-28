package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestFilePathHelpers(t *testing.T) {
	file := &File{Name: "avatar", ExtensionName: ".png", DirPath: "images", UUID: "fixed-uuid"}
	if got := file.GetFullName(); got != "avatar.png" {
		t.Fatalf("GetFullName() = %q", got)
	}
	if got := file.GetActualSaveDir(); !strings.HasSuffix(got, filepath.Join("storage", "images")) {
		t.Fatalf("GetActualSaveDir() = %q", got)
	}
	if got := file.GetActualSavePath(); !strings.HasSuffix(got, filepath.Join("storage", "images", "fixed-uuid.png")) {
		t.Fatalf("GetActualSavePath() = %q", got)
	}
	if got := file.GetUrlPath(); got != "/mos/images/fixed-uuid.png" {
		t.Fatalf("GetUrlPath() = %q", got)
	}
}

func TestFileSetUUIDAndMD5(t *testing.T) {
	file := &File{Name: "data", ExtensionName: ".txt", DirPath: "model", UUID: "model-file"}
	if err := os.MkdirAll(file.GetActualSaveDir(), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join("storage", "model")) })
	if err := os.WriteFile(file.GetActualSavePath(), []byte("content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := file.SetMD5(); err != nil {
		t.Fatalf("SetMD5() error = %v", err)
	}
	if file.MD5 == "" {
		t.Fatal("SetMD5() left MD5 empty")
	}
	if _, err := file.GetActualFileMD5(); err != nil {
		t.Fatalf("GetActualFileMD5() error = %v", err)
	}
	file.UUID = ""
	file.SetUUID()
	if file.UUID == "" {
		t.Fatal("SetUUID() left UUID empty")
	}
}

func TestFileBeforeCreateAndMoveDelete(t *testing.T) {
	file := &File{Name: "move", ExtensionName: ".txt", DirPath: "move", UUID: "move-file"}
	if err := os.MkdirAll(file.GetActualSaveDir(), 0755); err != nil {
		t.Fatalf("mkdir actual: %v", err)
	}
	if err := os.WriteFile(file.GetActualSavePath(), []byte("content"), 0644); err != nil {
		t.Fatalf("write actual: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join("storage", "move")) })
	if err := file.BeforeCreate(&gorm.DB{}); err != nil {
		t.Fatalf("BeforeCreate() error = %v", err)
	}
	if file.MD5 == "" {
		t.Fatal("BeforeCreate() left MD5 empty")
	}

	temp := file.GetTempSavePath()
	if err := os.WriteFile(temp, []byte("new"), 0644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	if err := file.MoveTempFileToActualPath(); err != nil {
		t.Fatalf("MoveTempFileToActualPath() error = %v", err)
	}
	if err := os.WriteFile(file.GetTempSavePath(), []byte("temp"), 0644); err != nil {
		t.Fatalf("write second temp: %v", err)
	}
	if _, err := file.GetTempSaveFileMD5(); err != nil {
		t.Fatalf("GetTempSaveFileMD5() error = %v", err)
	}
	if err := file.HardDelete(); err != nil {
		t.Fatalf("HardDelete() error = %v", err)
	}
}

func TestTagBeforeCreateGeneratesUID(t *testing.T) {
	tag := &Tag{}
	if err := tag.BeforeCreate(&gorm.DB{}); err != nil {
		t.Fatalf("BeforeCreate() error = %v", err)
	}
	if len(tag.UID) != 20 {
		t.Fatalf("generated tag UID length = %d, want 20", len(tag.UID))
	}
	existing := &Tag{UID: "existing"}
	if err := existing.BeforeCreate(&gorm.DB{}); err != nil {
		t.Fatalf("BeforeCreate() existing error = %v", err)
	}
	if existing.UID != "existing" {
		t.Fatalf("existing UID replaced with %q", existing.UID)
	}
}
