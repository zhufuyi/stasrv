// Package spa provides a Single Page Application (SPA) server for the Hertz framework.
package spa

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type options struct {
	is404ToHome          bool
	cacheMaxAge          int
	enableListFiles      bool
	injectFileContentMap map[string][]func(content []byte) []byte
}

func defaultOptions() *options {
	return &options{
		is404ToHome:     true,
		enableListFiles: false,
		cacheMaxAge:     0,
	}
}

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

func WithCacheMaxAge(seconds int) Option {
	return func(o *options) {
		if seconds < 0 {
			seconds = 0
		}
		o.cacheMaxAge = seconds
	}
}

func WithInjectFileContentByString(oldStr string, newStr string, files ...string) Option {
	return func(o *options) {
		if len(files) == 0 {
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

func WithInjectFileContentByRegular(regStr string, replaceStr string, files ...string) Option {
	return func(o *options) {
		if len(files) == 0 {
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

type Server struct {
	basePath string // URL prefix, default is /
	localDir string // local static file directory, e.g. /data/dist

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

	// adaptively turn compression on and off according to localDir permissions
	enableGzip bool
}

func NewLocal(basePath string, localDir string, opts ...Option) (*Server, error) {
	if !isExists(localDir) {
		return nil, fmt.Errorf("the system cannot find the file or directory '%s'", localDir)
	}
	o := defaultOptions()
	o.apply(opts...)
	return &Server{
		basePath:             normalizeBasePath(basePath),
		localDir:             localDir,
		injectFileContentMap: o.injectFileContentMap,
		is404ToHome:          o.is404ToHome,
		enableListFiles:      o.enableListFiles,
		cacheMaxAge:          o.cacheMaxAge,
		enableGzip:           isWritable(localDir),
	}, nil
}

func NewEmbedFS(basePath string, embedFS embed.FS, opts ...Option) (*Server, error) {
	o := defaultOptions()
	o.apply(opts...)

	n, err := countEmbedFS(embedFS)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errors.New("no file in embedFS, please copy the file to the specified embed file directory before compilation")
	}
	localDir := getEmbedDir(embedFS)
	if localDir == "" {
		return nil, errors.New("empty directory retrieved from embedFS")
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

// Register mounts the SPA routes to the Hertz server.
func (s *Server) Register(h *server.Hertz) error {
	if s.isUseEmbedFS {
		if s.injectFileContentMap == nil {
			return s.embedFSRegister(h)
		}
		if err := s.saveFSToLocal(); err != nil {
			return fmt.Errorf("save embed fs to local error: %w", err)
		}
	}
	s.localRegister(h)
	return nil
}

func (s *Server) localRegister(h *server.Hertz) {
	bp := s.basePath
	if bp == "/" {
		bp = ""
	}

	if s.is404ToHome {
		indexHTML := filepath.Join(s.localDir, "index.html")
		h.NoRoute(handleNotFound(indexHTML, s.basePath))
	}

	err := s.injectFileContent()
	if err != nil {
		fmt.Printf("Inject file content error: %v\n", err)
	}

	relativePath := bp + "/*filepath"
	handlerFunc := func(ctx context.Context, c *app.RequestContext) {
		filePath := c.Param("filepath")
		fullPath := filepath.Join(s.localDir, filepath.Clean(filePath))

		if !s.enableGzip {
			c.Request.Header.Del("Accept-Encoding")
		}

		// Security check
		if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(s.localDir)) {
			c.AbortWithStatus(consts.StatusForbidden)
			return
		}

		status, finalPath := checkAllowListFiles(fullPath, s.enableListFiles)
		if status > 0 {
			c.SetStatusCode(status)
			return
		}

		s.setCacheHeader(c, filePath)
		c.File(finalPath)
	}

	h.GET(relativePath, handlerFunc)
	h.HEAD(relativePath, handlerFunc)
}

func (s *Server) embedFSRegister(h *server.Hertz) error {
	bp := s.basePath
	if bp == "/" {
		bp = ""
	}

	subFS, err := fs.Sub(s.embedFS, s.localDir)
	if err != nil {
		return err
	}

	if s.is404ToHome {
		h.NoRoute(handleNotFoundFS(subFS, "index.html", s.basePath))
	}

	relativePath := bp + "/*filepath"
	handlerFunc := func(ctx context.Context, c *app.RequestContext) {
		filePath := strings.TrimPrefix(c.Param("filepath"), "/")
		if filePath == "" {
			filePath = "index.html"
		}

		status := checkAllowListFilesFS(subFS, filePath, s.enableListFiles)
		if status > 0 {
			c.SetStatusCode(status)
			return
		}

		content, err := fs.ReadFile(subFS, filePath)
		if err != nil {
			// If file not found, let NoRoute handle it or return 404
			c.AbortWithStatus(consts.StatusNotFound)
			return
		}

		s.setCacheHeader(c, filePath)
		// Hertz automatically detects content-type by extension
		ext := filepath.Ext(filePath)
		if ext != "" {
			c.SetContentType(getMimeType(ext))
		}
		_, _ = c.Write(content)
	}

	h.GET(relativePath, handlerFunc)
	h.HEAD(relativePath, handlerFunc)
	return nil
}

// GetBasePath returns the base path.
func (s *Server) GetBasePath() string {
	return s.basePath
}

// GetLocalDir returns the local directory path.
func (s *Server) GetLocalDir() string {
	return s.localDir
}

// --- Helper functions (mostly unchanged logic, adapted for Hertz) ---

func (s *Server) setCacheHeader(c *app.RequestContext, filePath string) {
	if s.cacheMaxAge <= 0 {
		return
	}
	file := strings.ToLower(filepath.Base(filePath))
	ext := strings.ToLower(filepath.Ext(file))

	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico",
		".woff", ".woff2", ".ttf", ".eot", ".mp4", ".webm", ".mp3":
		c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, must-revalidate", s.cacheMaxAge))
	case ".css", ".js":
		if hasHashFileName(file) {
			c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", s.cacheMaxAge))
		} else {
			c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, must-revalidate", s.cacheMaxAge))
		}
	default:
		c.Header("Cache-Control", "no-cache")
	}
}

func handleNotFound(indexHTML string, basePath string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if strings.HasPrefix(string(c.Request.URI().Path()), basePath) {
			if content, err := os.ReadFile(indexHTML); err == nil {
				c.SetContentType("text/html; charset=utf-8")
				c.SetStatusCode(consts.StatusOK)
				_, _ = c.Write(content)
				return
			}
		}
		c.String(consts.StatusNotFound, "404 Not Found")
	}
}

func handleNotFoundFS(subFS fs.FS, indexFile string, basePath string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if strings.HasPrefix(string(c.Request.URI().Path()), basePath) {
			if content, err := fs.ReadFile(subFS, indexFile); err == nil {
				c.SetContentType("text/html; charset=utf-8")
				c.SetStatusCode(consts.StatusOK)
				_, _ = c.Write(content)
				return
			}
		}
		c.String(consts.StatusNotFound, "404 Not Found")
	}
}

func (s *Server) saveFSToLocal() error {
	_ = os.RemoveAll(s.localDir)
	_ = os.MkdirAll(s.localDir, 0o755)
	return fs.WalkDir(s.embedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == "." {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(path, 0o755)
		}
		content, err := fs.ReadFile(s.embedFS, path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, content, 0o644)
	})
}

func (s *Server) injectFileContent() error {
	if s.injectFileContentMap == nil {
		return nil
	}

	filePaths, err := ListFiles(s.localDir)
	if err != nil {
		return err
	}

	for _, filePath := range filePaths {
		for file, fns := range s.injectFileContentMap {
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

func checkAllowListFiles(fullPath string, enableListFiles bool) (int, string) {
	if enableListFiles {
		return 0, fullPath
	}
	fi, err := os.Stat(fullPath)
	if err != nil {
		return consts.StatusNotFound, ""
	}
	if fi.IsDir() {
		fullPath = filepath.Join(fullPath, "index.html")
		if _, err := os.Stat(fullPath); err != nil {
			return consts.StatusForbidden, ""
		}
	}
	return 0, fullPath
}

func checkAllowListFilesFS(fsys fs.FS, filePath string, enableListFiles bool) int {
	if enableListFiles {
		return 0
	}
	filePath = strings.Trim(filePath, "/")
	if filePath == "" {
		filePath = "."
	}
	fi, err := fs.Stat(fsys, filePath)
	if err != nil {
		return consts.StatusNotFound
	}
	if fi.IsDir() {
		if _, err := fs.Stat(fsys, path.Join(filePath, "index.html")); err != nil {
			return consts.StatusForbidden
		}
	}
	return 0
}

func normalizeBasePath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath != "" {
		basePath = "/" + strings.TrimPrefix(basePath, "/")
		basePath = filepath.ToSlash(filepath.Clean(basePath))
		basePath = strings.TrimSuffix(basePath, "/")
	}
	if basePath == "" {
		basePath = "/"
	}
	return basePath
}

func countEmbedFS(efs embed.FS) (count int, err error) {
	err = fs.WalkDir(efs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		count++
		return nil
	})
	return count, err
}

func getEmbedDir(efs embed.FS) string {
	curr := "."
	base := ""
	for {
		entries, err := efs.ReadDir(curr)
		if err != nil || len(entries) != 1 || !entries[0].IsDir() {
			break
		}
		if base == "" {
			base = entries[0].Name()
		} else {
			base = base + "/" + entries[0].Name()
		}
		curr = base
	}
	return base
}

func isExists(dirPath string) bool {
	_, err := os.Stat(dirPath)
	return err == nil || os.IsExist(err)
}

func hasHashFileName(name string) bool {
	ext := filepath.Ext(name)
	name = strings.TrimSuffix(strings.ToLower(name), ext)
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '.' || r == '-' })
	if len(parts) < 2 {
		return false
	}
	last := parts[len(parts)-1]
	if len(last) < 6 {
		return false
	}
	for _, c := range last {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c == '_')) {
			return false
		}
	}
	return true
}

func isSuffixPath(p1, p2 string) bool {
	if p2 == "" {
		return false
	}
	norm := func(p string) string {
		p = strings.ReplaceAll(p, `\`, `/`)
		if len(p) >= 2 && p[1] == ':' {
			p = p[2:]
		}
		p = path.Clean("/" + p)
		return p
	}
	return strings.HasSuffix(norm(p1), norm(p2))
}

func getMimeType(ext string) string {
	switch strings.ToLower(ext) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func isWritable(dir string) bool {
	testFile := filepath.Join(dir, ".write_test")

	f, err := os.OpenFile(
		testFile,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0o644,
	)
	if err != nil {
		return false
	}

	_ = f.Close()
	_ = os.Remove(testFile)

	return true
}
