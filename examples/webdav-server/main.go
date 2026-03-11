// Example WebDAV server for testing Crush WebDAV sync functionality.
// This server implements a simple in-memory WebDAV server for testing purposes.
//
// Usage:
//
//	go run main.go [-port 8080] [-username admin] [-password admin] [-path /crush]
//
// Test with curl:
//
//	curl -X PROPFIND -u admin:admin http://localhost:8080/crush/
//
// Test with Crush:
//
//	Configure webdav_sync.go with URL: http://localhost:8080/crush/
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

// Resource represents a WebDAV resource (file or directory).
type Resource struct {
	Path        string
	Name        string
	IsDir       bool
	Data        []byte
	ContentType string
	ETag        string
	CreatedAt   time.Time
	ModifiedAt  time.Time
}

// MemoryStore is an in-memory WebDAV store.
type MemoryStore struct {
	mu        sync.RWMutex
	resources map[string]*Resource
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{
		resources: make(map[string]*Resource),
	}

	// Create root directory
	store.resources["/"] = &Resource{
		Path:       "/",
		Name:       "root",
		IsDir:      true,
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}

	return store
}

// Get returns a resource by path.
func (s *MemoryStore) Get(p string) (*Resource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.resources[p]
	return r, ok
}

// Put stores a file.
func (s *MemoryStore) Put(p string, data []byte, contentType string) *Resource {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	etag := fmt.Sprintf("\"%x-%d\"", hashData(data), now.UnixNano())

	r := &Resource{
		Path:        p,
		Name:        path.Base(p),
		IsDir:       false,
		Data:        data,
		ContentType: contentType,
		ETag:        etag,
		CreatedAt:   now,
		ModifiedAt:  now,
	}

	s.resources[p] = r

	// Update parent directory modification time
	parentPath := path.Dir(p)
	if parent, ok := s.resources[parentPath]; ok && parent.IsDir {
		parent.ModifiedAt = now
	}

	return r
}

// MkCol creates a directory.
func (s *MemoryStore) MkCol(p string) (*Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.resources[p]; ok {
		return nil, fmt.Errorf("resource already exists")
	}

	now := time.Now()
	r := &Resource{
		Path:       p,
		Name:       path.Base(p),
		IsDir:      true,
		CreatedAt:  now,
		ModifiedAt: now,
	}

	s.resources[p] = r

	// Update parent directory modification time
	parentPath := path.Dir(p)
	if parent, ok := s.resources[parentPath]; ok && parent.IsDir {
		parent.ModifiedAt = now
	}

	return r, nil
}

// Delete removes a resource.
func (s *MemoryStore) Delete(p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.resources[p]
	if !ok {
		return fmt.Errorf("not found")
	}

	if r.IsDir {
		// Check if directory is empty
		for path := range s.resources {
			if strings.HasPrefix(path, p+"/") {
				return fmt.Errorf("directory not empty")
			}
		}
	}

	delete(s.resources, p)
	return nil
}

// List returns all resources in a directory.
func (s *MemoryStore) List(dirPath string) []*Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var resources []*Resource
	for p, r := range s.resources {
		if path.Dir(p) == dirPath {
			resources = append(resources, r)
		}
	}
	return resources
}

// WebDAVHandler handles WebDAV requests.
type WebDAVHandler struct {
	store    *MemoryStore
	username string
	password string
	basePath string
}

// NewWebDAVHandler creates a new WebDAV handler.
func NewWebDAVHandler(basePath, username, password string) *WebDAVHandler {
	return &WebDAVHandler{
		store:    NewMemoryStore(),
		username: username,
		password: password,
		basePath: basePath,
	}
}

// ServeHTTP implements http.Handler.
func (h *WebDAVHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Basic authentication
	if h.username != "" && h.password != "" {
		username, password, ok := r.BasicAuth()
		if !ok || username != h.username || password != h.password {
			w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Get path
	p := r.URL.Path
	if !strings.HasPrefix(p, h.basePath) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	p = strings.TrimPrefix(p, h.basePath)
	if p == "" {
		p = "/"
	}

	log.Printf("[%s] %s %s", r.Method, h.basePath+p, r.RemoteAddr)

	// Route to appropriate handler
	switch r.Method {
	case http.MethodOptions:
		h.handleOptions(w, r)
	case http.MethodGet:
		h.handleGet(w, r, p)
	case http.MethodPut:
		h.handlePut(w, r, p)
	case http.MethodDelete:
		h.handleDelete(w, r, p)
	case "PROPFIND":
		h.handlePropFind(w, r, p)
	case "PROPPATCH":
		h.handlePropPatch(w, r, p)
	case "MKCOL":
		h.handleMkCol(w, r, p)
	case "COPY":
		h.handleCopy(w, r, p)
	case "MOVE":
		h.handleMove(w, r, p)
	case "LOCK":
		h.handleLock(w, r, p)
	case "UNLOCK":
		h.handleUnlock(w, r, p)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *WebDAVHandler) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("DAV", "1, 2")
	w.Header().Set("MS-Author-Via", "DAV")
	w.Header().Set("Allow", strings.Join([]string{
		"OPTIONS", "GET", "PUT", "DELETE",
		"PROPFIND", "PROPPATCH", "MKCOL",
		"COPY", "MOVE", "LOCK", "UNLOCK",
	}, ", "))
	w.WriteHeader(http.StatusNoContent)
}

func (h *WebDAVHandler) handleGet(w http.ResponseWriter, r *http.Request, p string) {
	res, ok := h.store.Get(p)
	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if res.IsDir {
		http.Error(w, "Is a directory", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", res.ContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(res.Data)))
	w.Header().Set("ETag", res.ETag)
	w.Header().Set("Last-Modified", res.ModifiedAt.Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
	w.Write(res.Data)
}

func (h *WebDAVHandler) handlePut(w http.ResponseWriter, r *http.Request, p string) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	h.store.Put(p, data, contentType)

	w.Header().Set("ETag", fmt.Sprintf("\"%x-%d\"", hashData(data), time.Now().UnixNano()))
	w.WriteHeader(http.StatusCreated)
}

func (h *WebDAVHandler) handleDelete(w http.ResponseWriter, r *http.Request, p string) {
	if err := h.store.Delete(p); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *WebDAVHandler) handlePropFind(w http.ResponseWriter, r *http.Request, p string) {
	depth := r.Header.Get("Depth")
	if depth == "" {
		depth = "0"
	}

	res, ok := h.store.Get(p)
	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	var resources []*Resource
	resources = append(resources, res)

	if depth == "1" && res.IsDir {
		resources = append(resources, h.store.List(p)...)
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusMultiStatus)
	w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>`))
	w.Write([]byte(`<D:multistatus xmlns:D="DAV:">`))

	for _, res := range resources {
		h.writePropFindResponse(w, res)
	}

	w.Write([]byte(`</D:multistatus>`))
}

func (h *WebDAVHandler) writePropFindResponse(w http.ResponseWriter, res *Resource) {
	w.Write([]byte(`<D:response>`))
	fmt.Fprintf(w, `<D:href>%s%s</D:href>`, h.basePath, res.Path)
	w.Write([]byte(`<D:propstat>`))
	w.Write([]byte(`<D:prop>`))

	if res.IsDir {
		w.Write([]byte(`<D:resourcetype><D:collection/></D:resourcetype>`))
	} else {
		w.Write([]byte(`<D:resourcetype/>`))
	}

	fmt.Fprintf(w, `<D:displayname>%s</D:displayname>`, res.Name)
	fmt.Fprintf(w, `<D:getcontenttype>%s</D:getcontenttype>`, res.ContentType)
	fmt.Fprintf(w, `<D:getcontentlength>%d</D:getcontentlength>`, len(res.Data))
	fmt.Fprintf(w, `<D:getetag>%s</D:getetag>`, res.ETag)
	fmt.Fprintf(w, `<D:getlastmodified>%s</D:getlastmodified>`, res.ModifiedAt.Format(http.TimeFormat))
	fmt.Fprintf(w, `<D:creationdate>%s</D:creationdate>`, res.CreatedAt.Format(time.RFC3339))

	w.Write([]byte(`</D:prop>`))
	w.Write([]byte(`<D:status>HTTP/1.1 200 OK</D:status>`))
	w.Write([]byte(`</D:propstat>`))
	w.Write([]byte(`</D:response>`))
}

func (h *WebDAVHandler) handlePropPatch(w http.ResponseWriter, r *http.Request, p string) {
	// For simplicity, we don't support custom properties
	w.WriteHeader(http.StatusNoContent)
}

func (h *WebDAVHandler) handleMkCol(w http.ResponseWriter, r *http.Request, p string) {
	_, err := h.store.MkCol(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *WebDAVHandler) handleCopy(w http.ResponseWriter, r *http.Request, p string) {
	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Destination header required", http.StatusBadRequest)
		return
	}

	// Parse destination path
	destPath := strings.TrimPrefix(dest, "http://"+r.Host+h.basePath)

	res, ok := h.store.Get(p)
	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if res.IsDir {
		// Copy directory recursively
		h.copyRecursive(p, destPath)
	} else {
		h.store.Put(destPath, res.Data, res.ContentType)
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *WebDAVHandler) handleMove(w http.ResponseWriter, r *http.Request, p string) {
	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Destination header required", http.StatusBadRequest)
		return
	}

	destPath := strings.TrimPrefix(dest, "http://"+r.Host+h.basePath)

	res, ok := h.store.Get(p)
	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if res.IsDir {
		h.moveRecursive(p, destPath)
	} else {
		h.store.Put(destPath, res.Data, res.ContentType)
		h.store.Delete(p)
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *WebDAVHandler) handleLock(w http.ResponseWriter, r *http.Request, p string) {
	// Return a simple lock token
	lockToken := fmt.Sprintf("opaquelocktoken:%x", time.Now().UnixNano())
	w.Header().Set("Lock-Token", "<"+lockToken+">")
	w.WriteHeader(http.StatusOK)
}

func (h *WebDAVHandler) handleUnlock(w http.ResponseWriter, r *http.Request, p string) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *WebDAVHandler) copyRecursive(src, dst string) {
	resources := h.store.List(src)
	for _, res := range resources {
		if res.IsDir {
			h.store.MkCol(path.Join(dst, res.Name))
			h.copyRecursive(path.Join(src, res.Name), path.Join(dst, res.Name))
		} else {
			h.store.Put(path.Join(dst, res.Name), res.Data, res.ContentType)
		}
	}
}

func (h *WebDAVHandler) moveRecursive(src, dst string) {
	h.copyRecursive(src, dst)
	h.store.Delete(src)
}

func hashData(data []byte) uint32 {
	var hash uint32 = 5381
	for _, b := range data {
		hash = ((hash << 5) + hash) + uint32(b)
	}
	return hash
}

func main() {
	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: webdav-server [flags]\n\n")
		fmt.Fprintf(os.Stderr, "A WebDAV server for testing Crush WebDAV sync functionality.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Start server on port 8080\n")
		fmt.Fprintf(os.Stderr, "  webdav-server -port 8080\n\n")
		fmt.Fprintf(os.Stderr, "  # Start server with custom credentials\n")
		fmt.Fprintf(os.Stderr, "  webdav-server -username user -password pass\n\n")
		fmt.Fprintf(os.Stderr, "  # Start server with custom path\n")
		fmt.Fprintf(os.Stderr, "  webdav-server -path /webdav\n")
	}

	port := flag.Int("port", 8080, "Port to listen on")
	username := flag.String("username", "admin", "Username for basic auth")
	password := flag.String("password", "admin", "Password for basic auth")
	basePath := flag.String("path", "/crush", "Root path for WebDAV")
	flag.Parse()

	handler := NewWebDAVHandler(*basePath, *username, *password)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting WebDAV server on http://localhost%s%s", addr, *basePath)
	log.Printf("Username: %s, Password: %s", *username, *password)
	log.Printf("Press Ctrl+C to stop")

	if err := http.ListenAndServe(addr, handler); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
