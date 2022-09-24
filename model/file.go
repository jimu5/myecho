package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"myecho/config"
	"os"
	"path/filepath"
	"strings"
)

type File struct {
	BaseModel
	Name          string `json:"name" gorm:"size:255"`
	Path          string `json:"path" gorm:"size:512"`
	UUID          string `json:"uuid" gorm:"size:36"`
	ExtensionName string `json:"type" gorm:"size:32"`
}

func (f *File) BeforeCreate(tx *gorm.DB) error {
	if len(f.UUID) == 0 {
		f.UUID = uuid.New().String()
	}
	return nil
}

func (f *File) GetUrlPath() string {
	uPath := filepath.Join(config.StorageRootUrl, f.Path, f.UUID)
	if config.OSName == config.WINDOWS {
		uPath = strings.Replace(uPath, "\\", "/", -1)
	}
	return uPath + f.ExtensionName
}

func (f *File) GetActualPath() string {
	fPath := filepath.Join(config.StorageRootPath, f.Path, f.UUID)
	return fPath + f.ExtensionName
}

func (f *File) GetActualDir() string {
	return filepath.Join(config.StorageRootPath, f.Path)
}

func (f *File) CreateDirIfNotExist() error {
	_, err := os.Stat(f.GetActualDir())
	if os.IsNotExist(err) {
		createErr := os.MkdirAll(f.GetActualDir(), os.ModePerm)
		if createErr != nil {
			return createErr
		}
	}
	return err
}
