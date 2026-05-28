package utils

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGetDuplicateSliceKeepsFirstOccurrenceOrder(t *testing.T) {
	got := GetDuplicateSlice([]string{"go", "js", "go", "css", "js"})
	want := []string{"go", "js", "css"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetDuplicateSlice() = %#v, want %#v", got, want)
	}
}

func TestGetDuplicateSliceHandlesNilAndNumericTypes(t *testing.T) {
	var nilInput []int
	if got := GetDuplicateSlice(nilInput); len(got) != 0 {
		t.Fatalf("nil input produced %v, want empty slice", got)
	}

	got := GetDuplicateSlice([]float32{1.5, 1.5, 2.5})
	want := []float32{1.5, 2.5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetDuplicateSlice() = %#v, want %#v", got, want)
	}
}

func TestGetFileMD5ReturnsExpectedDigest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.txt")
	content := []byte("myecho")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := GetFileMD5(path)
	if err != nil {
		t.Fatalf("GetFileMD5() error = %v", err)
	}
	sum := md5.Sum(content)
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("GetFileMD5() = %q, want %q", got, want)
	}
}

func TestCreateDirIfNotExist(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	if err := CreateDirIfNotExist(dir); err != nil {
		t.Fatalf("CreateDirIfNotExist() error = %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("created path is not a directory: info=%v err=%v", info, err)
	}
	if err := CreateDirIfNotExist(dir); err != nil {
		t.Fatalf("CreateDirIfNotExist() on existing dir error = %v", err)
	}
}

func TestParseFileFullName(t *testing.T) {
	tests := []struct {
		name     string
		fullName string
		wantName string
		wantExt  string
	}{
		{name: "plain extension", fullName: "avatar.png", wantName: "avatar", wantExt: ".png"},
		{name: "multi dot", fullName: "archive.tar.gz", wantName: "archive.tar", wantExt: ".gz"},
		{name: "unicode", fullName: "文章.md", wantName: "文章", wantExt: ".md"},
		{name: "no extension", fullName: "LICENSE", wantName: "LICENSE", wantExt: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotExt := ParseFileFullName(tt.fullName)
			if gotName != tt.wantName || gotExt != tt.wantExt {
				t.Fatalf("ParseFileFullName() = (%q, %q), want (%q, %q)", gotName, gotExt, tt.wantName, tt.wantExt)
			}
		})
	}
}
