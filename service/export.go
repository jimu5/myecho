package service

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"myecho/config/static_config"
	"myecho/dal/connect"
	"myecho/model"
)

const (
	maxExportBytes     int64 = 512 << 20
	maxExportFileBytes int64 = 256 << 20
)

var (
	ErrExportTooLarge   = errors.New("export exceeds the 512 MiB safety limit")
	errUnsafeExportPath = errors.New("unsafe storage path")
)

type exportUser struct {
	model.BaseModel
	Name           string    `json:"name"`
	NickName       string    `json:"nick_name"`
	LastLogin      time.Time `json:"last_login"`
	PermissionType int8      `json:"permission_type"`
}

type exportArticle struct {
	model.BaseModel
	UID            string                     `json:"uid"`
	AuthorID       uint                       `json:"author_id"`
	Title          string                     `json:"title"`
	Slug           string                     `json:"slug"`
	Type           model.ArticleType          `json:"type"`
	ContentFormat  model.ArticleContentFormat `json:"content_format"`
	Summary        string                     `json:"summary"`
	ReadCount      uint                       `json:"read_count"`
	LikeCount      int                        `json:"like_count"`
	IsAllowComment *bool                      `json:"is_allow_comment"`
	CommentCount   uint                       `json:"comment_count"`
	CategoryUID    string                     `json:"category_uid"`
	DetailUID      string                     `json:"detail_uid"`
	PostTime       time.Time                  `json:"post_time"`
	Status         int8                       `json:"status"`
}

type exportComment struct {
	model.BaseModel
	ArticleUID string    `json:"article_uid"`
	AuthorName string    `json:"author_name"`
	AuthorURL  string    `json:"author_url"`
	Content    string    `json:"content"`
	Status     *int8     `json:"status"`
	LikeCount  int       `json:"like_count"`
	ParentID   uint      `json:"parent_id"`
	UserID     uint      `json:"user_id"`
	PostTime   time.Time `json:"post_time"`
}

type exportSetting struct {
	model.BaseModel
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Description string `json:"description"`
	IsSystem    bool   `json:"is_system"`
}

type exportTheme struct {
	model.BaseModel
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Author       string `json:"author"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	Preview      string `json:"preview"`
	CSS          string `json:"css"`
	JS           string `json:"js"`
	IsDefault    bool   `json:"is_default"`
	IsActive     bool   `json:"is_active"`
	HasTemplates bool   `json:"has_templates"`
	Config       []byte `json:"config"`
	ConfigSchema []byte `json:"config_schema"`
}

type exportArticleTag struct {
	ArticleUID string `json:"article_uid"`
	TagUID     string `json:"tag_uid"`
}

type exportData struct {
	ExportedAt     time.Time             `json:"exported_at"`
	Users          []exportUser          `json:"users"`
	Articles       []exportArticle       `json:"articles"`
	ArticleDetails []model.ArticleDetail `json:"article_details"`
	Comments       []exportComment       `json:"comments"`
	Categories     []model.Category      `json:"categories"`
	Tags           []model.Tag           `json:"tags"`
	ArticleTags    []exportArticleTag    `json:"article_tags"`
	Links          []model.Link          `json:"links"`
	Files          []model.File          `json:"files"`
	Themes         []exportTheme         `json:"themes"`
	Settings       []exportSetting       `json:"settings"`
}

func CreateExportArchive() (archivePath string, size int64, err error) {
	data, err := loadExportData()
	if err != nil {
		return "", 0, err
	}
	file, err := os.CreateTemp("", "myecho-export-*.zip")
	if err != nil {
		return "", 0, err
	}
	archivePath = file.Name()
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(archivePath)
			archivePath = ""
			size = 0
		}
	}()

	zw := zip.NewWriter(file)
	remaining := maxExportBytes
	jsonEntry, err := zw.CreateHeader(&zip.FileHeader{
		Name:   "data.json",
		Method: zip.Deflate,
	})
	if err == nil {
		writer := &exportLimitWriter{Writer: jsonEntry, Remaining: &remaining}
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(data)
	}
	if err == nil {
		err = addStorageToArchive(zw, &remaining)
	}
	if closeErr := zw.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		var info os.FileInfo
		info, err = file.Stat()
		if err == nil {
			size = info.Size()
			if size > maxExportBytes {
				err = ErrExportTooLarge
			}
		}
	}
	return archivePath, size, err
}

func loadExportData() (exportData, error) {
	data := exportData{ExportedAt: time.Now()}
	queries := []struct {
		model interface{}
		dest  interface{}
	}{
		{&model.User{}, &data.Users},
		{&model.Article{}, &data.Articles},
		{&model.ArticleDetail{}, &data.ArticleDetails},
		{&model.Comment{}, &data.Comments},
		{&model.Category{}, &data.Categories},
		{&model.Tag{}, &data.Tags},
		{&model.Link{}, &data.Links},
		{&model.File{}, &data.Files},
		{&model.Theme{}, &data.Themes},
	}
	for _, query := range queries {
		if err := connect.Database.Model(query.model).Find(query.dest).Error; err != nil {
			return exportData{}, err
		}
	}
	if err := connect.Database.Table("article_tags").Find(&data.ArticleTags).Error; err != nil {
		return exportData{}, err
	}
	var settings []exportSetting
	if err := connect.Database.Model(&model.Setting{}).Find(&settings).Error; err != nil {
		return exportData{}, err
	}
	for _, setting := range settings {
		if !isSensitiveSettingKey(setting.Key) {
			data.Settings = append(data.Settings, setting)
		}
	}
	return data, nil
}

func isSensitiveSettingKey(key string) bool {
	if IsHiddenSettingKey(key) {
		return true
	}
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(key))
	for _, marker := range []string{"password", "secret", "token", "apikey", "credential", "privatekey"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func addStorageToArchive(zw *zip.Writer, remaining *int64) error {
	root, err := filepath.Abs(static_config.StorageRootPath)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(root, filePath)
		if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) || filepath.IsAbs(relativePath) {
			return errUnsafeExportPath
		}
		if relativePath == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if relativePath == "temp" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(entry.Name(), "config.yaml") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if info.Size() > maxExportFileBytes || info.Size() > *remaining {
			return ErrExportTooLarge
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = path.Join("storage", filepath.ToSlash(relativePath))
		header.Method = zip.Deflate
		target, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		source, err := os.Open(filePath)
		if err != nil {
			return err
		}
		openedInfo, statErr := source.Stat()
		if statErr != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
			_ = source.Close()
			return errUnsafeExportPath
		}
		written, copyErr := io.Copy(
			&exportLimitWriter{Writer: target, Remaining: remaining},
			io.LimitReader(source, maxExportFileBytes+1),
		)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if written > maxExportFileBytes {
			return ErrExportTooLarge
		}
		return closeErr
	})
}

type exportLimitWriter struct {
	io.Writer
	Remaining *int64
}

func (w *exportLimitWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > *w.Remaining {
		return 0, ErrExportTooLarge
	}
	n, err := w.Writer.Write(data)
	*w.Remaining -= int64(n)
	return n, err
}
