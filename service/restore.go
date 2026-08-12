package service

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"myecho/config"
	"myecho/config/static_config"
	"myecho/dal/cache"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

const maxRestoreRecords = 1_000_000

var (
	ErrInvalidBackup   = errors.New("invalid backup archive")
	ErrRestoreTooLarge = errors.New("backup exceeds the 512 MiB safety limit")
	restoreMu          sync.Mutex
)

type RestoreCounts struct {
	Users          int `json:"users"`
	Articles       int `json:"articles"`
	ArticleDetails int `json:"article_details"`
	Revisions      int `json:"article_revisions"`
	SlugRedirects  int `json:"article_slug_redirects"`
	DailyStats     int `json:"article_daily_stats"`
	Comments       int `json:"comments"`
	Categories     int `json:"categories"`
	Tags           int `json:"tags"`
	Links          int `json:"links"`
	Files          int `json:"files"`
	Themes         int `json:"themes"`
	Settings       int `json:"settings"`
}

type RestorePreview struct {
	Version      int           `json:"version"`
	ExportedAt   time.Time     `json:"exported_at"`
	Counts       RestoreCounts `json:"counts"`
	StorageFiles int           `json:"storage_files"`
	StorageBytes int64         `json:"storage_bytes"`
}

type RestoreResult struct {
	RestorePreview
	BackupPath     string `json:"backup_path"`
	CleanupWarning string `json:"cleanup_warning,omitempty"`
}

func PreviewRestoreArchive(reader io.ReaderAt, size int64) (RestorePreview, error) {
	_, preview, err := readRestoreArchive(reader, size, "")
	return preview, err
}

func RestoreArchive(reader io.ReaderAt, size int64, currentUserID uint) (RestoreResult, error) {
	restoreMu.Lock()
	defer restoreMu.Unlock()

	storageRoot, err := safeStorageRoot()
	if err != nil {
		return RestoreResult{}, err
	}
	parent := filepath.Dir(storageRoot)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return RestoreResult{}, err
	}
	stageRoot, err := os.MkdirTemp(parent, ".myecho-restore-")
	if err != nil {
		return RestoreResult{}, err
	}
	defer os.RemoveAll(stageRoot)

	data, preview, err := readRestoreArchive(reader, size, stageRoot)
	if err != nil {
		return RestoreResult{}, err
	}
	backupPath, err := createPersistentRestorePoint(parent)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("create restore point: %w", err)
	}

	tx := connect.Database.Begin()
	if tx.Error != nil {
		return RestoreResult{}, tx.Error
	}
	if err := replaceRestoreData(tx, data, currentUserID); err != nil {
		_ = tx.Rollback().Error
		return RestoreResult{}, err
	}

	oldRoot, hadOldRoot, err := swapStorageRoot(storageRoot, stageRoot)
	if err != nil {
		_ = tx.Rollback().Error
		return RestoreResult{}, err
	}
	if err := tx.Commit().Error; err != nil {
		rollbackErr := rollbackStorageRoot(storageRoot, oldRoot, hadOldRoot)
		if rollbackErr != nil {
			return RestoreResult{}, fmt.Errorf("commit restore: %w; storage rollback: %v", err, rollbackErr)
		}
		return RestoreResult{}, err
	}

	result := RestoreResult{RestorePreview: preview, BackupPath: backupPath}
	if hadOldRoot {
		if err := os.RemoveAll(oldRoot); err != nil {
			result.CleanupWarning = err.Error()
		}
	}
	config.MySqlSettingModelCache = cache.InitSettingCache()
	return result, nil
}

func readRestoreArchive(reader io.ReaderAt, size int64, extractRoot string) (exportData, RestorePreview, error) {
	if size <= 0 {
		return exportData{}, RestorePreview{}, fmt.Errorf("%w: empty archive", ErrInvalidBackup)
	}
	if size > maxExportBytes {
		return exportData{}, RestorePreview{}, ErrRestoreTooLarge
	}
	zr, err := zip.NewReader(reader, size)
	if err != nil {
		return exportData{}, RestorePreview{}, fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}

	var (
		data         exportData
		dataFound    bool
		totalBytes   int64
		storageBytes int64
		storageFiles int
		seen         = make(map[string]struct{}, len(zr.File))
	)
	for _, file := range zr.File {
		name, err := validateRestoreEntry(file)
		if err != nil {
			return exportData{}, RestorePreview{}, err
		}
		if _, exists := seen[name]; exists {
			return exportData{}, RestorePreview{}, fmt.Errorf("%w: duplicate entry %q", ErrInvalidBackup, name)
		}
		seen[name] = struct{}{}

		uncompressed := int64(file.UncompressedSize64)
		if uncompressed > maxExportFileBytes {
			return exportData{}, RestorePreview{}, ErrRestoreTooLarge
		}
		totalBytes += uncompressed
		if totalBytes > maxExportBytes {
			return exportData{}, RestorePreview{}, ErrRestoreTooLarge
		}
		if file.FileInfo().IsDir() {
			continue
		}

		source, err := file.Open()
		if err != nil {
			return exportData{}, RestorePreview{}, fmt.Errorf("%w: %v", ErrInvalidBackup, err)
		}
		if name == "data.json" {
			if dataFound {
				_ = source.Close()
				return exportData{}, RestorePreview{}, fmt.Errorf("%w: duplicate data.json", ErrInvalidBackup)
			}
			decoder := json.NewDecoder(io.LimitReader(source, maxExportFileBytes+1))
			decoder.DisallowUnknownFields()
			err = decoder.Decode(&data)
			if err == nil {
				var trailing interface{}
				if trailingErr := decoder.Decode(&trailing); trailingErr != io.EOF {
					err = errors.New("data.json contains trailing data")
				}
			}
			dataFound = true
		} else {
			storageFiles++
			storageBytes += uncompressed
			if extractRoot != "" {
				err = extractRestoreFile(source, extractRoot, strings.TrimPrefix(name, "storage/"))
			} else {
				var written int64
				written, err = io.Copy(io.Discard, io.LimitReader(source, maxExportFileBytes+1))
				if err == nil && written > maxExportFileBytes {
					err = ErrRestoreTooLarge
				}
			}
		}
		closeErr := source.Close()
		if err != nil {
			return exportData{}, RestorePreview{}, fmt.Errorf("%w: %v", ErrInvalidBackup, err)
		}
		if closeErr != nil {
			return exportData{}, RestorePreview{}, closeErr
		}
	}
	if !dataFound {
		return exportData{}, RestorePreview{}, fmt.Errorf("%w: data.json is required", ErrInvalidBackup)
	}
	if err := validateRestoreData(data); err != nil {
		return exportData{}, RestorePreview{}, err
	}
	return data, restorePreview(data, storageFiles, storageBytes), nil
}

func validateRestoreEntry(file *zip.File) (string, error) {
	name := file.Name
	if name == "" || strings.ContainsRune(name, 0) || strings.Contains(name, "\\") ||
		strings.HasPrefix(name, "/") || path.Clean(name) != strings.TrimSuffix(name, "/") {
		return "", fmt.Errorf("%w: unsafe entry path %q", ErrInvalidBackup, name)
	}
	if file.Flags&1 != 0 || file.Mode()&os.ModeSymlink != 0 || (!file.FileInfo().IsDir() && !file.Mode().IsRegular()) {
		return "", fmt.Errorf("%w: unsupported entry %q", ErrInvalidBackup, name)
	}
	if name != "data.json" && !strings.HasPrefix(name, "storage/") {
		return "", fmt.Errorf("%w: unsupported entry %q", ErrInvalidBackup, name)
	}
	if name == "storage/" && !file.FileInfo().IsDir() {
		return "", fmt.Errorf("%w: storage must be a directory", ErrInvalidBackup)
	}
	return strings.TrimSuffix(name, "/"), nil
}

func extractRestoreFile(source io.Reader, root, relative string) error {
	if relative == "" {
		return fmt.Errorf("%w: empty storage path", ErrInvalidBackup)
	}
	target := filepath.Join(root, filepath.FromSlash(relative))
	if !isPathInside(root, target) {
		return fmt.Errorf("%w: unsafe storage path", ErrInvalidBackup)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(output, io.LimitReader(source, maxExportFileBytes+1))
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if written > maxExportFileBytes {
		return ErrRestoreTooLarge
	}
	return closeErr
}

func validateRestoreData(data exportData) error {
	if data.Version != 1 || data.ExportedAt.IsZero() {
		return fmt.Errorf("%w: unsupported or incomplete data.json", ErrInvalidBackup)
	}
	total := len(data.Users) + len(data.Articles) + len(data.ArticleDetails) + len(data.Revisions) +
		len(data.SlugRedirects) + len(data.DailyStats) + len(data.Comments) +
		len(data.Categories) + len(data.Tags) + len(data.ArticleTags) + len(data.Links) +
		len(data.Files) + len(data.Themes) + len(data.Settings)
	if total > maxRestoreRecords {
		return fmt.Errorf("%w: too many records", ErrInvalidBackup)
	}
	if duplicateUintIDs(data.Users, func(item exportUser) uint { return item.ID }) ||
		duplicateUintIDs(data.Articles, func(item exportArticle) uint { return item.ID }) ||
		duplicateUintIDs(data.ArticleDetails, func(item model.ArticleDetail) uint { return item.ID }) ||
		duplicateUintIDs(data.Revisions, func(item model.ArticleRevision) uint { return item.ID }) ||
		duplicateUintIDs(data.SlugRedirects, func(item model.ArticleSlugRedirect) uint { return item.ID }) ||
		duplicateUintIDs(data.DailyStats, func(item model.ArticleDailyStat) uint { return item.ID }) ||
		duplicateUintIDs(data.Comments, func(item exportComment) uint { return item.ID }) ||
		duplicateUintIDs(data.Categories, func(item model.Category) uint { return item.ID }) ||
		duplicateUintIDs(data.Tags, func(item model.Tag) uint { return item.ID }) ||
		duplicateUintIDs(data.Links, func(item model.Link) uint { return item.ID }) ||
		duplicateUintIDs(data.Files, func(item model.File) uint { return item.ID }) ||
		duplicateUintIDs(data.Themes, func(item exportTheme) uint { return item.ID }) ||
		duplicateUintIDs(data.Settings, func(item exportSetting) uint { return item.ID }) {
		return fmt.Errorf("%w: duplicate record id", ErrInvalidBackup)
	}

	userIDs := make(map[uint]struct{}, len(data.Users))
	for _, user := range data.Users {
		userIDs[user.ID] = struct{}{}
	}
	detailUIDs := make(map[string]struct{}, len(data.ArticleDetails))
	for _, detail := range data.ArticleDetails {
		if detail.UID == "" {
			return invalidRestoreData("empty article detail uid")
		}
		if _, exists := detailUIDs[detail.UID]; exists {
			return invalidRestoreData("duplicate article detail uid")
		}
		detailUIDs[detail.UID] = struct{}{}
	}
	categoryUIDs := make(map[string]model.CategoryType, len(data.Categories))
	for _, category := range data.Categories {
		if category.UID == "" || !category.Type.IsCategoryTypeValid() {
			return invalidRestoreData("invalid category")
		}
		if _, exists := categoryUIDs[category.UID]; exists {
			return invalidRestoreData("duplicate category uid")
		}
		categoryUIDs[category.UID] = category.Type
	}
	if err := validateRestoreCategoryParents(data.Categories); err != nil {
		return err
	}
	tagUIDs := make(map[string]struct{}, len(data.Tags))
	for _, tag := range data.Tags {
		if tag.UID == "" {
			return invalidRestoreData("empty tag uid")
		}
		if _, exists := tagUIDs[tag.UID]; exists {
			return invalidRestoreData("duplicate tag uid")
		}
		tagUIDs[tag.UID] = struct{}{}
	}
	articleIDs := make(map[uint]struct{}, len(data.Articles))
	articleUIDs := make(map[string]struct{}, len(data.Articles))
	articleSlugs := make(map[struct {
		Slug string
		Type model.ArticleType
	}]struct{}, len(data.Articles))
	for _, article := range data.Articles {
		if article.UID == "" || strings.TrimSpace(article.Slug) == "" || !model.IsValidArticleType(article.Type) ||
			!model.IsValidArticleContentFormat(article.ContentFormat) ||
			article.Status < int8(mysql.ARTILCE_STATUS_PUBLIC) || article.Status > int8(mysql.ARTICLE_STATUS_RECYCLE) {
			return invalidRestoreData("invalid article")
		}
		if _, exists := articleUIDs[article.UID]; exists {
			return invalidRestoreData("duplicate article uid")
		}
		if article.AuthorID != 0 {
			if _, exists := userIDs[article.AuthorID]; !exists {
				return invalidRestoreData("article references missing user")
			}
		}
		if _, exists := detailUIDs[article.DetailUID]; !exists {
			return invalidRestoreData("article references missing detail")
		}
		if article.CategoryUID != "" {
			if categoryUIDs[article.CategoryUID] != model.CategoryTypeArticle {
				return invalidRestoreData("article references invalid category")
			}
		}
		slugKey := struct {
			Slug string
			Type model.ArticleType
		}{Slug: article.Slug, Type: article.Type}
		if _, exists := articleSlugs[slugKey]; exists {
			return invalidRestoreData("duplicate article slug")
		}
		articleIDs[article.ID] = struct{}{}
		articleUIDs[article.UID] = struct{}{}
		articleSlugs[slugKey] = struct{}{}
	}
	for _, revision := range data.Revisions {
		if _, exists := articleIDs[revision.ArticleID]; !exists {
			return invalidRestoreData("revision references missing article")
		}
	}
	redirectKeys := make(map[struct {
		Slug string
		Type model.ArticleType
	}]struct{}, len(data.SlugRedirects))
	for _, redirect := range data.SlugRedirects {
		if redirect.Slug == "" || !model.IsValidArticleType(redirect.Type) {
			return invalidRestoreData("invalid article redirect")
		}
		if _, exists := articleUIDs[redirect.ArticleUID]; !exists {
			return invalidRestoreData("redirect references missing article")
		}
		key := struct {
			Slug string
			Type model.ArticleType
		}{Slug: redirect.Slug, Type: redirect.Type}
		if _, exists := articleSlugs[key]; exists {
			return invalidRestoreData("redirect conflicts with article slug")
		}
		if _, exists := redirectKeys[key]; exists {
			return invalidRestoreData("duplicate article redirect")
		}
		redirectKeys[key] = struct{}{}
	}
	statKeys := make(map[struct {
		ArticleUID string
		Date       string
	}]struct{}, len(data.DailyStats))
	for _, stat := range data.DailyStats {
		if _, exists := articleUIDs[stat.ArticleUID]; !exists {
			return invalidRestoreData("daily stat references missing article")
		}
		if parsed, err := time.Parse("2006-01-02", stat.Date); err != nil || parsed.Format("2006-01-02") != stat.Date {
			return invalidRestoreData("invalid daily stat date")
		}
		key := struct {
			ArticleUID string
			Date       string
		}{ArticleUID: stat.ArticleUID, Date: stat.Date}
		if _, exists := statKeys[key]; exists {
			return invalidRestoreData("duplicate daily stat")
		}
		statKeys[key] = struct{}{}
	}
	commentByID := make(map[uint]exportComment, len(data.Comments))
	for _, comment := range data.Comments {
		if comment.Status != nil && *comment.Status != int8(model.CommentStatusLegacyApproved) &&
			!model.IsValidCommentStatus(model.CommentStatus(*comment.Status)) {
			return invalidRestoreData("invalid comment status")
		}
		if _, exists := articleUIDs[comment.ArticleUID]; !exists {
			return invalidRestoreData("comment references missing article")
		}
		if comment.UserID != 0 {
			if _, exists := userIDs[comment.UserID]; !exists {
				return invalidRestoreData("comment references missing user")
			}
		}
		commentByID[comment.ID] = comment
	}
	if err := validateRestoreCommentParents(commentByID); err != nil {
		return err
	}
	articleTagKeys := make(map[exportArticleTag]struct{}, len(data.ArticleTags))
	for _, relation := range data.ArticleTags {
		if _, exists := articleUIDs[relation.ArticleUID]; !exists {
			return invalidRestoreData("article tag references missing article")
		}
		if _, exists := tagUIDs[relation.TagUID]; !exists {
			return invalidRestoreData("article tag references missing tag")
		}
		if _, exists := articleTagKeys[relation]; exists {
			return invalidRestoreData("duplicate article tag")
		}
		articleTagKeys[relation] = struct{}{}
	}
	for _, link := range data.Links {
		if link.CategoryUID != "" && categoryUIDs[link.CategoryUID] != model.CategoryTypeLink {
			return invalidRestoreData("link references invalid category")
		}
	}

	themeNames := make(map[string]struct{}, len(data.Themes))
	for _, theme := range data.Themes {
		if strings.TrimSpace(theme.Name) == "" {
			return invalidRestoreData("invalid theme name")
		}
		if _, exists := themeNames[theme.Name]; exists {
			return invalidRestoreData("duplicate theme name")
		}
		themeNames[theme.Name] = struct{}{}
	}
	settingKeys := make(map[string]struct{}, len(data.Settings))
	for _, setting := range data.Settings {
		if strings.TrimSpace(setting.Key) == "" ||
			(IsSensitiveSettingKey(setting.Key) && !data.IncludesSensitive) ||
			(IsSensitiveSettingKey(setting.Key) && setting.IsPublic != nil && *setting.IsPublic) {
			return fmt.Errorf("%w: invalid setting key", ErrInvalidBackup)
		}
		if _, exists := settingKeys[setting.Key]; exists {
			return fmt.Errorf("%w: duplicate setting key", ErrInvalidBackup)
		}
		settingKeys[setting.Key] = struct{}{}
	}
	return nil
}

func invalidRestoreData(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidBackup, reason)
}

func validateRestoreCategoryParents(categories []model.Category) error {
	byUID := make(map[string]model.Category, len(categories))
	for _, category := range categories {
		byUID[category.UID] = category
	}
	for _, category := range categories {
		if category.FatherUID == "" {
			continue
		}
		parent, exists := byUID[category.FatherUID]
		if !exists || parent.Type != category.Type {
			return invalidRestoreData("category references invalid parent")
		}
	}
	states := make(map[string]uint8, len(categories))
	for uid := range byUID {
		if states[uid] != 0 {
			continue
		}
		path := make([]string, 0, 4)
		current := uid
		for current != "" && states[current] == 0 {
			states[current] = 1
			path = append(path, current)
			current = byUID[current].FatherUID
		}
		if current != "" && states[current] == 1 {
			return invalidRestoreData("cyclic category parents")
		}
		for _, pathUID := range path {
			states[pathUID] = 2
		}
	}
	return nil
}

func validateRestoreCommentParents(comments map[uint]exportComment) error {
	for _, comment := range comments {
		if comment.ParentID == 0 {
			continue
		}
		parent, exists := comments[comment.ParentID]
		if !exists || parent.ArticleUID != comment.ArticleUID {
			return invalidRestoreData("comment references invalid parent")
		}
	}
	states := make(map[uint]uint8, len(comments))
	for id := range comments {
		if states[id] != 0 {
			continue
		}
		path := make([]uint, 0, 4)
		current := id
		for current != 0 && states[current] == 0 {
			states[current] = 1
			path = append(path, current)
			current = comments[current].ParentID
		}
		if current != 0 && states[current] == 1 {
			return invalidRestoreData("cyclic comment parents")
		}
		for _, pathID := range path {
			states[pathID] = 2
		}
	}
	return nil
}

func duplicateUintIDs[T any](items []T, id func(T) uint) bool {
	seen := make(map[uint]struct{}, len(items))
	for _, item := range items {
		value := id(item)
		if value == 0 {
			return true
		}
		if _, exists := seen[value]; exists {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func restoreSettingVisibility(key string, value *bool) *bool {
	isPublic := false
	if value != nil {
		isPublic = *value
	} else {
		isPublic = IsSettingPublic(&mysql.SettingModel{Key: key})
	}
	if IsSensitiveSettingKey(key) {
		isPublic = false
	}
	return &isPublic
}

func restorePreview(data exportData, storageFiles int, storageBytes int64) RestorePreview {
	return RestorePreview{
		Version:    data.Version,
		ExportedAt: data.ExportedAt,
		Counts: RestoreCounts{
			Users:          len(data.Users),
			Articles:       len(data.Articles),
			ArticleDetails: len(data.ArticleDetails),
			Revisions:      len(data.Revisions),
			SlugRedirects:  len(data.SlugRedirects),
			DailyStats:     len(data.DailyStats),
			Comments:       len(data.Comments),
			Categories:     len(data.Categories),
			Tags:           len(data.Tags),
			Links:          len(data.Links),
			Files:          len(data.Files),
			Themes:         len(data.Themes),
			Settings:       len(data.Settings),
		},
		StorageFiles: storageFiles,
		StorageBytes: storageBytes,
	}
}

func replaceRestoreData(tx *gorm.DB, data exportData, currentUserID uint) error {
	var currentUsers []model.User
	if err := tx.Find(&currentUsers).Error; err != nil {
		return err
	}
	currentUsersByID := make(map[uint]model.User, len(currentUsers))
	for _, user := range currentUsers {
		currentUsersByID[user.ID] = user
	}
	users := make([]model.User, 0, len(data.Users)+1)
	seenUsers := make(map[uint]struct{}, len(data.Users))
	for _, item := range data.Users {
		user := model.User{
			BaseModel:      item.BaseModel,
			Name:           item.Name,
			NickName:       item.NickName,
			Email:          item.Email,
			LastLogin:      item.LastLogin,
			PermissionType: item.PermissionType,
			Password:       item.Password,
			Token:          item.Token,
		}
		if current, exists := currentUsersByID[item.ID]; exists && (!data.IncludesSensitive || item.ID == currentUserID) {
			user.Email = current.Email
			user.Password = current.Password
			user.Token = current.Token
		}
		users = append(users, user)
		seenUsers[item.ID] = struct{}{}
	}
	if current, exists := currentUsersByID[currentUserID]; exists {
		if _, restored := seenUsers[currentUserID]; !restored {
			users = append(users, current)
		}
	}

	var currentArticles []model.Article
	if err := tx.Select("uid", "password").Find(&currentArticles).Error; err != nil {
		return err
	}
	passwords := make(map[string]string, len(currentArticles))
	for _, article := range currentArticles {
		passwords[article.UID] = article.Password
	}
	articles := make([]model.Article, 0, len(data.Articles))
	for _, item := range data.Articles {
		allowComment := item.IsAllowComment
		password := item.Password
		if !data.IncludesSensitive {
			password = passwords[item.UID]
		}
		articles = append(articles, model.Article{
			BaseModel:      item.BaseModel,
			UID:            item.UID,
			AuthorID:       item.AuthorID,
			Title:          item.Title,
			Slug:           item.Slug,
			SEOTitle:       item.SEOTitle,
			SEODescription: item.SEODescription,
			ShareImage:     item.ShareImage,
			Type:           item.Type,
			ContentFormat:  item.ContentFormat,
			Summary:        item.Summary,
			ReadCount:      item.ReadCount,
			LikeCount:      item.LikeCount,
			IsAllowComment: allowComment,
			CommentCount:   item.CommentCount,
			CategoryUID:    item.CategoryUID,
			DetailUID:      item.DetailUID,
			PostTime:       item.PostTime,
			Status:         item.Status,
			Password:       password,
		})
	}

	var currentComments []model.Comment
	if err := tx.Select("id", "author_email", "author_ip", "author_agent").Find(&currentComments).Error; err != nil {
		return err
	}
	commentPrivate := make(map[uint]model.Comment, len(currentComments))
	for _, comment := range currentComments {
		commentPrivate[comment.ID] = comment
	}
	comments := make([]model.Comment, 0, len(data.Comments))
	for _, item := range data.Comments {
		comment := model.Comment{
			BaseModel:   item.BaseModel,
			ArticleUID:  item.ArticleUID,
			AuthorName:  item.AuthorName,
			AuthorEmail: item.AuthorEmail,
			AuthorIP:    item.AuthorIP,
			AuthorUrl:   item.AuthorURL,
			AuthorAgent: item.AuthorAgent,
			Content:     item.Content,
			Status:      item.Status,
			LikeCount:   item.LikeCount,
			ParentID:    item.ParentID,
			UserID:      item.UserID,
			PostTime:    item.PostTime,
		}
		if current, exists := commentPrivate[item.ID]; exists && !data.IncludesSensitive {
			comment.AuthorEmail = current.AuthorEmail
			comment.AuthorIP = current.AuthorIP
			comment.AuthorAgent = current.AuthorAgent
		}
		comments = append(comments, comment)
	}

	var currentSettings []model.Setting
	if err := tx.Find(&currentSettings).Error; err != nil {
		return err
	}
	settings := make([]model.Setting, 0, len(data.Settings)+len(currentSettings))
	var maxSettingID uint
	for _, item := range data.Settings {
		if item.ID > maxSettingID {
			maxSettingID = item.ID
		}
		settings = append(settings, model.Setting{
			BaseModel:   item.BaseModel,
			Key:         item.Key,
			Value:       item.Value,
			Type:        item.Type,
			Description: item.Description,
			IsSystem:    item.IsSystem,
			IsPublic:    restoreSettingVisibility(item.Key, item.IsPublic),
		})
	}
	if !data.IncludesSensitive {
		for _, setting := range currentSettings {
			if IsSensitiveSettingKey(setting.Key) {
				maxSettingID++
				setting.ID = maxSettingID
				setting.IsPublic = restoreSettingVisibility(setting.Key, setting.IsPublic)
				settings = append(settings, setting)
			}
		}
	}

	deleteModels := []interface{}{
		&model.Comment{}, &model.ArticleRevision{}, &model.ArticleSlugRedirect{}, &model.ArticleDailyStat{},
		&model.Article{}, &model.ArticleDetail{}, &model.Tag{}, &model.Category{},
		&model.Link{}, &model.File{}, &model.Theme{}, &model.Setting{}, &model.User{},
	}
	session := tx.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true})
	if err := session.Exec("DELETE FROM article_tags").Error; err != nil {
		return err
	}
	for _, value := range deleteModels {
		if err := session.Unscoped().Delete(value).Error; err != nil {
			return err
		}
	}
	createValues := []interface{}{
		&users, &data.Categories, &data.Tags, &data.ArticleDetails, &articles,
		&data.Revisions, &data.SlugRedirects, &data.DailyStats, &comments,
		&data.Links, &data.Files, &data.Themes, &settings,
	}
	for _, value := range createValues {
		if isEmptyRestoreSlice(value) {
			continue
		}
		if err := session.Create(value).Error; err != nil {
			return err
		}
	}
	if len(data.ArticleTags) != 0 {
		if err := session.Table("article_tags").Create(&data.ArticleTags).Error; err != nil {
			return err
		}
	}
	return resetPostgresRestoreSequences(session)
}

func resetPostgresRestoreSequences(tx *gorm.DB) error {
	if tx.Dialector.Name() != "postgres" {
		return nil
	}
	for _, table := range []string{
		"users", "categories", "tags", "article_details", "articles", "article_revisions",
		"article_slug_redirects", "article_daily_stats", "comments", "links", "files", "themes", "settings",
	} {
		query := fmt.Sprintf(
			`SELECT setval(pg_get_serial_sequence('%s', 'id'), COALESCE(MAX(id), 1), MAX(id) IS NOT NULL) FROM "%s"`,
			table,
			table,
		)
		if err := tx.Exec(query).Error; err != nil {
			return err
		}
	}
	return nil
}

func isEmptyRestoreSlice(value interface{}) bool {
	switch items := value.(type) {
	case *[]model.User:
		return len(*items) == 0
	case *[]model.Category:
		return len(*items) == 0
	case *[]model.Tag:
		return len(*items) == 0
	case *[]model.ArticleDetail:
		return len(*items) == 0
	case *[]model.Article:
		return len(*items) == 0
	case *[]model.ArticleRevision:
		return len(*items) == 0
	case *[]model.ArticleSlugRedirect:
		return len(*items) == 0
	case *[]model.ArticleDailyStat:
		return len(*items) == 0
	case *[]model.Comment:
		return len(*items) == 0
	case *[]model.Link:
		return len(*items) == 0
	case *[]model.File:
		return len(*items) == 0
	case *[]exportTheme:
		return len(*items) == 0
	case *[]model.Setting:
		return len(*items) == 0
	default:
		return false
	}
}

func safeStorageRoot() (string, error) {
	root, err := filepath.Abs(static_config.StorageRootPath)
	if err != nil {
		return "", err
	}
	if root == string(filepath.Separator) || root == filepath.Dir(root) {
		return "", errUnsafeExportPath
	}
	if info, err := os.Lstat(root); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", errUnsafeExportPath
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return root, nil
}

func isPathInside(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func createPersistentRestorePoint(parent string) (string, error) {
	tempPath, _, err := createExportArchive(true)
	if err != nil {
		return "", err
	}
	defer os.Remove(tempPath)
	backupDir := filepath.Join(parent, ".myecho-backups")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return "", err
	}
	target := filepath.Join(backupDir, "before-restore-"+time.Now().Format("20060102-150405.000000000")+".zip")
	source, err := os.Open(tempPath)
	if err != nil {
		return "", err
	}
	defer source.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(output, source)
	if copyErr == nil {
		copyErr = output.Sync()
	}
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return "", closeErr
	}
	return target, nil
}

func swapStorageRoot(root, stage string) (oldRoot string, hadOld bool, err error) {
	oldRoot = filepath.Join(filepath.Dir(root), ".myecho-storage-"+time.Now().Format("20060102-150405.000000000"))
	if info, statErr := os.Lstat(root); statErr == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", false, errUnsafeExportPath
		}
		if err := os.Rename(root, oldRoot); err != nil {
			return "", false, err
		}
		hadOld = true
	} else if !os.IsNotExist(statErr) {
		return "", false, statErr
	}
	if err := os.Rename(stage, root); err != nil {
		if hadOld {
			_ = os.Rename(oldRoot, root)
		}
		return "", false, err
	}
	return oldRoot, hadOld, nil
}

func rollbackStorageRoot(root, oldRoot string, hadOld bool) error {
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	if hadOld {
		return os.Rename(oldRoot, root)
	}
	return nil
}
