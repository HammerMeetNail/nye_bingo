package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipResponseWriter wraps http.ResponseWriter to provide gzip compression.
type gzipResponseWriter struct {
	http.ResponseWriter
	writer io.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	return g.writer.Write(b)
}

// Pool of gzip writers to reduce allocations.
var gzipPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(nil, gzip.BestSpeed)
		return w
	},
}

// Compress provides gzip compression for responses.
type Compress struct{}

// NewCompress creates a new compression middleware.
func NewCompress() *Compress {
	return &Compress{}
}

// Apply adds gzip compression to responses when the client accepts it.
func (c *Compress) Apply(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client accepts gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip compression for small responses or already compressed content
		// We'll let the response handler decide by checking content type
		// Don't compress images, videos, etc.
		path := r.URL.Path
		if isPreCompressedPath(path) {
			next.ServeHTTP(w, r)
			return
		}

		// Get a gzip writer from the pool
		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(w)
		defer func() {
			_ = gz.Close()
			gzipPool.Put(gz)
		}()

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		// Delete Content-Length as it will be different after compression
		w.Header().Del("Content-Length")

		gzw := &gzipResponseWriter{
			ResponseWriter: w,
			writer:         gz,
		}

		next.ServeHTTP(gzw, r)
	})
}

// isPreCompressedPath returns true for file types that are already compressed.
func isPreCompressedPath(path string) bool {
	compressedExtensions := []string{
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".ico",
		".mp4", ".webm", ".mp3", ".ogg",
		".zip", ".gz", ".br", ".zst",
		".woff", ".woff2",
	}

	lowerPath := strings.ToLower(path)
	for _, ext := range compressedExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			return true
		}
	}
	return false
}
