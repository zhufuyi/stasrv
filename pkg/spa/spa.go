// Package spa provides a Single Page Application (SPA) server for the Gin framework.
// It supports serving static assets from both local file systems and embed.FS,
// handles HTML5 History API by redirecting 404 errors to the entry point,
// and allows dynamic content injection into static files.
package spa

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

type options struct {
	is404ToHome          bool
	cacheMaxAge          int
	enableListFiles      bool
	injectFileContentMap map[string][]func(content []byte) []byte // file: function
}

func defaultOptions() *options {
	return &options{
		is404ToHome:     true,
		enableListFiles: false,
		cacheMaxAge:     0,
	}
}

// Option set the jwt options.
type Option func(*options)

func (o *options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

// With404ToHome set 404 to home page
func With404ToHome(enable bool) Option {
	return func(o *options) {
		o.is404ToHome = enable
	}
}

// WithListFiles enable list files
func WithListFiles(enable bool) Option {
	return func(o *options) {
		o.enableListFiles = enable
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

// WithInjectFileContentByString set inject file content by string
func WithInjectFileContentByString(oldStr string, newStr string, files ...string) Option {
	return func(o *options) {
		if len(files) == 0 {
			fmt.Println("[Tip] no specified files to handle content, please use WithInjectFileContentByString to specify files.")
			return
		}

		if o.injectFileContentMap == nil {
			o.injectFileContentMap = make(map[string][]func(content []byte) []byte, len(files))
		}
		fn := func(content []byte) []byte {
			return bytes.ReplaceAll(content, []byte(oldStr), []byte(newStr))
		}
		for _, file := range files {
			o.injectFileContentMap[file] = append(o.injectFileContentMap[file], fn)
		}
	}
}

// WithInjectFileContentByRegular set inject file content by regular expression
func WithInjectFileContentByRegular(regStr string, replaceStr string, files ...string) Option {
	return func(o *options) {
		if len(files) == 0 {
			fmt.Println("[Tip] no specified files to handle content, please use WithInjectFileContentByRegular to specify files.")
			return
		}

		if o.injectFileContentMap == nil {
			o.injectFileContentMap = make(map[string][]func(content []byte) []byte, len(files))
		}
		re := regexp.MustCompile(regStr)
		fn := func(content []byte) []byte {
			return re.ReplaceAll(content, []byte(replaceStr))
		}
		for _, file := range files {
			o.injectFileContentMap[file] = append(o.injectFileContentMap[file], fn)
		}
	}
}

// ------------------------------------------------------------------------------------

// Server configures and serves the static assets and SPA routes.
type Server struct {
	localDir string // local static file directory, e.g. /data/dist
	basePath string // URL prefix, default is /

	isUseEmbedFS bool     // if true, use embed.FS, otherwise local static file.
	embedFS      embed.FS // embed.FS static resources.

	// inject file content, example: replace config.js apiBaseUrl value.
	injectFileContentMap map[string][]func(content []byte) []byte

	// when request route notfound, default is true.
	// true: redirect to index.html
	// false: returns 404, default is false.
	is404ToHome bool

	// enable list files, default is false
	enableListFiles bool

	// static file cache seconds, 0 means disable cache
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
		localDir:             localDir,
		basePath:             normalizeBasePath(basePath),
		injectFileContentMap: o.injectFileContentMap,
		is404ToHome:          o.is404ToHome,
		enableListFiles:      o.enableListFiles,
		cacheMaxAge:          o.cacheMaxAge,
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
		localDir:             localDir,
		basePath:             normalizeBasePath(basePath),
		embedFS:              embedFS,
		isUseEmbedFS:         true,
		injectFileContentMap: o.injectFileContentMap,
		is404ToHome:          o.is404ToHome,
		enableListFiles:      o.enableListFiles,
		cacheMaxAge:          o.cacheMaxAge,
	}, nil
}

// Register mounts the SPA routes to the gin Engine.
func (f *Server) Register(r *gin.Engine) error {
	// use embed file
	if f.isUseEmbedFS {
		if f.injectFileContentMap == nil {
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
		indexHTML := fmt.Sprintf("%s/index.html", f.basePath)
		r.NoRoute(handleNotFound(indexHTML, f.basePath))
	}

	err := f.injectFileContent()
	if err != nil {
		fmt.Printf("Handle file content error: %v\n", err)
	}

	relativePath := fmt.Sprintf("%s/*filepath", f.basePath)
	handlerFunc := func(c *gin.Context) {
		filePath := c.Param("filepath")
		fullPath := filepath.Join(f.localDir, filepath.Clean(filePath))
		if !strings.HasPrefix(fullPath, filepath.Clean(f.localDir)) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		var httpStatus int
		httpStatus, fullPath = checkAllowListFiles(fullPath, f.enableListFiles)
		if httpStatus > 0 {
			c.Status(httpStatus)
			return
		}

		f.setCacheHeader(c, filePath)
		c.File(fullPath)
	}
	r.GET(relativePath, handlerFunc)
	r.HEAD(relativePath, handlerFunc)
}

func (f *Server) embedFSRegister(r *gin.Engine) error {
	if f.basePath == "/" {
		f.basePath = ""
	}

	if f.is404ToHome {
		indexHTML := fmt.Sprintf("%s/index.html", f.basePath)
		r.NoRoute(handleNotFoundFS(f.embedFS, indexHTML, f.basePath))
	}

	// Use fs.Sub to switch the root directory of the file system to the actual localDir
	subFS, err := fs.Sub(f.embedFS, f.localDir)
	if err != nil {
		return fmt.Errorf("failed to create sub filesystem for %s: %v", f.localDir, err)
	}

	fileServer := http.FileServer(http.FS(subFS))
	handler := http.StripPrefix(f.basePath, fileServer)
	relativePath := fmt.Sprintf("%s/*filepath", f.basePath)

	handlerFunc := func(c *gin.Context) {
		filePath := c.Param("filepath")

		var httpStatus int
		httpStatus = checkAllowListFilesFS(subFS, filePath, f.enableListFiles)
		if httpStatus > 0 {
			c.Status(httpStatus)
			return
		}

		f.setCacheHeader(c, filePath)
		handler.ServeHTTP(c.Writer, c.Request)
	}

	r.GET(relativePath, handlerFunc)
	r.HEAD(relativePath, handlerFunc)

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

func (f *Server) injectFileContent() error {
	if f.injectFileContentMap == nil {
		return nil
	}

	filePaths, err := ListFiles(f.localDir)
	if err != nil {
		return err
	}

	for _, filePath := range filePaths {
		for file, fns := range f.injectFileContentMap {
			if isSuffixPath(filePath, file) {
				content, err := os.ReadFile(filePath)
				if err != nil {
					return err
				}
				for _, fn := range fns {
					content = fn(content)
				}
				err = os.WriteFile(filePath, content, 0o644)
				if err != nil {
					return err
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

	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp",
		".svg", ".ico",
		".woff", ".woff2", ".ttf", ".eot",
		".mp4", ".webm", ".mp3":
		c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, must-revalidate", f.cacheMaxAge))
	case ".css", ".js":
		var cacheControlValue string
		if hasHashFileName(file) {
			cacheControlValue = fmt.Sprintf("public, max-age=%d, immutable", f.cacheMaxAge)
		} else {
			cacheControlValue = fmt.Sprintf("public, max-age=%d, must-revalidate", f.cacheMaxAge)
		}
		c.Header("Cache-Control", cacheControlValue)
	default:
		c.Header("Cache-Control", "no-cache")
	}
}

func checkAllowListFiles(fullPath string, enableListFiles bool) (int, string) {
	if enableListFiles {
		return 0, fullPath
	}

	fi, err := os.Stat(fullPath)
	if err != nil {
		return http.StatusNotFound, ""
	}

	if fi.IsDir() {
		fullPath = filepath.Join(fullPath, "index.html")
		_, err = os.Stat(fullPath)
		if err != nil {
			return http.StatusForbidden, ""
		}
	}
	return 0, fullPath
}

func checkAllowListFilesFS(fsys fs.FS, filePath string, enableListFiles bool) int {
	if enableListFiles {
		return 0
	}

	filePath = strings.TrimLeft(filePath, "/")
	filePath = strings.TrimRight(filePath, "/")
	if filePath == "" {
		filePath = "."
	}

	fi, err := fs.Stat(fsys, filePath)
	if err != nil {
		return http.StatusNotFound
	}

	if fi.IsDir() {
		indexFile := path.Join(filePath, "index.html")
		_, err = fs.Stat(fsys, indexFile)
		if err != nil {
			return http.StatusForbidden
		}
	}

	return 0
}

// solve vue using history route 404 problem, for location file
func handleNotFound(indexHTML string, basePath string) func(c *gin.Context) {
	return func(c *gin.Context) {
		isReturnHome := false
		if strings.HasPrefix(c.Request.URL.Path, basePath) {
			content, err := os.ReadFile(indexHTML)
			if err == nil {
				isReturnHome = true
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.Writer.WriteHeader(http.StatusOK)
				_, _ = c.Writer.Write(content)
				c.Writer.Flush()
			}
		}
		if !isReturnHome {
			c.String(http.StatusNotFound, "404 Not Found")
		}
	}
}

// solve vue using history route 404 problem, for embed.FS
func handleNotFoundFS(efs embed.FS, indexHTML string, basePath string) func(c *gin.Context) {
	return func(c *gin.Context) {
		isReturnHome := false
		if strings.HasPrefix(c.Request.URL.Path, basePath) {
			content, err := efs.ReadFile(indexHTML)
			if err == nil {
				isReturnHome = true
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.Writer.WriteHeader(http.StatusOK)
				_, _ = c.Writer.Write(content)
				c.Writer.Flush()
			}
		}
		if !isReturnHome {
			c.String(http.StatusNotFound, "404 Not Found")
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
			(c >= 'a' && c <= 'z') ||
			(c == '_')) {
			return false
		}
	}

	return true
}

func normalizePath(p string) string {
	if p == "" {
		return ""
	}

	// convert '\' -> '/'
	p = strings.ReplaceAll(p, `\`, `/`)

	// remove drive letter
	if len(p) >= 2 && p[1] == ':' {
		p = p[2:]
	}

	// use path.Clean instead of filepath.Clean
	p = path.Clean(p)

	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	return p
}

func isSuffixPath(path1 string, path2 string) bool {
	if path2 == "" {
		return false
	}
	return strings.HasSuffix(normalizePath(path1), normalizePath(path2))
}
