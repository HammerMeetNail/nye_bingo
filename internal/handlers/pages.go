package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"
)

type PageHandler struct {
	templates *template.Template
}

func NewPageHandler(templatesDir string) (*PageHandler, error) {
	templates, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return nil, err
	}

	return &PageHandler{templates: templates}, nil
}

type PageData struct {
	Title      string
	HideHeader bool
	Content    template.HTML
	Scripts    template.HTML
}

func (h *PageHandler) Index(w http.ResponseWriter, r *http.Request) {
	// For a SPA, we serve the same template for all routes
	// The JavaScript router handles the actual routing
	data := PageData{
		Title: "New Year's Resolution Bingo",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.templates.ExecuteTemplate(w, "index.html", data)
}
