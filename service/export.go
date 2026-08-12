package service

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
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
	Email          string    `json:"email,omitempty"`
	LastLogin      time.Time `json:"last_login"`
	PermissionType int8      `json:"permission_type"`
	Password       string    `json:"password,omitempty"`
	Token          string    `json:"token,omitempty"`
}

type exportArticle struct {
	model.BaseModel
	UID            string                     `json:"uid"`
	AuthorID       uint                       `json:"author_id"`
	Title          string                     `json:"title"`
	Slug           string                     `json:"slug"`
	SEOTitle       string                     `json:"seo_title"`
	SEODescription string                     `json:"seo_description"`
	ShareImage     string                     `json:"share_image"`
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
	Password       string                     `json:"password,omitempty"`
}

type exportComment struct {
	model.BaseModel
	ArticleUID  string    `json:"article_uid"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email,omitempty"`
	AuthorIP    string    `json:"author_ip,omitempty"`
	AuthorURL   string    `json:"author_url"`
	AuthorAgent string    `json:"author_agent,omitempty"`
	Content     string    `json:"content"`
	Status      *int8     `json:"status"`
	LikeCount   int       `json:"like_count"`
	ParentID    uint      `json:"parent_id"`
	UserID      uint      `json:"user_id"`
	PostTime    time.Time `json:"post_time"`
}

type exportSetting struct {
	model.BaseModel
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Description string `json:"description"`
	IsSystem    bool   `json:"is_system"`
	IsPublic    *bool  `json:"is_public"`
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
	Version           int                         `json:"version"`
	ExportedAt        time.Time                   `json:"exported_at"`
	IncludesSensitive bool                        `json:"includes_sensitive,omitempty"`
	Users             []exportUser                `json:"users"`
	Articles          []exportArticle             `json:"articles"`
	ArticleDetails    []model.ArticleDetail       `json:"article_details"`
	Revisions         []model.ArticleRevision     `json:"article_revisions"`
	SlugRedirects     []model.ArticleSlugRedirect `json:"article_slug_redirects"`
	DailyStats        []model.ArticleDailyStat    `json:"article_daily_stats"`
	Comments          []exportComment             `json:"comments"`
	Categories        []model.Category            `json:"categories"`
	Tags              []model.Tag                 `json:"tags"`
	ArticleTags       []exportArticleTag          `json:"article_tags"`
	Links             []model.Link                `json:"links"`
	Files             []model.File                `json:"files"`
	Themes            []exportTheme               `json:"themes"`
	Settings          []exportSetting             `json:"settings"`
}

func CreateExportArchive() (archivePath string, size int64, err error) {
	return createExportArchive(false)
}

func createExportArchive(includeSensitive bool) (archivePath string, size int64, err error) {
	data, err := loadExportDataWithSensitive(includeSensitive)
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
	return loadExportDataWithSensitive(false)
}

func loadExportDataWithSensitive(includeSensitive bool) (exportData, error) {
	data := exportData{Version: 1, ExportedAt: time.Now(), IncludesSensitive: includeSensitive}
	queries := []struct {
		model interface{}
		dest  interface{}
	}{
		{&model.User{}, &data.Users},
		{&model.Article{}, &data.Articles},
		{&model.ArticleDetail{}, &data.ArticleDetails},
		{&model.ArticleRevision{}, &data.Revisions},
		{&model.ArticleSlugRedirect{}, &data.SlugRedirects},
		{&model.ArticleDailyStat{}, &data.DailyStats},
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
		if includeSensitive || !IsSensitiveSettingKey(setting.Key) {
			data.Settings = append(data.Settings, setting)
		}
	}
	normalizeExportRelations(&data)
	if !includeSensitive {
		for i := range data.Users {
			data.Users[i].Email = ""
			data.Users[i].Password = ""
			data.Users[i].Token = ""
		}
		for i := range data.Articles {
			data.Articles[i].Password = ""
		}
		for i := range data.Comments {
			data.Comments[i].AuthorEmail = ""
			data.Comments[i].AuthorIP = ""
			data.Comments[i].AuthorAgent = ""
		}
	}
	return data, nil
}

func normalizeExportRelations(data *exportData) {
	userIDs := make(map[uint]struct{}, len(data.Users))
	for _, user := range data.Users {
		userIDs[user.ID] = struct{}{}
	}
	categoryIndexes := make(map[string]int, len(data.Categories))
	for i := range data.Categories {
		categoryIndexes[data.Categories[i].UID] = i
	}
	for i := range data.Categories {
		category := &data.Categories[i]
		if parentIndex, exists := categoryIndexes[category.FatherUID]; category.FatherUID != "" &&
			(!exists || data.Categories[parentIndex].Type != category.Type) {
			category.FatherUID = ""
		}
	}
	for i := range data.Categories {
		seen := map[string]struct{}{data.Categories[i].UID: {}}
		for parentUID := data.Categories[i].FatherUID; parentUID != ""; parentUID = data.Categories[categoryIndexes[parentUID]].FatherUID {
			if _, exists := seen[parentUID]; exists {
				data.Categories[i].FatherUID = ""
				break
			}
			seen[parentUID] = struct{}{}
		}
	}

	articleIDs := make(map[uint]struct{}, len(data.Articles))
	articleUIDs := make(map[string]struct{}, len(data.Articles))
	detailUIDs := make(map[string]struct{}, len(data.Articles))
	for i := range data.Articles {
		article := &data.Articles[i]
		articleIDs[article.ID] = struct{}{}
		articleUIDs[article.UID] = struct{}{}
		detailUIDs[article.DetailUID] = struct{}{}
		if _, exists := userIDs[article.AuthorID]; article.AuthorID != 0 && !exists {
			article.AuthorID = 0
		}
		if categoryIndex, exists := categoryIndexes[article.CategoryUID]; article.CategoryUID != "" &&
			(!exists || data.Categories[categoryIndex].Type != model.CategoryTypeArticle) {
			article.CategoryUID = ""
		}
	}
	data.ArticleDetails = slices.DeleteFunc(data.ArticleDetails, func(detail model.ArticleDetail) bool {
		_, exists := detailUIDs[detail.UID]
		return !exists
	})
	data.Revisions = slices.DeleteFunc(data.Revisions, func(revision model.ArticleRevision) bool {
		_, exists := articleIDs[revision.ArticleID]
		return !exists
	})
	data.SlugRedirects = slices.DeleteFunc(data.SlugRedirects, func(redirect model.ArticleSlugRedirect) bool {
		_, exists := articleUIDs[redirect.ArticleUID]
		return !exists
	})
	data.DailyStats = slices.DeleteFunc(data.DailyStats, func(stat model.ArticleDailyStat) bool {
		_, exists := articleUIDs[stat.ArticleUID]
		return !exists
	})
	data.Comments = slices.DeleteFunc(data.Comments, func(comment exportComment) bool {
		_, exists := articleUIDs[comment.ArticleUID]
		return !exists
	})
	commentIDs := make(map[uint]struct{}, len(data.Comments))
	for _, comment := range data.Comments {
		commentIDs[comment.ID] = struct{}{}
	}
	for i := range data.Comments {
		if _, exists := userIDs[data.Comments[i].UserID]; data.Comments[i].UserID != 0 && !exists {
			data.Comments[i].UserID = 0
		}
		if _, exists := commentIDs[data.Comments[i].ParentID]; data.Comments[i].ParentID != 0 && !exists {
			data.Comments[i].ParentID = 0
		}
	}
	tagUIDs := make(map[string]struct{}, len(data.Tags))
	for _, tag := range data.Tags {
		tagUIDs[tag.UID] = struct{}{}
	}
	data.ArticleTags = slices.DeleteFunc(data.ArticleTags, func(relation exportArticleTag) bool {
		_, articleExists := articleUIDs[relation.ArticleUID]
		_, tagExists := tagUIDs[relation.TagUID]
		return !articleExists || !tagExists
	})
	for i := range data.Links {
		if categoryIndex, exists := categoryIndexes[data.Links[i].CategoryUID]; data.Links[i].CategoryUID != "" &&
			(!exists || data.Categories[categoryIndex].Type != model.CategoryTypeLink) {
			data.Links[i].CategoryUID = ""
		}
	}
}

func IsSensitiveSettingKey(key string) bool {
	if IsHiddenSettingKey(key) {
		return true
	}
	if key == "CommentNotificationWebhook" {
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
