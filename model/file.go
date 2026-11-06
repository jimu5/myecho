package model

import (
	"errors"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"myecho/config/static_config"
	"myecho/utils"
)

const (
	thumbnailMaxDimension = 320
	thumbnailSuffix       = ".thumb.jpg"
	maxImagePixels        = 40_000_000
)

type File struct {
	BaseModel
	Name          string `json:"name" gorm:"size:255"` // 文件名, 不含后缀
	DirPath       string `json:"dir_path" gorm:"size:512"`
	UUID          string `json:"uuid" gorm:"size:36;index:idx_files_uuid"`
	ExtensionName string `json:"extension_name" gorm:"size:32"`
	MD5           string `json:"md5" gorm:"size:255"`
	Note          string `json:"note" gorm:"size:256"` // 备注
	UploaderID    uint   `json:"uploader_id" gorm:"index"`
	AltText       string `json:"alt_text" gorm:"size:512"`
	Caption       string `json:"caption" gorm:"size:512"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	ThumbnailURL  string `json:"thumbnail_url" gorm:"size:512"`
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

func (f *File) GetFullName() string {
	return f.Name + f.ExtensionName
}

func (f *File) GetActualSaveDir() string {
	return filepath.Join(static_config.StorageRootPath, f.DirPath)
}

func (f *File) GetThumbnailSavePath() string {
	if len(f.UUID) == 0 {
		f.SetUUID()
	}
	return filepath.Join(f.GetActualSaveDir(), f.UUID+thumbnailSuffix)
}

func (f *File) GetThumbnailURL() string {
	if len(f.UUID) == 0 {
		f.SetUUID()
	}
	uPath := filepath.Join(static_config.StorageRootUrl, f.DirPath, f.UUID+thumbnailSuffix)
	if static_config.OSName == static_config.WINDOWS {
		uPath = strings.Replace(uPath, "\\", "/", -1)
	}
	return uPath
}

// PopulateImageMetadata keeps unsupported formats usable while enriching standard images.
func (f *File) PopulateImageMetadata(sourcePath string) error {
	f.Width = 0
	f.Height = 0
	f.ThumbnailURL = ""

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	config, format, err := image.DecodeConfig(sourceFile)
	if err != nil {
		return nil
	}
	f.Width = config.Width
	f.Height = config.Height
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return errors.New("image dimensions exceed the safety limit")
	}
	if format != "jpeg" && format != "png" {
		return nil
	}
	if _, err := sourceFile.Seek(0, io.SeekStart); err != nil {
		return err
	}
	decoded, _, err := image.Decode(sourceFile)
	if err != nil {
		return nil
	}
	return f.createThumbnail(decoded)
}

func (f *File) createThumbnail(source image.Image) error {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil
	}
	thumbnailWidth, thumbnailHeight := width, height
	if width > thumbnailMaxDimension || height > thumbnailMaxDimension {
		if width >= height {
			thumbnailWidth = thumbnailMaxDimension
			thumbnailHeight = int(int64(height) * thumbnailMaxDimension / int64(width))
		} else {
			thumbnailHeight = thumbnailMaxDimension
			thumbnailWidth = int(int64(width) * thumbnailMaxDimension / int64(height))
		}
		if thumbnailWidth < 1 {
			thumbnailWidth = 1
		}
		if thumbnailHeight < 1 {
			thumbnailHeight = 1
		}
	}

	thumbnail := image.NewRGBA(image.Rect(0, 0, thumbnailWidth, thumbnailHeight))
	// ponytail: nearest-neighbor keeps this dependency-free; use a higher-quality resampler if quality becomes a requirement.
	for y := 0; y < thumbnailHeight; y++ {
		sourceY := bounds.Min.Y + int(int64(y)*int64(height)/int64(thumbnailHeight))
		for x := 0; x < thumbnailWidth; x++ {
			sourceX := bounds.Min.X + int(int64(x)*int64(width)/int64(thumbnailWidth))
			thumbnail.Set(x, y, source.At(sourceX, sourceY))
		}
	}

	if err := f.CreateDirIfNotExist(); err != nil {
		return err
	}
	thumbnailPath := f.GetThumbnailSavePath()
	thumbnailFile, err := os.Create(thumbnailPath)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(thumbnailFile, thumbnail, &jpeg.Options{Quality: 80}); err != nil {
		_ = thumbnailFile.Close()
		_ = os.Remove(thumbnailPath)
		return err
	}
	if err := thumbnailFile.Close(); err != nil {
		_ = os.Remove(thumbnailPath)
		return err
	}
	f.ThumbnailURL = f.GetThumbnailURL()
	return nil
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

func (f *File) HardDelete() error {
	savePath := f.GetActualSavePath()
	if savePath == "/" || savePath == "" {
		return nil
	}
	originalErr := safeRemoveStorageFile(savePath)
	thumbnailErr := safeRemoveStorageFile(f.GetThumbnailSavePath())
	if originalErr != nil && !os.IsNotExist(originalErr) {
		return originalErr
	}
	if thumbnailErr != nil && !os.IsNotExist(thumbnailErr) {
		return thumbnailErr
	}
	return nil
}

func safeRemoveStorageFile(filePath string) error {
	if !isStoragePath(filePath) {
		return nil
	}
	return os.Remove(filePath)
}

func isStoragePath(filePath string) bool {
	root, err := filepath.Abs(static_config.StorageRootPath)
	if err != nil {
		return false
	}
	target, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(relative) || relative == ".." {
		return false
	}
	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
