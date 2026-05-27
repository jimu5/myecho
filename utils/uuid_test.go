package utils

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestGenUID(t *testing.T) {
	fmt.Println(GenUID20())
}

func TestGetFileMD5ReturnsErrorForMissingFile(t *testing.T) {
	if _, err := GetFileMD5(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatal("GetFileMD5() expected error for missing file")
	}
}
