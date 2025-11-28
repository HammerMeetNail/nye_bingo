package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/HammerMeetNail/yearofbingo/internal/assets"
)

type PageHandler struct {
	templates *template.Template
	manifest  *assets.Manifest
}

func NewPageHandler(templatesDir string) (*PageHandler, error) {
	templates, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return nil, err
	}

	// Load asset manifest for cache-busted filenames
	manifest := assets.NewManifest(".")
	if err := manifest.Load(); err != nil {
		// Non-fatal: fall back to original paths in dev mode
		_ = err
	}

	return &PageHandler{
		templates: templates,
		manifest:  manifest,
	}, nil
}

type PageData struct {
	Title               string
	HideHeader          bool
	Content             template.HTML
	Scripts             template.HTML
	CSSPath             string
	APIJSPath           string
	AnonymousCardJSPath string
	AppJSPath           string
}

func (h *PageHandler) Index(w http.ResponseWriter, r *http.Request) {
	// For a SPA, we serve the same template for all routes
	// The JavaScript router handles the actual routing
	data := PageData{
		Title:               "Year of Bingo",
		CSSPath:             h.manifest.GetCSS(),
		APIJSPath:           h.manifest.GetAPIJS(),
		AnonymousCardJSPath: h.manifest.GetAnonymousCardJS(),
		AppJSPath:           h.manifest.GetAppJS(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// NotFound renders the 404 error page.
func (h *PageHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := h.templates.ExecuteTemplate(w, "404.html", nil); err != nil {
		http.Error(w, "Page not found", http.StatusNotFound)
	}
}

// InternalError renders the 500 error page.
func (h *PageHandler) InternalError(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	if err := h.templates.ExecuteTemplate(w, "500.html", nil); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
