package service

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v2"
	"gorm.io/gorm"

	"myecho/config/static_config"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
	"myecho/utils"
)

const (
	// MaxContentArchiveBytes is the maximum compressed archive size accepted by
	// the portable content importer/exporter.
	MaxContentArchiveBytes     int64 = 512 << 20
	MaxContentFileBytes        int64 = 16 << 20
	MaxContentFiles                  = 10000
	maxContentFrontMatterBytes       = 128 << 10
	maxContentTagsPerFile            = 256
	maxContentMediaFileBytes   int64 = 256 << 20
)

var (
	ErrContentArchiveTooLarge = errors.New("content archive exceeds the safety limit")
	ErrInvalidContentArchive  = errors.New("invalid content archive")
	ErrContentImportConflict  = errors.New("content import has conflicts")
)

// ContentImportResult is returned by both preview and apply. Existing
// slug/type matches are counted as updates; conflicts are always listed and
// prevent an apply.
type ContentImportResult struct {
	DryRun            bool                `json:"dry_run"`
	Applied           bool                `json:"applied"`
	Valid             bool                `json:"valid"`
	Files             int                 `json:"files"`
	Posts             int                 `json:"posts"`
	Pages             int                 `json:"pages"`
	Created           int                 `json:"created"`
	Updated           int                 `json:"updated"`
	CategoriesCreated int                 `json:"categories_created"`
	TagsCreated       int                 `json:"tags_created"`
	Conflicts         []string            `json:"conflicts"`
	Authors           []string            `json:"authors"`
	UnmappedAuthors   []string            `json:"unmapped_authors"`
	Plans             []ContentImportPlan `json:"plans"`
}

type ContentImportPlan struct {
	Filename        string   `json:"filename"`
	Slug            string   `json:"slug"`
	Type            string   `json:"type"`
	Action          string   `json:"action"`
	CurrentVersion  uint64   `json:"current_version,omitempty"`
	ExpectedVersion uint64   `json:"expected_version,omitempty"`
	ChangedFields   []string `json:"changed_fields"`
}

type ContentImportOptions struct {
	DefaultAuthorID uint
	AuthorMap       map[string]uint
	ActorID         uint
}

type contentFrontMatter struct {
	Title          string   `yaml:"title"`
	Slug           string   `yaml:"slug"`
	Type           string   `yaml:"type"`
	Status         int8     `yaml:"status"`
	Summary        string   `yaml:"summary"`
	SEOTitle       string   `yaml:"seo_title"`
	SEODescription string   `yaml:"seo_description"`
	ShareImage     string   `yaml:"share_image"`
	Category       string   `yaml:"category"`
	Tags           []string `yaml:"tags"`
	PostTime       string   `yaml:"post_time"`
	CreatedAt      string   `yaml:"created_at"`
	UpdatedAt      string   `yaml:"updated_at"`
	EditVersion    *uint64  `yaml:"edit_version,omitempty"`
	IsAllowComment *bool    `yaml:"is_allow_comment"`
	ContentFormat  string   `yaml:"content_format"`
	Author         string   `yaml:"author,omitempty"`
}

type contentImportEntry struct {
	name            string
	articleType     model.ArticleType
	slug            string
	title           string
	status          int8
	summary         string
	seoTitle        string
	seoDescription  string
	shareImage      string
	categoryName    string
	tagNames        []string
	content         string
	contentFormat   model.ArticleContentFormat
	postTime        time.Time
	createdAt       time.Time
	updatedAt       time.Time
	expectedVersion uint64
	hasVersion      bool
	hasCreatedAt    bool
	hasUpdatedAt    bool
	allowComment    bool
	hasAllowComment bool
	author          string
}

type contentMediaManifest struct {
	Version int                 `yaml:"version"`
	Files   []contentMediaEntry `yaml:"files"`
}

type contentMediaEntry struct {
	URL           string `yaml:"url"`
	ArchivePath   string `yaml:"path"`
	Name          string `yaml:"name"`
	DirPath       string `yaml:"dir_path"`
	UUID          string `yaml:"uuid"`
	ExtensionName string `yaml:"extension_name"`
	MD5           string `yaml:"md5"`
	Note          string `yaml:"note,omitempty"`
	AltText       string `yaml:"alt_text,omitempty"`
	Caption       string `yaml:"caption,omitempty"`
	Width         int    `yaml:"width,omitempty"`
	Height        int    `yaml:"height,omitempty"`
	Data          []byte `yaml:"-"`
}

type contentArchive struct {
	Entries []contentImportEntry
	Media   []contentMediaEntry
}

// CreateContentExportArchive returns a portable ZIP containing one Markdown
// file for every post and page. It intentionally omits all account and
// application state.
func CreateContentExportArchive() ([]byte, error) {
	return CreateContentExportArchiveWithMedia(false)
}

func CreateContentExportArchiveWithMedia(includeMedia bool) ([]byte, error) {
	if connect.Database == nil {
		return nil, errors.New("database is not initialized")
	}

	articles := make([]mysql.ArticleModel, 0)
	err := connect.Database.Model(&mysql.ArticleModel{}).
		Preload("Detail").
		Preload("Category").
		Preload("Tags").
		Preload("Author").
		Where("type IN ?", []model.ArticleType{model.ArticleTypePost, model.ArticleTypePage}).
		Order("type, id").
		Find(&articles).Error
	if err != nil {
		return nil, err
	}
	if len(articles) > MaxContentFiles {
		return nil, ErrContentArchiveTooLarge
	}

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	usedNames := make(map[string]struct{}, len(articles))
	for _, article := range articles {
		content := ""
		if article.Detail != nil {
			content = article.Detail.Content
		}
		if int64(len(content)) > MaxContentFileBytes {
			_ = writer.Close()
			return nil, ErrContentArchiveTooLarge
		}
		if !utf8.ValidString(content) {
			_ = writer.Close()
			return nil, fmt.Errorf("article %q contains invalid UTF-8 content", article.Slug)
		}

		frontMatter := exportContentFrontMatter(&article)
		if err := validateExportFrontMatter(&frontMatter); err != nil {
			_ = writer.Close()
			return nil, err
		}
		frontMatterBytes, err := yaml.Marshal(&frontMatter)
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		fileData := make([]byte, 0, len(frontMatterBytes)+len(content)+8)
		fileData = append(fileData, []byte("---\n")...)
		fileData = append(fileData, frontMatterBytes...)
		fileData = append(fileData, []byte("---\n")...)
		fileData = append(fileData, content...)
		if int64(len(fileData)) > MaxContentFileBytes {
			_ = writer.Close()
			return nil, ErrContentArchiveTooLarge
		}

		name := contentExportFilename(&article)
		if _, exists := usedNames[name]; exists {
			name = contentExportFilenameWithID(&article)
		}
		if _, exists := usedNames[name]; exists {
			_ = writer.Close()
			return nil, fmt.Errorf("duplicate export filename %q", name)
		}
		usedNames[name] = struct{}{}

		file, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		if _, err := file.Write(fileData); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if includeMedia {
		if err := writeContentMedia(writer, articles); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if int64(archive.Len()) > MaxContentArchiveBytes {
		return nil, ErrContentArchiveTooLarge
	}
	return archive.Bytes(), nil
}

func writeContentMedia(writer *zip.Writer, articles []mysql.ArticleModel) error {
	var files []model.File
	if err := connect.Database.Find(&files).Error; err != nil {
		return err
	}
	references := make([]string, 0, len(articles)*2)
	for i := range articles {
		references = append(references, articles[i].ShareImage)
		if articles[i].Detail != nil {
			references = append(references, articles[i].Detail.Content)
		}
	}
	manifest := contentMediaManifest{Version: 1, Files: []contentMediaEntry{}}
	var total int64
	for i := range files {
		archivePath, sourcePath, err := contentMediaPaths(&files[i])
		if err != nil {
			return err
		}
		fileURL := path.Join(static_config.StorageRootUrl, strings.ReplaceAll(files[i].DirPath, "\\", "/"), files[i].UUID) + files[i].ExtensionName
		referenced := false
		for _, value := range references {
			if strings.Contains(value, fileURL) {
				referenced = true
				break
			}
		}
		if !referenced {
			continue
		}
		info, err := os.Lstat(sourcePath)
		if err != nil {
			return fmt.Errorf("media %s: %w", fileURL, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > maxContentMediaFileBytes {
			return ErrContentArchiveTooLarge
		}
		total += info.Size()
		if total > MaxContentArchiveBytes {
			return ErrContentArchiveTooLarge
		}
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		digest := md5.Sum(data)
		entry := contentMediaEntry{
			URL: fileURL, ArchivePath: archivePath, Name: files[i].Name, DirPath: files[i].DirPath,
			UUID: files[i].UUID, ExtensionName: files[i].ExtensionName, MD5: hex.EncodeToString(digest[:]),
			Note: files[i].Note, AltText: files[i].AltText, Caption: files[i].Caption,
			Width: files[i].Width, Height: files[i].Height,
		}
		destination, err := writer.CreateHeader(&zip.FileHeader{Name: archivePath, Method: zip.Deflate})
		if err != nil {
			return err
		}
		if _, err := destination.Write(data); err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, entry)
	}
	manifestData, err := yaml.Marshal(&manifest)
	if err != nil {
		return err
	}
	file, err := writer.CreateHeader(&zip.FileHeader{Name: "media/manifest.yaml", Method: zip.Deflate})
	if err != nil {
		return err
	}
	_, err = file.Write(manifestData)
	return err
}

func contentMediaPaths(file *model.File) (string, string, error) {
	dir := strings.ReplaceAll(strings.TrimSpace(file.DirPath), "\\", "/")
	if dir == "." {
		dir = ""
	}
	if dir != "" && (path.IsAbs(dir) || path.Clean(dir) != dir || strings.HasPrefix(dir, "../")) {
		return "", "", fmt.Errorf("%w: unsafe media path", ErrInvalidContentArchive)
	}
	name := file.UUID + file.ExtensionName
	if file.UUID == "" || path.Base(name) != name || strings.ContainsAny(name, "/\\\x00") {
		return "", "", fmt.Errorf("%w: unsafe media filename", ErrInvalidContentArchive)
	}
	archivePath := path.Join("media/files", dir, name)
	root, err := filepath.Abs(static_config.StorageRootPath)
	if err != nil {
		return "", "", err
	}
	source := filepath.Join(root, filepath.FromSlash(dir), name)
	absSource, err := filepath.Abs(source)
	if err != nil || (absSource != root && !strings.HasPrefix(absSource, root+string(filepath.Separator))) {
		return "", "", fmt.Errorf("%w: unsafe media path", ErrInvalidContentArchive)
	}
	for current := filepath.Dir(absSource); ; current = filepath.Dir(current) {
		if info, statErr := os.Lstat(current); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", "", fmt.Errorf("%w: media path contains a symlink", ErrInvalidContentArchive)
		} else if statErr != nil && !os.IsNotExist(statErr) {
			return "", "", statErr
		}
		if current == root {
			break
		}
		if current == filepath.Dir(current) {
			return "", "", fmt.Errorf("%w: unsafe media path", ErrInvalidContentArchive)
		}
	}
	return archivePath, absSource, nil
}

func validateExportFrontMatter(frontMatter *contentFrontMatter) error {
	for _, field := range []struct {
		name  string
		value string
		limit int
	}{
		{"title", frontMatter.Title, 128},
		{"slug", frontMatter.Slug, 160},
		{"summary", frontMatter.Summary, 255},
		{"seo_title", frontMatter.SEOTitle, 160},
		{"seo_description", frontMatter.SEODescription, 255},
		{"share_image", frontMatter.ShareImage, 512},
		{"category", frontMatter.Category, 64},
		{"author", frontMatter.Author, 64},
	} {
		if err := validateContentText(field.name, field.value, field.limit); err != nil {
			return fmt.Errorf("invalid export front matter: %w", err)
		}
	}
	for _, tag := range frontMatter.Tags {
		if err := validateContentText("tag", tag, 64); err != nil {
			return fmt.Errorf("invalid export front matter: %w", err)
		}
	}
	return nil
}

func exportContentFrontMatter(article *mysql.ArticleModel) contentFrontMatter {
	typeName := "post"
	if article.Type == model.ArticleTypePage {
		typeName = "page"
	}
	allowComment := true
	if article.IsAllowComment != nil {
		allowComment = *article.IsAllowComment
	}
	tags := make([]string, 0, len(article.Tags))
	for _, tag := range article.Tags {
		if tag != nil {
			tags = append(tags, tag.Name)
		}
	}
	category := ""
	if article.Category != nil {
		category = article.Category.Name
	}
	author := ""
	if article.Author != nil {
		author = article.Author.Name
	}
	return contentFrontMatter{
		Title:          article.Title,
		Slug:           article.Slug,
		Type:           typeName,
		Status:         article.Status,
		Summary:        article.Summary,
		SEOTitle:       article.SEOTitle,
		SEODescription: article.SEODescription,
		ShareImage:     article.ShareImage,
		Category:       category,
		Tags:           tags,
		PostTime:       formatContentTime(article.PostTime),
		CreatedAt:      formatContentTime(article.CreatedAt),
		UpdatedAt:      formatContentTime(article.UpdatedAt),
		EditVersion:    &article.EditVersion,
		IsAllowComment: &allowComment,
		ContentFormat:  string(model.NormalizeArticleContentFormat(article.ContentFormat)),
		Author:         author,
	}
}

func formatContentTime(value time.Time) string {
	return value.Format(time.RFC3339Nano)
}

func contentExportFilename(article *mysql.ArticleModel) string {
	typeName := "post"
	if article.Type == model.ArticleTypePage {
		typeName = "page"
	}
	slug := safeContentFilename(article.Slug)
	if slug == "" {
		slug = fmt.Sprintf("article-%d", article.ID)
	}
	return typeName + "/" + slug + ".md"
}

func contentExportFilenameWithID(article *mysql.ArticleModel) string {
	typeName := "post"
	if article.Type == model.ArticleTypePage {
		typeName = "page"
	}
	slug := safeContentFilename(article.Slug)
	if slug == "" {
		slug = "article"
	}
	return fmt.Sprintf("%s/%s-%d.md", typeName, slug, article.ID)
}

func safeContentFilename(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.TrimSpace(value) {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), r == '.', r == '_':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-.")
}

// PreviewContentImportArchive validates and previews a portable content ZIP
// without writing categories, tags, articles, or details.
func PreviewContentImportArchive(reader io.Reader, archiveSize int64) (ContentImportResult, error) {
	archive, err := readContentArchive(reader, archiveSize)
	if err != nil {
		return ContentImportResult{DryRun: true, Conflicts: []string{}}, err
	}
	return planContentImport(archive.Entries, true)
}

// ApplyContentImportArchive validates and applies a portable content ZIP in a
// single transaction. Existing slug/type matches are updated in place.
func ApplyContentImportArchive(reader io.Reader, archiveSize int64, authorIDs ...uint) (ContentImportResult, error) {
	options := ContentImportOptions{}
	if len(authorIDs) > 0 {
		options.DefaultAuthorID = authorIDs[0]
	}
	return ApplyContentImportArchiveWithOptions(reader, archiveSize, options)
}

func ApplyContentImportArchiveWithOptions(reader io.Reader, archiveSize int64, options ContentImportOptions) (ContentImportResult, error) {
	archive, err := readContentArchive(reader, archiveSize)
	if err != nil {
		return ContentImportResult{Conflicts: []string{}}, err
	}
	result, err := planContentImport(archive.Entries, false)
	if err != nil {
		return result, err
	}
	if len(result.Conflicts) != 0 {
		return result, fmt.Errorf("%w: %s", ErrContentImportConflict, strings.Join(result.Conflicts, "; "))
	}

	if connect.Database == nil {
		return result, errors.New("database is not initialized")
	}
	actorID := options.ActorID
	if actorID == 0 {
		actorID = options.DefaultAuthorID
	}
	plannedActions := make(map[string]string, len(result.Plans))
	for _, plan := range result.Plans {
		plannedActions[plan.Filename] = plan.Action
	}
	createdMedia := make([]string, 0, len(archive.Media))
	err = connect.Database.Transaction(func(tx *gorm.DB) error {
		if err := restoreContentMedia(tx, archive.Media, options.DefaultAuthorID, &createdMedia); err != nil {
			return err
		}
		for _, entry := range archive.Entries {
			authorID, err := resolveContentAuthorID(tx, entry.author, options)
			if err != nil {
				return err
			}
			categoryUID, _, err := ensureContentCategory(tx, entry.categoryName)
			if err != nil {
				return err
			}
			tags, _, err := ensureContentTags(tx, entry.tagNames)
			if err != nil {
				return err
			}

			var matches []mysql.ArticleModel
			err = tx.Model(&mysql.ArticleModel{}).
				Where("slug = ? AND type = ?", entry.slug, entry.articleType).
				Order("id").Find(&matches).Error
			if err != nil {
				return err
			}
			switch {
			case plannedActions[entry.name] == "create" && len(matches) == 0:
				if err := createImportedArticle(tx, &entry, categoryUID, tags, authorID); err != nil {
					return err
				}
			case plannedActions[entry.name] == "update" && len(matches) == 1:
				if err := updateImportedArticle(tx, &matches[0], &entry, categoryUID, tags, authorID, actorID); err != nil {
					return err
				}
			case len(matches) > 1:
				return fmt.Errorf("%w: multiple existing articles for slug/type %q", ErrContentImportConflict, contentImportKey(entry.slug, entry.articleType))
			default:
				return fmt.Errorf("%w: %s changed after preview; preview and apply again", ErrContentImportConflict, entry.name)
			}
		}
		return nil
	})
	if err != nil {
		for _, name := range createdMedia {
			_ = os.Remove(name)
		}
		if errors.Is(err, mysql.ErrArticleEditConflict) || errors.Is(err, mysql.ErrArticleSubmissionChanged) || errors.Is(err, mysql.ErrArticleTrashed) {
			err = fmt.Errorf("%w: %v", ErrContentImportConflict, err)
		}
		if errors.Is(err, ErrContentImportConflict) {
			result.Valid = false
			result.Conflicts = append(result.Conflicts, err.Error())
		}
		return result, err
	}
	result.Applied = true
	result.Valid = true
	return result, nil
}

func readContentArchive(reader io.Reader, archiveSize int64) (contentArchive, error) {
	if reader == nil || archiveSize < 0 {
		return contentArchive{}, fmt.Errorf("%w: missing archive", ErrInvalidContentArchive)
	}
	if archiveSize > MaxContentArchiveBytes {
		return contentArchive{}, ErrContentArchiveTooLarge
	}
	archive, err := io.ReadAll(io.LimitReader(reader, MaxContentArchiveBytes+1))
	if err != nil {
		return contentArchive{}, fmt.Errorf("%w: %v", ErrInvalidContentArchive, err)
	}
	if int64(len(archive)) > MaxContentArchiveBytes {
		return contentArchive{}, ErrContentArchiveTooLarge
	}
	if len(archive) == 0 {
		return contentArchive{}, fmt.Errorf("%w: empty archive", ErrInvalidContentArchive)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return contentArchive{}, fmt.Errorf("%w: %v", ErrInvalidContentArchive, err)
	}
	entries := make([]contentImportEntry, 0, len(zipReader.File))
	seenPaths := make(map[string]struct{}, len(zipReader.File))
	seenEntries := 0
	var totalContentBytes int64
	mediaData := make(map[string][]byte)
	var mediaManifest []byte
	for _, file := range zipReader.File {
		seenEntries++
		if seenEntries > MaxContentFiles {
			return contentArchive{}, ErrContentArchiveTooLarge
		}
		isDirectory := file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/")
		cleanName, err := validateContentArchivePath(file.Name, isDirectory)
		if err != nil {
			return contentArchive{}, fmt.Errorf("%w: %v", ErrInvalidContentArchive, err)
		}
		if isDirectory {
			continue
		}
		if _, exists := seenPaths[cleanName]; exists {
			return contentArchive{}, fmt.Errorf("%w: duplicate path %q", ErrInvalidContentArchive, cleanName)
		}
		seenPaths[cleanName] = struct{}{}
		if cleanName == "media/manifest.yaml" || strings.HasPrefix(cleanName, "media/files/") {
			limit := maxContentMediaFileBytes
			if cleanName == "media/manifest.yaml" {
				limit = maxContentFrontMatterBytes
			}
			content, err := readContentZipFile(file, limit)
			if err != nil {
				return contentArchive{}, err
			}
			totalContentBytes += int64(len(content))
			if totalContentBytes > MaxContentArchiveBytes {
				return contentArchive{}, ErrContentArchiveTooLarge
			}
			if cleanName == "media/manifest.yaml" {
				mediaManifest = content
			} else {
				mediaData[cleanName] = content
			}
			continue
		}
		if !strings.EqualFold(path.Ext(cleanName), ".md") {
			return contentArchive{}, fmt.Errorf("%w: only Markdown and packaged media are supported", ErrInvalidContentArchive)
		}
		if file.UncompressedSize64 > uint64(MaxContentFileBytes) {
			return contentArchive{}, ErrContentArchiveTooLarge
		}

		source, err := file.Open()
		if err != nil {
			return contentArchive{}, fmt.Errorf("%w: %v", ErrInvalidContentArchive, err)
		}
		content, readErr := io.ReadAll(io.LimitReader(source, MaxContentFileBytes+1))
		closeErr := source.Close()
		if readErr != nil {
			return contentArchive{}, fmt.Errorf("%w: %v", ErrInvalidContentArchive, readErr)
		}
		if closeErr != nil {
			return contentArchive{}, fmt.Errorf("%w: %v", ErrInvalidContentArchive, closeErr)
		}
		if int64(len(content)) > MaxContentFileBytes {
			return contentArchive{}, ErrContentArchiveTooLarge
		}
		totalContentBytes += int64(len(content))
		if totalContentBytes > MaxContentArchiveBytes {
			return contentArchive{}, ErrContentArchiveTooLarge
		}

		entry, err := parseContentMarkdown(cleanName, content)
		if err != nil {
			return contentArchive{}, err
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return contentArchive{}, fmt.Errorf("%w: archive has no Markdown files", ErrInvalidContentArchive)
	}
	media, err := parseContentMediaArchive(mediaManifest, mediaData)
	if err != nil {
		return contentArchive{}, err
	}
	return contentArchive{Entries: entries, Media: media}, nil
}

func readContentZipFile(file *zip.File, limit int64) ([]byte, error) {
	if file.UncompressedSize64 > uint64(limit) {
		return nil, ErrContentArchiveTooLarge
	}
	source, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidContentArchive, err)
	}
	defer source.Close()
	data, err := io.ReadAll(io.LimitReader(source, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidContentArchive, err)
	}
	if int64(len(data)) > limit {
		return nil, ErrContentArchiveTooLarge
	}
	return data, nil
}

func parseContentMediaArchive(manifestData []byte, mediaData map[string][]byte) ([]contentMediaEntry, error) {
	if len(manifestData) == 0 {
		if len(mediaData) != 0 {
			return nil, fmt.Errorf("%w: media manifest is required", ErrInvalidContentArchive)
		}
		return nil, nil
	}
	var manifest contentMediaManifest
	if err := yaml.UnmarshalStrict(manifestData, &manifest); err != nil || manifest.Version != 1 {
		return nil, fmt.Errorf("%w: invalid media manifest", ErrInvalidContentArchive)
	}
	seen := make(map[string]struct{}, len(manifest.Files))
	for i := range manifest.Files {
		entry := &manifest.Files[i]
		if _, exists := seen[entry.ArchivePath]; exists {
			return nil, fmt.Errorf("%w: duplicate media path", ErrInvalidContentArchive)
		}
		seen[entry.ArchivePath] = struct{}{}
		file := model.File{
			Name: entry.Name, DirPath: entry.DirPath, UUID: entry.UUID, ExtensionName: entry.ExtensionName,
		}
		expectedPath, _, err := contentMediaPaths(&file)
		if err != nil || expectedPath != entry.ArchivePath {
			return nil, fmt.Errorf("%w: invalid media path", ErrInvalidContentArchive)
		}
		expectedURL := path.Join(static_config.StorageRootUrl, strings.ReplaceAll(entry.DirPath, "\\", "/"), entry.UUID) + entry.ExtensionName
		if entry.URL != expectedURL || len(entry.Name) > 255 || len(entry.Note) > 256 || len(entry.AltText) > 512 || len(entry.Caption) > 512 {
			return nil, fmt.Errorf("%w: invalid media metadata", ErrInvalidContentArchive)
		}
		data, exists := mediaData[entry.ArchivePath]
		if !exists {
			return nil, fmt.Errorf("%w: missing media file %q", ErrInvalidContentArchive, entry.ArchivePath)
		}
		digest := md5.Sum(data)
		if !strings.EqualFold(entry.MD5, hex.EncodeToString(digest[:])) {
			return nil, fmt.Errorf("%w: media checksum mismatch", ErrInvalidContentArchive)
		}
		entry.Data = data
	}
	if len(seen) != len(mediaData) {
		return nil, fmt.Errorf("%w: unlisted media file", ErrInvalidContentArchive)
	}
	return manifest.Files, nil
}

func restoreContentMedia(tx *gorm.DB, entries []contentMediaEntry, uploaderID uint, createdPaths *[]string) error {
	for _, entry := range entries {
		var existing model.File
		err := tx.Where("uuid = ?", entry.UUID).First(&existing).Error
		hasExisting := err == nil
		if err == nil {
			if !strings.EqualFold(existing.MD5, entry.MD5) {
				return fmt.Errorf("%w: media UUID %q already exists", ErrContentImportConflict, entry.UUID)
			}
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		file := model.File{
			Name: entry.Name, DirPath: entry.DirPath, UUID: entry.UUID, ExtensionName: entry.ExtensionName,
			MD5: entry.MD5, Note: entry.Note, UploaderID: uploaderID, AltText: entry.AltText,
			Caption: entry.Caption, Width: entry.Width, Height: entry.Height,
		}
		_, destination, err := contentMediaPaths(&file)
		if err != nil {
			return err
		}
		info, statErr := os.Lstat(destination)
		if statErr == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
			return fmt.Errorf("%w: unsafe media destination", ErrContentImportConflict)
		}
		if data, readErr := os.ReadFile(destination); readErr == nil {
			digest := md5.Sum(data)
			if !strings.EqualFold(entry.MD5, hex.EncodeToString(digest[:])) {
				return fmt.Errorf("%w: media path %q already exists", ErrContentImportConflict, entry.URL)
			}
		} else if statErr != nil && !os.IsNotExist(statErr) {
			return statErr
		} else if !os.IsNotExist(readErr) {
			return readErr
		} else {
			if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
				return err
			}
			output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				return err
			}
			_, writeErr := output.Write(entry.Data)
			closeErr := output.Close()
			if writeErr != nil {
				_ = os.Remove(destination)
				return writeErr
			}
			if closeErr != nil {
				_ = os.Remove(destination)
				return closeErr
			}
			*createdPaths = append(*createdPaths, destination)
		}
		if !hasExisting {
			if err := tx.Session(&gorm.Session{SkipHooks: true}).Create(&file).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func validateContentArchivePath(name string, directory bool) (string, error) {
	if name == "" || !utf8.ValidString(name) || strings.ContainsRune(name, '\x00') || strings.Contains(name, "\\") {
		return "", errors.New("unsafe archive path")
	}
	if strings.HasSuffix(name, "/") {
		if !directory {
			return "", errors.New("file path cannot end with slash")
		}
		name = strings.TrimSuffix(name, "/")
	}
	if name == "" || path.IsAbs(name) || strings.HasPrefix(name, "../") || name == ".." || strings.Contains(name, ":") {
		return "", errors.New("unsafe archive path")
	}
	cleanName := path.Clean(name)
	if cleanName != name || cleanName == "." || strings.HasPrefix(cleanName, "../") {
		return "", errors.New("unsafe archive path")
	}
	return cleanName, nil
}

func parseContentMarkdown(name string, data []byte) (contentImportEntry, error) {
	frontMatterBytes, body, err := splitContentFrontMatter(data)
	if err != nil {
		if errors.Is(err, ErrContentArchiveTooLarge) {
			return contentImportEntry{}, fmt.Errorf("%w: %s", ErrContentArchiveTooLarge, name)
		}
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	var frontMatter contentFrontMatter
	if err := yaml.UnmarshalStrict(frontMatterBytes, &frontMatter); err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s front matter: %v", ErrInvalidContentArchive, name, err)
	}
	if !utf8.Valid(body) {
		return contentImportEntry{}, fmt.Errorf("%w: %s body is not UTF-8", ErrInvalidContentArchive, name)
	}

	articleType, err := parseContentArticleType(frontMatter.Type)
	if err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	slug := strings.TrimSpace(frontMatter.Slug)
	if strings.TrimSpace(frontMatter.Title) == "" || slug == "" {
		return contentImportEntry{}, fmt.Errorf("%w: %s requires title and slug", ErrInvalidContentArchive, name)
	}
	if strings.ContainsAny(slug, "/\\") || strings.ContainsRune(slug, '\x00') {
		return contentImportEntry{}, fmt.Errorf("%w: %s has an unsafe slug", ErrInvalidContentArchive, name)
	}
	if err := validateContentText("title", frontMatter.Title, 128); err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	if err := validateContentText("slug", slug, 160); err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	if err := validateContentText("summary", frontMatter.Summary, 255); err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	if err := validateContentText("seo_title", frontMatter.SEOTitle, 160); err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	if err := validateContentText("seo_description", frontMatter.SEODescription, 255); err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	if err := validateContentText("share_image", frontMatter.ShareImage, 512); err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	if err := validateContentText("category", frontMatter.Category, 64); err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	if err := validateContentText("author", frontMatter.Author, 64); err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	if frontMatter.Status < int8(mysql.ARTILCE_STATUS_PUBLIC) || frontMatter.Status > int8(mysql.ARTICLE_STATUS_RECYCLE) {
		return contentImportEntry{}, fmt.Errorf("%w: %s has invalid status %d", ErrInvalidContentArchive, name, frontMatter.Status)
	}

	contentFormat := model.NormalizeArticleContentFormat(model.ArticleContentFormat(strings.ToLower(strings.TrimSpace(frontMatter.ContentFormat))))
	if !model.IsValidArticleContentFormat(contentFormat) {
		return contentImportEntry{}, fmt.Errorf("%w: %s has invalid content_format %q", ErrInvalidContentArchive, name, frontMatter.ContentFormat)
	}
	postTime, hasPostTime, err := parseContentTime("post_time", frontMatter.PostTime)
	if err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	if !hasPostTime {
		return contentImportEntry{}, fmt.Errorf("%w: %s requires post_time", ErrInvalidContentArchive, name)
	}
	createdAt, hasCreatedAt, err := parseContentTime("created_at", frontMatter.CreatedAt)
	if err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	updatedAt, hasUpdatedAt, err := parseContentTime("updated_at", frontMatter.UpdatedAt)
	if err != nil {
		return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
	}
	if len(frontMatter.Tags) > maxContentTagsPerFile {
		return contentImportEntry{}, fmt.Errorf("%w: %s has too many tags", ErrInvalidContentArchive, name)
	}
	tagNames := make([]string, 0, len(frontMatter.Tags))
	seenTags := make(map[string]struct{}, len(frontMatter.Tags))
	for _, rawTag := range frontMatter.Tags {
		tagName := strings.TrimSpace(rawTag)
		if tagName == "" {
			return contentImportEntry{}, fmt.Errorf("%w: %s has an empty tag", ErrInvalidContentArchive, name)
		}
		if err := validateContentText("tag", tagName, 64); err != nil {
			return contentImportEntry{}, fmt.Errorf("%w: %s: %v", ErrInvalidContentArchive, name, err)
		}
		if _, exists := seenTags[tagName]; exists {
			continue
		}
		seenTags[tagName] = struct{}{}
		tagNames = append(tagNames, tagName)
	}

	allowComment := true
	hasAllowComment := frontMatter.IsAllowComment != nil
	if hasAllowComment {
		allowComment = *frontMatter.IsAllowComment
	}
	expectedVersion := uint64(0)
	if frontMatter.EditVersion != nil {
		expectedVersion = *frontMatter.EditVersion
	}
	return contentImportEntry{
		name:            name,
		articleType:     articleType,
		slug:            slug,
		title:           strings.TrimSpace(frontMatter.Title),
		status:          frontMatter.Status,
		summary:         frontMatter.Summary,
		seoTitle:        frontMatter.SEOTitle,
		seoDescription:  frontMatter.SEODescription,
		shareImage:      frontMatter.ShareImage,
		categoryName:    strings.TrimSpace(frontMatter.Category),
		tagNames:        tagNames,
		content:         string(body),
		contentFormat:   contentFormat,
		postTime:        postTime,
		createdAt:       createdAt,
		updatedAt:       updatedAt,
		expectedVersion: expectedVersion,
		hasVersion:      frontMatter.EditVersion != nil,
		hasCreatedAt:    hasCreatedAt,
		hasUpdatedAt:    hasUpdatedAt,
		allowComment:    allowComment,
		hasAllowComment: hasAllowComment,
		author:          strings.TrimSpace(frontMatter.Author),
	}, nil
}

func splitContentFrontMatter(data []byte) ([]byte, []byte, error) {
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		data = data[3:]
	}
	headerLength := 0
	switch {
	case bytes.HasPrefix(data, []byte("---\n")):
		headerLength = 4
	case bytes.HasPrefix(data, []byte("---\r\n")):
		headerLength = 5
	default:
		return nil, nil, errors.New("front matter must start with ---")
	}
	lineStart := headerLength
	for lineStart <= len(data) {
		lineEnd := bytes.IndexByte(data[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(data)
		} else {
			lineEnd += lineStart
		}
		line := bytes.TrimSuffix(data[lineStart:lineEnd], []byte{'\r'})
		if bytes.Equal(line, []byte("---")) {
			if lineStart-headerLength > maxContentFrontMatterBytes {
				return nil, nil, ErrContentArchiveTooLarge
			}
			bodyStart := lineEnd
			if bodyStart < len(data) && data[bodyStart] == '\n' {
				bodyStart++
			}
			return data[headerLength:lineStart], data[bodyStart:], nil
		}
		if lineEnd == len(data) {
			return nil, nil, errors.New("front matter closing --- is missing")
		}
		lineStart = lineEnd + 1
		if lineStart-headerLength > maxContentFrontMatterBytes {
			return nil, nil, ErrContentArchiveTooLarge
		}
	}
	return nil, nil, errors.New("front matter closing --- is missing")
}

func parseContentArticleType(value string) (model.ArticleType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "post":
		return model.ArticleTypePost, nil
	case "page":
		return model.ArticleTypePage, nil
	default:
		return 0, fmt.Errorf("type must be post or page")
	}
}

func parseContentTime(field, value string) (time.Time, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, true, fmt.Errorf("%s must be RFC3339: %w", field, err)
	}
	return parsed, true, nil
}

func validateContentText(field, value string, maxRunes int) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s is not UTF-8", field)
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return fmt.Errorf("%s exceeds %d characters", field, maxRunes)
	}
	return nil
}

func planContentImport(entries []contentImportEntry, dryRun bool) (ContentImportResult, error) {
	result := ContentImportResult{
		DryRun:    dryRun,
		Conflicts: []string{},
		Authors:   []string{}, UnmappedAuthors: []string{}, Plans: []ContentImportPlan{},
	}
	if connect.Database == nil {
		return result, errors.New("database is not initialized")
	}
	seenKeys := make(map[string]string, len(entries))
	authorNames := make(map[string]struct{})
	uniqueEntries := make([]contentImportEntry, 0, len(entries))
	plans := make(map[string]ContentImportPlan, len(entries))
	for _, entry := range entries {
		result.Files++
		if entry.articleType == model.ArticleTypePost {
			result.Posts++
		} else {
			result.Pages++
		}
		key := contentImportKey(entry.slug, entry.articleType)
		if previous, exists := seenKeys[key]; exists {
			result.Conflicts = append(result.Conflicts, fmt.Sprintf("duplicate slug/type %q in %s and %s", key, previous, entry.name))
			plans[entry.name] = ContentImportPlan{
				Filename: entry.name, Slug: entry.slug, Type: contentTypeName(entry.articleType),
				Action: "conflict", ExpectedVersion: entry.expectedVersion, ChangedFields: []string{},
			}
			continue
		}
		seenKeys[key] = entry.name
		uniqueEntries = append(uniqueEntries, entry)
		if entry.author != "" {
			authorNames[entry.author] = struct{}{}
		}
	}
	for author := range authorNames {
		result.Authors = append(result.Authors, author)
		var count int64
		if err := connect.Database.Model(&model.User{}).Where("name = ? OR slug = ?", author, model.NormalizeUserSlug(author)).Count(&count).Error; err != nil {
			return result, err
		}
		if count == 0 {
			result.UnmappedAuthors = append(result.UnmappedAuthors, author)
		}
	}
	sort.Strings(result.Authors)
	sort.Strings(result.UnmappedAuthors)

	categoryNames := make(map[string]struct{})
	tagNames := make(map[string]struct{})
	for _, entry := range uniqueEntries {
		if entry.categoryName != "" {
			categoryNames[entry.categoryName] = struct{}{}
		}
		for _, tagName := range entry.tagNames {
			tagNames[tagName] = struct{}{}
		}
	}
	for categoryName := range categoryNames {
		var count int64
		if err := connect.Database.Model(&mysql.CategoryModel{}).
			Where("name = ? AND type = ?", categoryName, model.CategoryTypeArticle).
			Count(&count).Error; err != nil {
			return result, err
		}
		if count == 0 {
			result.CategoriesCreated++
		}
	}
	for tagName := range tagNames {
		var count int64
		if err := connect.Database.Model(&model.Tag{}).Where("name = ?", tagName).Count(&count).Error; err != nil {
			return result, err
		}
		if count == 0 {
			result.TagsCreated++
		}
	}

	for _, entry := range uniqueEntries {
		var matches []mysql.ArticleModel
		if err := connect.Database.Model(&mysql.ArticleModel{}).
			Preload("Detail").Preload("Category").Preload("Tags").Preload("Author").
			Where("slug = ? AND type = ?", entry.slug, entry.articleType).
			Order("id").Find(&matches).Error; err != nil {
			return result, err
		}
		plan := ContentImportPlan{
			Filename: entry.name, Slug: entry.slug, Type: contentTypeName(entry.articleType),
			Action: "create", ChangedFields: contentCreateFields(entry),
		}
		switch len(matches) {
		case 0:
			result.Created++
		case 1:
			result.Updated++
			existing := &matches[0]
			plan.Action = "update"
			plan.CurrentVersion = existing.EditVersion
			plan.ExpectedVersion = entry.expectedVersion
			plan.ChangedFields = contentChangedFields(existing, entry)
			if !entry.hasVersion || entry.expectedVersion == 0 {
				result.Conflicts = append(result.Conflicts, fmt.Sprintf("%s matches existing article %q, but edit_version is required; export a fresh archive before updating", entry.name, entry.slug))
			} else if entry.expectedVersion != existing.EditVersion {
				result.Conflicts = append(result.Conflicts, fmt.Sprintf("%s expected edit_version %d, current version is %d; export a fresh archive before updating", entry.name, entry.expectedVersion, existing.EditVersion))
			}
			if existing.Status == int8(mysql.ARTICLE_STATUS_WAIT_REVIEW) {
				result.Conflicts = append(result.Conflicts, fmt.Sprintf("%s matches an article awaiting review", entry.name))
			}
			if existing.Status == int8(mysql.ARTICLE_STATUS_RECYCLE) {
				result.Conflicts = append(result.Conflicts, fmt.Sprintf("%s matches an article in trash", entry.name))
			}
			var pending int64
			if err := connect.Database.Model(&model.ArticleSubmission{}).
				Where("article_id = ? AND status = ?", existing.ID, model.ArticleSubmissionPending).
				Count(&pending).Error; err != nil {
				return result, err
			}
			if pending > 0 && existing.Status != int8(mysql.ARTICLE_STATUS_WAIT_REVIEW) {
				result.Conflicts = append(result.Conflicts, fmt.Sprintf("%s matches an article with a pending review submission", entry.name))
			}
		default:
			result.Updated++
			plan.Action = "update"
			plan.ExpectedVersion = entry.expectedVersion
			plan.ChangedFields = []string{}
			result.Conflicts = append(result.Conflicts, fmt.Sprintf("multiple existing articles for slug/type %q", contentImportKey(entry.slug, entry.articleType)))
		}
		plans[entry.name] = plan
	}
	for _, entry := range entries {
		if plan, exists := plans[entry.name]; exists {
			result.Plans = append(result.Plans, plan)
		}
	}
	result.Valid = len(result.Conflicts) == 0
	return result, nil
}

func contentTypeName(articleType model.ArticleType) string {
	if articleType == model.ArticleTypePage {
		return "page"
	}
	return "post"
}

func contentCreateFields(entry contentImportEntry) []string {
	fields := []string{"title", "slug", "type", "status", "summary", "seo_title", "seo_description", "share_image", "category", "tags", "content", "content_format", "post_time", "author", "is_allow_comment"}
	if entry.hasCreatedAt {
		fields = append(fields, "created_at")
	}
	if entry.hasUpdatedAt {
		fields = append(fields, "updated_at")
	}
	return fields
}

func contentChangedFields(article *mysql.ArticleModel, entry contentImportEntry) []string {
	fields := make([]string, 0, 16)
	add := func(name string, changed bool) {
		if changed {
			fields = append(fields, name)
		}
	}
	add("title", article.Title != entry.title)
	add("slug", article.Slug != entry.slug)
	add("summary", article.Summary != entry.summary)
	add("seo_title", article.SEOTitle != entry.seoTitle)
	add("seo_description", article.SEODescription != entry.seoDescription)
	add("share_image", article.ShareImage != entry.shareImage)
	add("content_format", model.NormalizeArticleContentFormat(article.ContentFormat) != entry.contentFormat)
	add("content", article.Detail == nil || article.Detail.Content != entry.content)
	add("post_time", !article.PostTime.Equal(entry.postTime))
	add("status", article.Status != entry.status)
	categoryName := ""
	if article.Category != nil {
		categoryName = article.Category.Name
	}
	add("category", categoryName != entry.categoryName)
	add("tags", !sameContentTags(article.Tags, entry.tagNames))
	if entry.author != "" {
		authorMatches := false
		if article.Author != nil {
			authorMatches = article.Author.Name == entry.author || article.Author.Slug == model.NormalizeUserSlug(entry.author)
		}
		add("author", !authorMatches)
	}
	if entry.hasAllowComment {
		allowComment := true
		if article.IsAllowComment != nil {
			allowComment = *article.IsAllowComment
		}
		add("is_allow_comment", allowComment != entry.allowComment)
	}
	if entry.hasCreatedAt {
		add("created_at", !article.CreatedAt.Equal(entry.createdAt))
	}
	if entry.hasUpdatedAt {
		add("updated_at", !article.UpdatedAt.Equal(entry.updatedAt))
	}
	return fields
}

func sameContentTags(existing []*model.Tag, imported []string) bool {
	existingNames := make([]string, 0, len(existing))
	for _, tag := range existing {
		if tag != nil {
			existingNames = append(existingNames, tag.Name)
		}
	}
	importedNames := append([]string(nil), imported...)
	sort.Strings(existingNames)
	sort.Strings(importedNames)
	if len(existingNames) != len(importedNames) {
		return false
	}
	for i := range existingNames {
		if existingNames[i] != importedNames[i] {
			return false
		}
	}
	return true
}

func contentImportKey(slug string, articleType model.ArticleType) string {
	typeName := "post"
	if articleType == model.ArticleTypePage {
		typeName = "page"
	}
	return typeName + "/" + slug
}

func ensureContentCategory(tx *gorm.DB, name string) (string, bool, error) {
	if name == "" {
		return "", false, nil
	}
	var category mysql.CategoryModel
	err := tx.Model(&mysql.CategoryModel{}).
		Where("name = ? AND type = ?", name, model.CategoryTypeArticle).
		Order("id").First(&category).Error
	if err == nil {
		return category.UID, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, err
	}
	category = mysql.CategoryModel{Name: name, Type: model.CategoryTypeArticle}
	if err := tx.Create(&category).Error; err != nil {
		return "", false, err
	}
	return category.UID, true, nil
}

func ensureContentTags(tx *gorm.DB, names []string) ([]*model.Tag, int, error) {
	tags := make([]*model.Tag, 0, len(names))
	created := 0
	for _, name := range names {
		var tag model.Tag
		err := tx.Where("name = ?", name).Order("id").First(&tag).Error
		if err == nil {
			tags = append(tags, &tag)
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, created, err
		}
		tag = model.Tag{Name: name}
		if err := tx.Create(&tag).Error; err != nil {
			return nil, created, err
		}
		created++
		tags = append(tags, &tag)
	}
	return tags, created, nil
}

func createImportedArticle(tx *gorm.DB, entry *contentImportEntry, categoryUID string, tags []*model.Tag, authorID uint) error {
	articleUID := utils.GenUID20()
	detail := &model.ArticleDetail{
		UID:     articleUID + "_" + utils.GenUID20(),
		Content: entry.content,
	}
	if err := tx.Create(detail).Error; err != nil {
		return err
	}
	article := &mysql.ArticleModel{
		BaseModel:      model.BaseModel{CreatedAt: entry.createdAt, UpdatedAt: entry.updatedAt},
		UID:            articleUID,
		Title:          entry.title,
		Slug:           entry.slug,
		SEOTitle:       entry.seoTitle,
		SEODescription: entry.seoDescription,
		ShareImage:     entry.shareImage,
		Type:           entry.articleType,
		ContentFormat:  entry.contentFormat,
		Summary:        entry.summary,
		IsAllowComment: boolPointer(entry.allowComment),
		CategoryUID:    categoryUID,
		DetailUID:      detail.UID,
		PostTime:       entry.postTime,
		Status:         entry.status,
		AuthorID:       authorID,
	}
	if err := tx.Omit("Author", "Tags", "Detail", "Password").Create(article).Error; err != nil {
		return err
	}
	if article.Slug != entry.slug {
		return fmt.Errorf("%w: slug %q was normalized to %q", ErrContentImportConflict, entry.slug, article.Slug)
	}
	if err := tx.Model(article).Association("Tags").Replace(tags); err != nil {
		return err
	}
	return restoreImportedArticleTimestamps(tx, article.ID, entry)
}

func updateImportedArticle(tx *gorm.DB, existing *mysql.ArticleModel, entry *contentImportEntry, categoryUID string, tags []*model.Tag, authorID, actorID uint) error {
	candidate := *existing
	candidate.Title = entry.title
	candidate.Slug = entry.slug
	candidate.SEOTitle = entry.seoTitle
	candidate.SEODescription = entry.seoDescription
	candidate.ShareImage = entry.shareImage
	candidate.Type = entry.articleType
	candidate.ContentFormat = entry.contentFormat
	candidate.Summary = entry.summary
	candidate.CategoryUID = categoryUID
	candidate.PostTime = entry.postTime
	candidate.Status = entry.status
	candidate.Detail = &model.ArticleDetail{UID: existing.DetailUID, Content: entry.content}
	candidate.Tags = tags
	if entry.hasAllowComment {
		candidate.IsAllowComment = boolPointer(entry.allowComment)
	}
	if err := (&mysql.ArticleDBRepo{}).UpdateForImport(tx, &candidate, entry.expectedVersion, actorID); err != nil {
		if errors.Is(err, mysql.ErrArticleEditConflict) || errors.Is(err, mysql.ErrArticleSubmissionChanged) || errors.Is(err, mysql.ErrArticleTrashed) {
			return fmt.Errorf("%w: %s: %v", ErrContentImportConflict, entry.name, err)
		}
		return err
	}
	if candidate.Slug != entry.slug {
		return fmt.Errorf("%w: slug %q was normalized to %q", ErrContentImportConflict, entry.slug, candidate.Slug)
	}
	if authorID != 0 {
		if err := tx.Session(&gorm.Session{SkipHooks: true}).Model(&mysql.ArticleModel{}).
			Where("id = ?", existing.ID).UpdateColumn("author_id", authorID).Error; err != nil {
			return err
		}
	}
	return restoreImportedArticleTimestamps(tx, existing.ID, entry)
}

func resolveContentAuthorID(tx *gorm.DB, source string, options ContentImportOptions) (uint, error) {
	if source == "" {
		return options.DefaultAuthorID, nil
	}
	if mapped := options.AuthorMap[source]; mapped != 0 {
		var count int64
		if err := tx.Model(&model.User{}).Where("id = ? AND disabled = ?", mapped, false).Count(&count).Error; err != nil {
			return 0, err
		}
		if count == 0 {
			return 0, fmt.Errorf("%w: author mapping for %q is invalid", ErrContentImportConflict, source)
		}
		return mapped, nil
	}
	var user model.User
	err := tx.Where("(name = ? OR slug = ?) AND disabled = ?", source, model.NormalizeUserSlug(source), false).Order("id").First(&user).Error
	if err == nil {
		return user.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	return options.DefaultAuthorID, nil
}

func restoreImportedArticleTimestamps(tx *gorm.DB, articleID uint, entry *contentImportEntry) error {
	timestamps := make(map[string]interface{}, 2)
	if entry.hasCreatedAt {
		timestamps["created_at"] = entry.createdAt
	}
	if entry.hasUpdatedAt {
		timestamps["updated_at"] = entry.updatedAt
	}
	if len(timestamps) == 0 {
		return nil
	}
	return tx.Session(&gorm.Session{SkipHooks: true}).
		Model(&mysql.ArticleModel{}).
		Where("id = ?", articleID).
		UpdateColumns(timestamps).Error
}

func boolPointer(value bool) *bool {
	return &value
}
