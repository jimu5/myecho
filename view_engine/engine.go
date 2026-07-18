package view_engine

import (
	"github.com/CloudyKit/jet/v6"
	"github.com/CloudyKit/jet/v6/loaders/multi"
	"github.com/fsnotify/fsnotify"
	"github.com/gofiber/fiber/v2"
	"io"
	"log"
	"myecho/dal/mysql"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// HotReloadEngine implements the fiber.Views interface
type HotReloadEngine struct {
	sync.RWMutex
	*jet.Set
	viewDir   string
	ext       string
	themeSets map[string]cachedThemeSet
}

type cachedThemeSet struct {
	version int64
	set     *jet.Set
}

// New creates a new instance of the HotReloadEngine
func New(directory, extension string) *HotReloadEngine {
	engine := &HotReloadEngine{
		viewDir:   directory,
		ext:       extension,
		themeSets: make(map[string]cachedThemeSet),
	}
	engine.Reload()
	go engine.watchForChanges()
	return engine
}

func (e *HotReloadEngine) Load() error {
	e.Reload()
	return nil
}

func (e *HotReloadEngine) Render(out io.Writer, template string, data interface{}, layout ...string) error {
	set := e.setForData(data)
	vars := make(jet.VarMap)
	if data != nil {
		if d, ok := data.(fiber.Map); ok {
			for k, v := range d {
				vars.Set(k, v)
			}
		}
	}
	e.RLock()
	defaultSet := e.Set
	e.RUnlock()
	if set != defaultSet {
		var themedOutput strings.Builder
		t, err := set.GetTemplate(template)
		if err == nil {
			err = t.Execute(&themedOutput, vars, nil)
		}
		if err == nil {
			output := injectThemeScript(themedOutput.String(), themeFromData(data))
			_, err = io.WriteString(out, output)
			return err
		}
		log.Printf("Theme template %q failed, falling back to default: %v", template, err)
	}

	t, err := defaultSet.GetTemplate(template)
	if err != nil {
		return err
	}
	return t.Execute(out, vars, nil)
}

func injectThemeScript(content string, theme *mysql.ThemeModel) string {
	if theme == nil {
		return content
	}
	js := strings.TrimSpace(theme.JS)
	if js == "" || strings.Contains(content, "data-myecho-theme-runtime") || strings.Contains(content, js) {
		return content
	}
	script := "\n<script type=\"text/javascript\" data-myecho-theme-runtime>\n" + theme.JS + "\n</script>\n"
	if bodyEnd := strings.LastIndex(strings.ToLower(content), "</body>"); bodyEnd >= 0 {
		return content[:bodyEnd] + script + content[bodyEnd:]
	}
	return content + script
}

// Reload creates a new Jet Set and replaces the old one
func (e *HotReloadEngine) Reload() {
	e.Lock()
	defer e.Unlock()
	log.Println("Hot-reloading Jet templates from", e.viewDir)
	e.Set = jet.NewSet(
		jet.NewOSFileSystemLoader(e.viewDir),
		jet.InDevelopmentMode(), // This helps with debugging
	)
	e.themeSets = make(map[string]cachedThemeSet)
}

func (e *HotReloadEngine) setForData(data interface{}) *jet.Set {
	theme := themeFromData(data)
	if theme == nil || !theme.HasTemplates || !isSafeThemeName(theme.Name) {
		e.RLock()
		defer e.RUnlock()
		return e.Set
	}
	return e.themeSet(theme)
}

func (e *HotReloadEngine) themeSet(theme *mysql.ThemeModel) *jet.Set {
	version := theme.UpdatedAt.UnixNano()
	e.RLock()
	cached, ok := e.themeSets[theme.Name]
	defaultSet := e.Set
	e.RUnlock()
	if ok && cached.version == version {
		return cached.set
	}

	e.Lock()
	defer e.Unlock()
	if cached, ok := e.themeSets[theme.Name]; ok && cached.version == version {
		return cached.set
	}
	themeTemplateDir := filepath.Join("storage", "themes", theme.Name, "templates")
	if _, err := os.Stat(themeTemplateDir); err != nil {
		return defaultSet
	}
	set := jet.NewSet(
		multi.NewLoader(
			jet.NewOSFileSystemLoader(themeTemplateDir),
			jet.NewOSFileSystemLoader(e.viewDir),
		),
		jet.InDevelopmentMode(),
	)
	e.themeSets[theme.Name] = cachedThemeSet{version: version, set: set}
	return set
}

func themeFromData(data interface{}) *mysql.ThemeModel {
	d, ok := data.(fiber.Map)
	if !ok {
		return nil
	}
	theme, ok := d["Theme"].(*mysql.ThemeModel)
	if ok {
		return theme
	}
	return nil
}

func isSafeThemeName(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func (e *HotReloadEngine) watchForChanges() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("Failed to create file watcher:", err)
	}
	defer watcher.Close()

	err = filepath.Walk(e.viewDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})

	if err != nil {
		log.Fatal("Failed to watch template directory:", err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// We only care about .jet.html files
			if strings.HasSuffix(event.Name, e.ext) {
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Remove == fsnotify.Remove {
					e.Reload()
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("File watcher error:", err)
		}
	}
}
