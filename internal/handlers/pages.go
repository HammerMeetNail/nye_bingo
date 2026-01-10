package handlers

import (
	"html/template"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

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
	BaseURL             string
	CSSPath             string
	APIJSPath           string
	AnonymousCardJSPath string
	AppJSPath           string
	AIWizardJSPath      string
}

func (h *PageHandler) Index(w http.ResponseWriter, r *http.Request) {
	// For a SPA, we serve the same template for all routes
	// The JavaScript router handles the actual routing
	data := PageData{
		Title:               "Year of Bingo",
		BaseURL:             resolveBaseURL(r),
		CSSPath:             h.manifest.GetCSS(),
		APIJSPath:           h.manifest.GetAPIJS(),
		AnonymousCardJSPath: h.manifest.GetAnonymousCardJS(),
		AppJSPath:           h.manifest.GetAppJS(),
		AIWizardJSPath:      h.manifest.GetAIWizardJS(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
	}
}

func resolveBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if v := sanitizeProto(firstForwardedValue(r.Header.Get("X-Forwarded-Proto"))); v != "" {
		scheme = v
	}

	host := sanitizeHost(r.Host)
	if v := sanitizeHost(firstForwardedValue(r.Header.Get("X-Forwarded-Host"))); v != "" {
		host = v
	}

	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host
}

func firstForwardedValue(v string) string {
	if v == "" {
		return ""
	}
	parts := strings.Split(v, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func sanitizeProto(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "http":
		return "http"
	case "https":
		return "https"
	default:
		return ""
	}
}

func sanitizeHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.Contains(raw, "://") {
		return ""
	}
	if strings.ContainsAny(raw, " \t\r\n/\\?#") {
		return ""
	}
	if strings.Contains(raw, "@") {
		return ""
	}

	host := raw
	port := ""

	if strings.HasPrefix(raw, "[") {
		parsedHost, parsedPort, err := net.SplitHostPort(raw)
		if err != nil {
			if strings.HasSuffix(raw, "]") {
				trimmed := strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]")
				if net.ParseIP(trimmed) != nil {
					return "[" + trimmed + "]"
				}
			}
			return ""
		}
		host, port = parsedHost, parsedPort
	} else if strings.Count(raw, ":") == 1 {
		parsedHost, parsedPort, err := net.SplitHostPort(raw)
		if err == nil {
			host, port = parsedHost, parsedPort
		} else {
			if net.ParseIP(raw) == nil {
				return ""
			}
			return raw
		}
	} else if strings.Contains(raw, ":") {
		if net.ParseIP(raw) != nil {
			return raw
		}
		return ""
	}

	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return ""
		}
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	hostLower := strings.ToLower(host)
	if net.ParseIP(hostLower) == nil && !isValidHostname(hostLower) {
		return ""
	}

	if port == "" {
		return hostLower
	}
	return net.JoinHostPort(hostLower, port)
}

func isValidHostname(host string) bool {
	if host == "localhost" {
		return true
	}
	if len(host) > 253 {
		return false
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}

	labels := strings.Split(host, ".")
	if len(labels) == 0 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}
		if !isAlphaNum(label[0]) || !isAlphaNum(label[len(label)-1]) {
			return false
		}
		for i := 0; i < len(label); i++ {
			b := label[i]
			if isAlphaNum(b) || b == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
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
