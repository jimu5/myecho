package view_engine

import (
	"github.com/CloudyKit/jet/v6"
	"github.com/fsnotify/fsnotify"
	"github.com/gofiber/fiber/v2"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// HotReloadEngine implements the fiber.Views interface
type HotReloadEngine struct {
	sync.RWMutex
	*jet.Set
	viewDir string
	ext     string
}

// New creates a new instance of the HotReloadEngine
func New(directory, extension string) *HotReloadEngine {
	engine := &HotReloadEngine{
		viewDir: directory,
		ext:     extension,
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
	e.RLock()
	defer e.RUnlock()

	t, err := e.GetTemplate(template)
	if err != nil {
		return err
	}

	vars := make(jet.VarMap)
	if data != nil {
		if d, ok := data.(fiber.Map); ok {
			for k, v := range d {
				vars.Set(k, v)
			}
		}
	}

	return t.Execute(out, vars, nil)
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