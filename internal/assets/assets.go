package assets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Manifest holds the mapping of original asset paths to hashed versions
type Manifest struct {
	mu       sync.RWMutex
	assets   map[string]string
	basePath string
}

// NewManifest creates a new asset manifest
func NewManifest(basePath string) *Manifest {
	return &Manifest{
		assets:   make(map[string]string),
		basePath: basePath,
	}
}

// Load reads the manifest.json file
func (m *Manifest) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	manifestPath := filepath.Join(m.basePath, "web", "static", "dist", "manifest.json")

	// #nosec G304 -- manifestPath is constructed from trusted basePath, not user input
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// If manifest doesn't exist, use original paths (dev mode)
		if os.IsNotExist(err) {
			m.assets = make(map[string]string)
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &m.assets)
}

// Get returns the hashed path for an asset, or the original if not found
func (m *Manifest) Get(path string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if hashed, ok := m.assets[path]; ok {
		return "/static/" + hashed
	}
	// Fallback to original path (dev mode)
	return "/static/" + path
}

// GetCSS returns the hashed path for the main CSS file
func (m *Manifest) GetCSS() string {
	return m.Get("css/styles.css")
}

// GetAPIJS returns the hashed path for api.js
func (m *Manifest) GetAPIJS() string {
	return m.Get("js/api.js")
}

// GetAppJS returns the hashed path for app.js
func (m *Manifest) GetAppJS() string {
	return m.Get("js/app.js")
}

// GetAnonymousCardJS returns the hashed path for anonymous-card.js
func (m *Manifest) GetAnonymousCardJS() string {
	return m.Get("js/anonymous-card.js")
}
