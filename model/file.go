package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"myecho/config/static_config"
	"myecho/utils"
	"os"
	"path/filepath"
	"strings"
)

type File struct {
	BaseModel
	Name          string `json:"name" gorm:"size:255"`
	DirPath       string `json:"dir_path" gorm:"size:512"`
	UUID          string `json:"uuid" gorm:"size:36"`
	ExtensionName string `json:"type" gorm:"size:32"`
	MD5           string `json:"md5" gorm:"size:255"`
}

func (f *File) BeforeCreate(tx *gorm.DB) error {
	f.SetUUID()
	return f.SetMD5()
}

func (f *File) SetUUID() {
	if len(f.UUID) == 0 {
		f.UUID = uuid.New().String()
	}
}

func (f *File) SetMD5() error {
	if len(f.MD5) == 0 {
		md5, err := utils.GetFileMD5(f.GetActualSavePath())
		if err != nil {
			return err
		}
		f.MD5 = md5
	}
	return nil
}

// 写入库的文件都不会存在 temp 中
func (f *File) GetTempSavePath() string {
	if len(f.UUID) == 0 {
		f.SetUUID()
	}
	err := f.CreateTempIfNotExist()
	if err != nil {
		panic(err)
	}
	tempPath := filepath.Join(static_config.StorageTempPath, f.UUID)
	return tempPath + f.ExtensionName
}

func (f *File) GetUrlPath() string {
	if len(f.UUID) == 0 {
		f.SetUUID()
	}
	err := f.CreateDirIfNotExist()
	if err != nil {
		panic(err)
	}
	uPath := filepath.Join(static_config.StorageRootUrl, f.DirPath, f.UUID)
	if static_config.OSName == static_config.WINDOWS {
		uPath = strings.Replace(uPath, "\\", "/", -1)
	}
	return uPath + f.ExtensionName
}

func (f *File) GetActualSavePath() string {
	if len(f.UUID) == 0 {
		f.SetUUID()
	}
	fPath := filepath.Join(static_config.StorageRootPath, f.DirPath, f.UUID)
	return fPath + f.ExtensionName
}

func (f *File) GetActualSaveDir() string {
	return filepath.Join(static_config.StorageRootPath, f.DirPath)
}

func (f *File) GetTempSaveFileMD5() (string, error) {
	return utils.GetFileMD5(f.GetTempSavePath())
}
func (f *File) GetActualFileMD5() (string, error) {
	return utils.GetFileMD5(f.GetActualSavePath())
}

func (f *File) CreateDirIfNotExist() error {
	return utils.CreateDirIfNotExist(f.GetActualSaveDir())
}

func (f *File) CreateTempIfNotExist() error {
	return utils.CreateDirIfNotExist(static_config.StorageTempPath)
}

func (f *File) MoveTempFileToActualPath() error {
	return os.Rename(f.GetTempSavePath(), f.GetActualSavePath())
}
