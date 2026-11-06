package model

import (
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
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

func TestFileImageMetadataThumbnailAndHardDelete(t *testing.T) {
	file := &File{Name: "photo", ExtensionName: ".png", DirPath: "thumb", UUID: "thumb-file"}
	if err := file.CreateDirIfNotExist(); err != nil {
		t.Fatalf("CreateDirIfNotExist() error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join("storage", "thumb")) })

	originalImage := image.NewRGBA(image.Rect(0, 0, 640, 320))
	for y := 0; y < 320; y++ {
		for x := 0; x < 640; x++ {
			originalImage.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 120, A: 255})
		}
	}
	originalFile, err := os.Create(file.GetActualSavePath())
	if err != nil {
		t.Fatalf("create original image: %v", err)
	}
	if err := png.Encode(originalFile, originalImage); err != nil {
		_ = originalFile.Close()
		t.Fatalf("encode original image: %v", err)
	}
	if err := originalFile.Close(); err != nil {
		t.Fatalf("close original image: %v", err)
	}
	originalBytes, err := os.ReadFile(file.GetActualSavePath())
	if err != nil {
		t.Fatalf("read original image: %v", err)
	}

	if err := file.PopulateImageMetadata(file.GetActualSavePath()); err != nil {
		t.Fatalf("PopulateImageMetadata() error = %v", err)
	}
	if file.Width != 640 || file.Height != 320 {
		t.Fatalf("dimensions = %dx%d, want 640x320", file.Width, file.Height)
	}
	if file.ThumbnailURL != "/mos/thumb/thumb-file.thumb.jpg" {
		t.Fatalf("ThumbnailURL = %q", file.ThumbnailURL)
	}
	thumbnailBytes, err := os.ReadFile(file.GetThumbnailSavePath())
	if err != nil {
		t.Fatalf("read thumbnail: %v", err)
	}
	thumbnailConfig, format, err := image.DecodeConfig(bytes.NewReader(thumbnailBytes))
	if err != nil {
		t.Fatalf("decode thumbnail config: %v", err)
	}
	if format != "jpeg" || thumbnailConfig.Width != 320 || thumbnailConfig.Height != 160 {
		t.Fatalf("thumbnail = %s %dx%d, want jpeg 320x160", format, thumbnailConfig.Width, thumbnailConfig.Height)
	}
	afterBytes, err := os.ReadFile(file.GetActualSavePath())
	if err != nil {
		t.Fatalf("read original image after thumbnail: %v", err)
	}
	if !bytes.Equal(originalBytes, afterBytes) {
		t.Fatal("thumbnail generation modified the original image")
	}

	if err := file.HardDelete(); err != nil {
		t.Fatalf("HardDelete() error = %v", err)
	}
	if _, err := os.Stat(file.GetActualSavePath()); !os.IsNotExist(err) {
		t.Fatalf("original image still exists, stat error = %v", err)
	}
	if _, err := os.Stat(file.GetThumbnailSavePath()); !os.IsNotExist(err) {
		t.Fatalf("thumbnail still exists, stat error = %v", err)
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
