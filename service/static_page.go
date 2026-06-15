package service

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	StaticPageStorageDir      = "storage/static-pages"
	StaticPageManifestFile    = "static-page.json"
	MaxStaticPagePackageBytes = 50 << 20

	maxStaticPagePackageFiles  = 1000
	maxStaticPageExtractedSize = 100 << 20
)

type StaticPageService struct{}

type StaticPageManifest struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Author      string `json:"author"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Entry       string `json:"entry"`
}

type StaticPage struct {
	Name         string    `json:"name"`
	DisplayName  string    `json:"display_name"`
	Author       string    `json:"author"`
	Version      string    `json:"version"`
	Description  string    `json:"description"`
	Entry        string    `json:"entry"`
	URL          string    `json:"url"`
	AssetBaseURL string    `json:"asset_base_url"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (s *StaticPageService) InstallStaticPagePackage(zipPath string) (*StaticPage, error) {
	manifest, manifestDir, err := readStaticPageManifest(zipPath)
	if err != nil {
		return nil, err
	}
	if err := validateStaticPageManifest(manifest); err != nil {
		return nil, err
	}
	if err := validateStaticPagePackage(zipPath, manifestDir, manifest.Entry); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(StaticPageStorageDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll("storage/temp", 0755); err != nil {
		return nil, err
	}
	tmpDir := filepath.Join("storage/temp", fmt.Sprintf("static-page-%s-%d", manifest.Name, time.Now().UnixNano()))
	if err := os.RemoveAll(tmpDir); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	if err := extractStaticPagePackage(zipPath, manifestDir, tmpDir); err != nil {
		return nil, err
	}

	destDir := filepath.Join(StaticPageStorageDir, manifest.Name)
	if err := os.RemoveAll(destDir); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpDir, destDir); err != nil {
		return nil, err
	}
	return buildStaticPage(manifest, destDir)
}

func (s *StaticPageService) ListStaticPages() ([]*StaticPage, error) {
	entries, err := os.ReadDir(StaticPageStorageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*StaticPage{}, nil
		}
		return nil, err
	}

	pages := make([]*StaticPage, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pageDir := filepath.Join(StaticPageStorageDir, entry.Name())
		manifest, err := readStaticPageManifestFile(filepath.Join(pageDir, StaticPageManifestFile))
		if err != nil {
			continue
		}
		if err := validateStaticPageManifest(manifest); err != nil {
			continue
		}
		page, err := buildStaticPage(manifest, pageDir)
		if err != nil {
			continue
		}
		pages = append(pages, page)
	}
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].Name < pages[j].Name
	})
	return pages, nil
}

func (s *StaticPageService) DeleteStaticPage(name string) error {
	name = strings.TrimSpace(name)
	if !themeNamePattern.MatchString(name) {
		return fmt.Errorf("static page name can only contain lowercase letters, numbers, hyphens and underscores")
	}
	return os.RemoveAll(filepath.Join(StaticPageStorageDir, name))
}

func readStaticPageManifest(zipPath string) (*StaticPageManifest, string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	for _, file := range reader.File {
		cleanName, ok := cleanZipName(file.Name)
		if !ok || file.FileInfo().IsDir() {
			continue
		}
		if path.Base(cleanName) != StaticPageManifestFile {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, "", err
		}
		var manifest StaticPageManifest
		err = json.NewDecoder(rc).Decode(&manifest)
		closeErr := rc.Close()
		if err != nil {
			return nil, "", err
		}
		if closeErr != nil {
			return nil, "", closeErr
		}
		return &manifest, path.Dir(cleanName), nil
	}
	return nil, "", fmt.Errorf("%s not found in package", StaticPageManifestFile)
}

func readStaticPageManifestFile(filename string) (*StaticPageManifest, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var manifest StaticPageManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func validateStaticPageManifest(manifest *StaticPageManifest) error {
	if manifest == nil {
		return fmt.Errorf("static page manifest is empty")
	}
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.DisplayName = strings.TrimSpace(manifest.DisplayName)
	manifest.Author = strings.TrimSpace(manifest.Author)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Description = strings.TrimSpace(manifest.Description)
	manifest.Entry = strings.TrimSpace(strings.ReplaceAll(manifest.Entry, "\\", "/"))
	if manifest.Name == "" {
		return fmt.Errorf("static page name is required")
	}
	if !themeNamePattern.MatchString(manifest.Name) {
		return fmt.Errorf("static page name can only contain lowercase letters, numbers, hyphens and underscores")
	}
	if manifest.DisplayName == "" {
		manifest.DisplayName = manifest.Name
	}
	if manifest.Version == "" {
		manifest.Version = "1.0.0"
	}
	if manifest.Entry == "" {
		manifest.Entry = "index.html"
	}
	entry, ok := cleanStaticPageAssetPath(manifest.Entry)
	if !ok || !isAllowedStaticPageFile(entry) || !strings.EqualFold(path.Ext(entry), ".html") {
		return fmt.Errorf("static page entry path is invalid")
	}
	manifest.Entry = entry
	return nil
}

func validateStaticPagePackage(zipPath, manifestDir, entry string) error {
	stat, err := os.Stat(zipPath)
	if err != nil {
		return err
	}
	if stat.Size() > MaxStaticPagePackageBytes {
		return fmt.Errorf("static page package is too large")
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	var fileCount int
	var totalSize uint64
	entryFound := false
	for _, file := range reader.File {
		cleanName, ok := cleanZipName(file.Name)
		if !ok {
			return fmt.Errorf("invalid file path in static page package: %s", file.Name)
		}
		if !isUnderZipDir(cleanName, manifestDir) {
			continue
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed in static page packages")
		}
		fileCount++
		if fileCount > maxStaticPagePackageFiles {
			return fmt.Errorf("static page package contains too many files")
		}
		totalSize += file.UncompressedSize64
		if totalSize > maxStaticPageExtractedSize {
			return fmt.Errorf("static page package extracted size is too large")
		}
		relName := strings.TrimPrefix(cleanName, manifestDir)
		relName = strings.TrimPrefix(relName, "/")
		if !isAllowedStaticPageFile(relName) {
			return fmt.Errorf("static page package contains unsupported file type: %s", relName)
		}
		if relName == entry {
			entryFound = true
		}
	}
	if !entryFound {
		return fmt.Errorf("static page entry file not found: %s", entry)
	}
	return nil
}

func extractStaticPagePackage(zipPath, manifestDir, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	destRoot, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destRoot, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		cleanName, ok := cleanZipName(file.Name)
		if !ok || !isUnderZipDir(cleanName, manifestDir) {
			continue
		}
		relName := strings.TrimPrefix(cleanName, manifestDir)
		relName = strings.TrimPrefix(relName, "/")
		if relName == "" {
			continue
		}
		target := filepath.Join(destRoot, filepath.FromSlash(relName))
		if !isUnderDir(destRoot, target) {
			return fmt.Errorf("invalid file path in static page package: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if !isAllowedStaticPageFile(relName) {
			return fmt.Errorf("static page package contains unsupported file type: %s", relName)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := unzipFile(file, target); err != nil {
			return err
		}
	}
	return nil
}

func buildStaticPage(manifest *StaticPageManifest, pageDir string) (*StaticPage, error) {
	stat, err := os.Stat(pageDir)
	if err != nil {
		return nil, err
	}
	return &StaticPage{
		Name:         manifest.Name,
		DisplayName:  manifest.DisplayName,
		Author:       manifest.Author,
		Version:      manifest.Version,
		Description:  manifest.Description,
		Entry:        manifest.Entry,
		URL:          StaticPagePublicURL(manifest.Name, manifest.Entry),
		AssetBaseURL: StaticPagePublicBaseURL(manifest.Name),
		UpdatedAt:    stat.ModTime(),
	}, nil
}

func StaticPagePublicBaseURL(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || !themeNamePattern.MatchString(name) {
		return ""
	}
	return "/static-pages/" + name + "/"
}

func StaticPagePublicURL(name, entry string) string {
	baseURL := StaticPagePublicBaseURL(name)
	if baseURL == "" {
		return ""
	}
	entry, ok := cleanStaticPageAssetPath(entry)
	if !ok {
		return ""
	}
	if entry == "index.html" {
		return baseURL
	}
	return baseURL + entry
}

func cleanStaticPageAssetPath(assetPath string) (string, bool) {
	assetPath = strings.TrimSpace(strings.ReplaceAll(assetPath, "\\", "/"))
	if assetPath == "" {
		return "", false
	}
	cleanPath := path.Clean(assetPath)
	if cleanPath == "." || strings.HasPrefix(cleanPath, "../") || strings.HasPrefix(cleanPath, "/") {
		return "", false
	}
	return cleanPath, true
}

func isAllowedStaticPageFile(relName string) bool {
	relName = strings.TrimSpace(strings.ReplaceAll(relName, "\\", "/"))
	if relName == "" {
		return false
	}
	cleanName, ok := cleanStaticPageAssetPath(relName)
	if !ok {
		return false
	}
	if cleanName == StaticPageManifestFile {
		return true
	}
	switch strings.ToLower(path.Ext(cleanName)) {
	case ".html", ".css", ".js", ".mjs", ".json", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".otf", ".eot", ".txt", ".md", ".xml", ".pdf", ".map":
		return true
	default:
		return false
	}
}
