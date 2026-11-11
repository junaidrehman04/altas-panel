// Package webui serves the panel UI embedded in the API binary.
// Release builds replace dist/ with the Vite output before compiling.
// A non-release build ships a placeholder page so the package still compiles.
package webui

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// files is the embedded dist tree with the dist/ prefix removed.
func files() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return dist
	}
	return sub
}

// Serve writes one UI file. API paths are not handled here.
// A missing client route returns index.html so the browser router can load.
func Serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel, ok := safeRel(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	body, name, ok := open(rel)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeAsset(w, r, name, body)
}

func safeRel(raw string) (string, bool) {
	if strings.Contains(raw, "\x00") || strings.Contains(raw, "\\") {
		return "", false
	}
	cleaned := path.Clean("/" + raw)
	if cleaned == "/" || cleaned == "." {
		return "index.html", true
	}
	if !strings.HasPrefix(cleaned, "/") || strings.Contains(cleaned, "..") {
		return "", false
	}
	rel := strings.TrimPrefix(cleaned, "/")
	if rel == "" || strings.HasPrefix(rel, "/") {
		return "", false
	}
	return rel, true
}

func open(rel string) ([]byte, string, bool) {
	body, ok := readFile(rel)
	if ok {
		return body, rel, true
	}
	if strings.Contains(path.Base(rel), ".") {
		return nil, "", false
	}
	body, ok = readFile("index.html")
	if !ok {
		return nil, "", false
	}
	return body, "index.html", true
}

func readFile(rel string) ([]byte, bool) {
	if rel == "" || strings.Contains(rel, "..") || strings.HasPrefix(rel, "/") {
		return nil, false
	}
	file, err := files().Open(rel)
	if err != nil {
		return nil, false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(file, 32<<20))
	if err != nil {
		return nil, false
	}
	return body, true
}

func writeAsset(w http.ResponseWriter, r *http.Request, name string, body []byte) {
	header := w.Header()
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Content-Type", contentType(name, body))
	if name == "index.html" {
		header.Set("Cache-Control", "no-cache")
	} else if strings.HasPrefix(name, "assets/") {
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	if r.Method == http.MethodHead {
		header.Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(body)
}

func contentType(name string, body []byte) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".woff2":
		return "font/woff2"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		detected := mime.TypeByExtension(path.Ext(name))
		if detected != "" {
			return detected
		}
		return http.DetectContentType(body)
	}
}

// ReadIndex reports whether the embedded UI includes index.html.
// Tests use it to assert the bundle is present without serving it.
func ReadIndex() []byte {
	body, ok := readFile("index.html")
	if !ok {
		return nil
	}
	return bytes.Clone(body)
}
