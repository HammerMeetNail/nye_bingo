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

// GetAIWizardJS returns the hashed path for ai-wizard.js
func (m *Manifest) GetAIWizardJS() string {
	return m.Get("js/ai-wizard.js")
}

// GetAppCoreJS returns the hashed path for app-core.js
func (m *Manifest) GetAppCoreJS() string {
	return m.Get("js/app-core.js")
}

// GetAppActionsJS returns the hashed path for app-actions.js
func (m *Manifest) GetAppActionsJS() string {
	return m.Get("js/app-actions.js")
}

// GetAppModalsJS returns the hashed path for app-modals.js
func (m *Manifest) GetAppModalsJS() string {
	return m.Get("js/app-modals.js")
}

// GetAppNotificationsJS returns the hashed path for app-notifications.js
func (m *Manifest) GetAppNotificationsJS() string {
	return m.Get("js/app-notifications.js")
}

// GetAppRemindersJS returns the hashed path for app-reminders.js
func (m *Manifest) GetAppRemindersJS() string {
	return m.Get("js/app-reminders.js")
}

// GetAppFriendsJS returns the hashed path for app-friends.js
func (m *Manifest) GetAppFriendsJS() string {
	return m.Get("js/app-friends.js")
}

// GetAppBillingJS returns the hashed path for app-billing.js
func (m *Manifest) GetAppBillingJS() string {
	return m.Get("js/app-billing.js")
}

// GetAppTemplatesJS returns the hashed path for app-templates.js
func (m *Manifest) GetAppTemplatesJS() string {
	return m.Get("js/app-templates.js")
}

// GetAppAIJS returns the hashed path for app-ai.js
func (m *Manifest) GetAppAIJS() string {
	return m.Get("js/app-ai.js")
}

// GetAppAuthJS returns the hashed path for app-auth.js
func (m *Manifest) GetAppAuthJS() string {
	return m.Get("js/app-auth.js")
}

// GetAppCardsJS returns the hashed path for app-cards.js
func (m *Manifest) GetAppCardsJS() string {
	return m.Get("js/app-cards.js")
}

// GetAppModuleJSPaths returns deterministic module loading order.
// app.js remains the composition source and is loaded before these modules.
func (m *Manifest) GetAppModuleJSPaths() []string {
	return []string{
		m.GetAppCoreJS(),
		m.GetAppActionsJS(),
		m.GetAppModalsJS(),
		m.GetAppNotificationsJS(),
		m.GetAppRemindersJS(),
		m.GetAppFriendsJS(),
		m.GetAppBillingJS(),
		m.GetAppTemplatesJS(),
		m.GetAppAIJS(),
		m.GetAppAuthJS(),
		m.GetAppCardsJS(),
	}
}
