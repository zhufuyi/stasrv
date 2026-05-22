// Package spa provides a simple way to serve Single Page Applications
// and static assets with Gin router.
package spa

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

type options struct {
	handleContentFn func(content []byte) []byte
	specifiedFile   map[string]struct{}
	is404ToHome     bool
	cacheMaxAge     int
}

func defaultOptions() *options {
	return &options{
		cacheMaxAge: 31536000, // 31536000 seconds = 1 year
	}
}

// Option set the jwt options.
type Option func(*options)

func (o *options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

// WithHandleContent set function to handle content and specified files
func WithHandleContent(fn func(content []byte) []byte, files ...string) Option {
	return func(o *options) {
		o.handleContentFn = fn
		if len(files) > 0 {
			o.specifiedFile = make(map[string]struct{})
			for _, file := range files {
				o.specifiedFile[file] = struct{}{}
			}
		} else {
			fmt.Printf("[Tip] no specified files to handle content, please use WithHandleContent to specify files.")
		}
	}
}

// With404ToHome set 404 to home page
func With404ToHome() Option {
	return func(o *options) {
		o.is404ToHome = true
	}
}

// WithCacheMaxAge set static file cache max-age seconds.
// seconds <= 0 means disable cache.
func WithCacheMaxAge(seconds int) Option {
	return func(o *options) {
		if seconds < 0 {
			seconds = 0
		}
		o.cacheMaxAge = seconds
	}
}

// ------------------------------------------------------------------------------------

// Server configures and serves the static assets and SPA routes.
type Server struct {
	localDir string // local static file directory, e.g. /data/dist
	basePath string // URL prefix, default is /

	isUseEmbedFS bool     // if true, use embed.FS, otherwise local static file.
	embedFS      embed.FS // embed.FS static resources.

	handleContentFn func(content []byte) []byte // handle file content, example: replace config.js apiBaseUrl value.
	specifiedFile   map[string]struct{}         // specified files to handle content, e.g. config.js

	// when request route notfound
	// true: redirect to index.html
	// false: returns 404, default is false.
	is404ToHome bool

	// static file cache seconds
	// 0 means disable cache, default 1 year
	cacheMaxAge int
}

// NewLocal creates a new SPA Server with local static files.
func NewLocal(localDir string, basePath string, opts ...Option) (*Server, error) {
	if !isExists(localDir) {
		return nil, fmt.Errorf("the system cannot find the file or directory '%s'", localDir)
	}

	o := defaultOptions()
	o.apply(opts...)

	return &Server{
		localDir:        localDir,
		basePath:        normalizeBasePath(basePath),
		handleContentFn: o.handleContentFn,
		specifiedFile:   o.specifiedFile,
		is404ToHome:     o.is404ToHome,
		cacheMaxAge:     o.cacheMaxAge,
	}, nil
}

// NewEmbedFS creates a new SPA Server with embed.FS.
func NewEmbedFS(embedFS embed.FS, basePath string, opts ...Option) (*Server, error) {
	o := defaultOptions()
	o.apply(opts...)

	localDir := getEmbedDir(embedFS)
	if localDir == "" {
		return nil, errors.New("localDir cannot be empty, go:embed cannot specify a file, must specify a directory")
	}

	return &Server{
		localDir:        localDir,
		basePath:        normalizeBasePath(basePath),
		embedFS:         embedFS,
		isUseEmbedFS:    true,
		handleContentFn: o.handleContentFn,
		specifiedFile:   o.specifiedFile,
		is404ToHome:     o.is404ToHome,
		cacheMaxAge:     o.cacheMaxAge,
	}, nil
}

// Register mounts the SPA routes to the gin Engine.
func (f *Server) Register(r *gin.Engine) error {
	// use embed file
	if f.isUseEmbedFS {
		if f.handleContentFn == nil {
			err := f.embedFSRegister(r)
			if err != nil {
				return err
			}
		} else {
			err := f.saveFSToLocal()
			if err != nil {
				return fmt.Errorf("save embed fs to local error: %w", err)
			}
			f.localRegister(r)
		}
		return nil
	}

	// use local file
	f.localRegister(r)
	return nil
}

func (f *Server) localRegister(r *gin.Engine) {
	if f.basePath == "/" {
		f.basePath = ""
	}

	if f.is404ToHome {
		homePage := fmt.Sprintf("%s/index.html", f.basePath)
		r.NoRoute(browserRefresh(homePage)) // solve using history route 404 problem
	}

	err := f.handleFileContent()
	if err != nil {
		fmt.Printf("Handle file content error: %v\n", err)
	}

	relativePath := fmt.Sprintf("%s/*filepath", f.basePath)
	r.GET(relativePath, func(c *gin.Context) {
		filePath := c.Param("filepath")
		f.setCacheHeader(c, filePath)
		fullPath := filepath.Join(f.localDir, filepath.Clean(filePath))
		if !strings.HasPrefix(fullPath, filepath.Clean(f.localDir)) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.File(fullPath)
	})
}

func (f *Server) embedFSRegister(r *gin.Engine) error {
	if f.basePath == "/" {
		f.basePath = ""
	}

	if f.is404ToHome {
		homePage := fmt.Sprintf("%s/index.html", f.basePath)
		r.NoRoute(browserRefreshFS(f.embedFS, homePage)) // solve using history route 404 problem
	}

	// Use fs.Sub to switch the root directory of the file system to the actual localDir
	subFS, err := fs.Sub(f.embedFS, f.localDir)
	if err != nil {
		return fmt.Errorf("failed to create sub filesystem for %s: %v", f.localDir, err)
	}

	fileServer := http.FileServer(http.FS(subFS))
	handler := http.StripPrefix(f.basePath, fileServer)
	relativePath := fmt.Sprintf("%s/*filepath", f.basePath)
	r.GET(relativePath, func(c *gin.Context) {
		filePath := c.Param("filepath")
		f.setCacheHeader(c, filePath)
		handler.ServeHTTP(c.Writer, c.Request)
	})

	return nil
}

func (f *Server) saveFSToLocal() error {
	if err := os.RemoveAll(f.localDir); err != nil {
		return err
	}
	if err := os.MkdirAll(f.localDir, 0o755); err != nil {
		return err
	}

	// Walk through the embedded filesystem
	return fs.WalkDir(f.embedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}

		// Create the corresponding directory structure locally
		localPath := path
		if d.IsDir() {
			return os.MkdirAll(localPath, 0o755)
		}
		// Read the file from the embedded filesystem
		content, err := fs.ReadFile(f.embedFS, path)
		if err != nil {
			return err
		}

		// Save the content to the local file
		return os.WriteFile(localPath, content, 0o644)
	})
}

// handle file content
func (f *Server) handleFileContent() error {
	if f.handleContentFn != nil && len(f.specifiedFile) > 0 {
		filePaths, err := ListFiles(f.localDir)
		if err != nil {
			return err
		}

		for _, filePath := range filePaths {
			for file := range f.specifiedFile {
				if strings.HasSuffix(filePath, file) {
					content, err := os.ReadFile(filePath)
					if err != nil {
						return err
					}
					content = f.handleContentFn(content)
					err = os.WriteFile(filePath, content, 0o644)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (f *Server) setCacheHeader(c *gin.Context, filePath string) {
	if f.cacheMaxAge <= 0 {
		return
	}

	file := strings.ToLower(filepath.Base(filePath))
	ext := strings.ToLower(filepath.Ext(file))

	// never cache html
	if ext == ".html" {
		return
	}

	// image/font/media cache
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp",
		".svg", ".ico",
		".woff", ".woff2", ".ttf", ".eot",
		".mp4", ".webm", ".mp3":
		c.Header(
			"Cache-Control",
			fmt.Sprintf("public, max-age=%d", f.cacheMaxAge),
		)
		return
	}

	// only cache hashed js/css
	if ext == ".js" || ext == ".css" {
		if hasHashFileName(file) {
			c.Header(
				"Cache-Control",
				fmt.Sprintf(
					"public, max-age=%d, immutable",
					f.cacheMaxAge,
				),
			)
		}
	}
}

// solve vue using history route 404 problem, for system file
func browserRefresh(path string) func(c *gin.Context) {
	return func(c *gin.Context) {
		flag := strings.Contains(c.GetHeader("Accept"), "text/html")
		if flag {
			content, err := os.ReadFile(path)
			if err != nil {
				c.Writer.WriteHeader(http.StatusNotFound)
				_, _ = c.Writer.WriteString("Not Found")
				return
			}
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.Writer.WriteHeader(http.StatusOK)
			_, _ = c.Writer.Write(content)
			c.Writer.Flush()
		}
	}
}

// solve vue using history route 404 problem, for embed.FS
func browserRefreshFS(efs embed.FS, path string) func(c *gin.Context) {
	return func(c *gin.Context) {
		flag := strings.Contains(c.GetHeader("Accept"), "text/html")
		if flag {
			content, err := efs.ReadFile(path)
			if err != nil {
				c.Writer.WriteHeader(http.StatusNotFound)
				_, _ = c.Writer.WriteString("Not Found")
				return
			}
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.Writer.WriteHeader(http.StatusOK)
			_, _ = c.Writer.Write(content)
			c.Writer.Flush()
		}
	}
}

func normalizeBasePath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath != "" {
		basePath = "/" + strings.TrimPrefix(basePath, "/")
		basePath = filepath.Clean(basePath)
		basePath = filepath.ToSlash(basePath)
		basePath = strings.TrimSuffix(basePath, "/")
	}
	if basePath == "" {
		basePath = "/"
	}
	return basePath
}

func getEmbedDir(efs embed.FS) string {
	currentPath := "."
	baseDir := ""

	for {
		entries, err := efs.ReadDir(currentPath)
		if err != nil || len(entries) != 1 {
			break
		}

		entry := entries[0]
		if !entry.IsDir() {
			break
		}

		if baseDir == "" {
			baseDir = entry.Name()
		} else {
			baseDir = baseDir + "/" + entry.Name()
		}

		currentPath = baseDir
	}

	return baseDir
}

func isExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

func hasHashFileName(name string) bool {
	// example:
	// app.a1b2c3.js
	// app-8f3d2c.css
	// vendor.123456789.js

	name = strings.ToLower(name)

	ext := filepath.Ext(name)
	name = strings.TrimSuffix(name, ext)

	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '.' || r == '-'
	})

	if len(parts) < 2 {
		return false
	}

	last := parts[len(parts)-1]

	// hash length check
	if len(last) < 6 {
		return false
	}

	for _, c := range last {
		if !((c >= '0' && c <= '9') ||
			(c >= 'a' && c <= 'f')) {
			return false
		}
	}

	return true
}
