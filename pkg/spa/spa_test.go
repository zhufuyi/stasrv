package spa

import (
	"embed"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/*
var testEmbedFS embed.FS

func newTmpDir(t *testing.T) string {
	// mock index.html
	dir := t.TempDir()
	indexContent := []byte(`<!DOCTYPE html>
<html>
	<head>
		<title>First Page</title>
	</head>
	<body>
		<h1>Hello World!</h1>
	</body>
</html>
`)
	err := os.WriteFile(filepath.Join(dir, "index.html"), indexContent, 0o644)
	require.NoError(t, err)

	// mock config.js
	configContent := []byte(`
const appConfig = {
  api_base: "/api/v1",
};
`)
	err = os.WriteFile(filepath.Join(dir, "config.js"), configContent, 0o644)
	require.NoError(t, err)

	return dir
}

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNormalizeBasePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "/"},
		{"/", "/"},
		{"/api", "/api"},
		{"api/", "/api"},
		{"/api/", "/api"},
		{"  /api/v1/  ", "/api/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeBasePath(tt.input))
		})
	}
}

func TestIsExists(t *testing.T) {
	assert.True(t, isExists("spa.go"))
	assert.False(t, isExists("non_existent_file_12345.go"))
}

func Test_Options(t *testing.T) {
	// Test default
	o := defaultOptions()
	assert.True(t, o.is404ToHome)
	assert.Nil(t, o.injectFileContentMap)

	// Test With404ToHome
	opt1 := With404ToHome(true)
	opt1(o)
	assert.True(t, o.is404ToHome)

	// Test WithInjectFileContent
	opt2 := WithInjectFileContentByString("java", "golang", "index.html")
	opt3 := WithInjectFileContentByRegular(
		`const\s+BASE_URL\s*=\s*"[^"]*"\s*;`,
		`const BASE_URL = "/myapp/api/v1";`,
		"config.js",
	)

	opt2(o)
	opt3(o)

	assert.Contains(t, o.injectFileContentMap, "config.js")
	assert.Contains(t, o.injectFileContentMap, "index.html")

	opt4 := WithInjectFileContentByString("php", "golang", "index.html")
	opt4(o)
	assert.Equal(t, 2, len(o.injectFileContentMap))

	// not specified file
	opt5 := WithInjectFileContentByString("java", "golang")
	opt6 := WithInjectFileContentByRegular(
		`const\s+BASE_URL\s*=\s*"[^"]*"\s*;`,
		`const BASE_URL = "/myapp/api/v1";`,
	)
	o2 := defaultOptions()
	opt5(o2)
	opt6(o2)
	assert.Nil(t, o2.injectFileContentMap)
}

func TestNewLocal(t *testing.T) {
	// Exists dir
	dir := newTmpDir(t)
	fe, err := NewLocal(dir, "/app",
		With404ToHome(true),
		WithListFiles(true),
		WithCacheMaxAge(86400),
	)
	assert.NoError(t, err)
	assert.NotNil(t, fe)
	assert.Equal(t, dir, fe.localDir)
	assert.Equal(t, "/app", fe.basePath)
	assert.True(t, fe.is404ToHome)

	// Not exists dir
	_, err = NewLocal("non_existent_dir", "/")
	assert.Error(t, err)
}

func TestNewEmbedFS(t *testing.T) {
	f, err := NewEmbedFS(testEmbedFS, "/app",
		With404ToHome(false),
		WithListFiles(false),
		WithCacheMaxAge(86400),
	)
	assert.Nil(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, "testdata", f.localDir)
	assert.Equal(t, "/app", f.basePath)
}

func TestLocalRegister_And_Routing(t *testing.T) {
	dir := newTmpDir(t)

	fe, err := NewLocal(dir, "/static",
		With404ToHome(true),
		WithCacheMaxAge(86400),
		WithInjectFileContentByRegular(`<title>.*?</title>`, "<title>golang</title>", "index.html"),
		WithInjectFileContentByRegular(`api_base\s*:\s*"[^"]*"`, `api_base: "/myapp/api"`, "config.js"),
	)
	require.NoError(t, err)

	r := gin.New()
	err = fe.Register(r)
	assert.NoError(t, err)

	// Test 1: Fetch existing file directly
	url := "/static/"
	req, _ := http.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "<h1>Hello World!</h1>")

	// Test 2: Fetch handled file (config.js)
	req, _ = http.NewRequest("GET", "/static/config.js", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "/myapp/api") // File should have been modified

	// Test 4: 404 WITHOUT Accept HTML (Should return standard 404)
	req, _ = http.NewRequest("GET", "/static/not-exist-route", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleNotFound(t *testing.T) {
	// Test the browserRefreshFS middleware standalone
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create a fake request with Accept text/html
	c.Request, _ = http.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Accept", "text/html")

	handler := handleNotFound("testdata/index.html", "/")
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Hello World")
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))

	// Test fallback to 404 if file missing in embedFS
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request, _ = http.NewRequest("GET", "/", nil)
	c2.Request.Header.Set("Accept", "text/html")

	handler2 := handleNotFound("non_existent.html", "/")
	handler2(c2)

	assert.Equal(t, http.StatusNotFound, w2.Code)
	assert.Equal(t, "404 Not Found", w2.Body.String())
}

func TestHandleNotFoundFS(t *testing.T) {
	// Test the browserRefreshFS middleware standalone
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create a fake request with Accept text/html
	c.Request, _ = http.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Accept", "text/html")

	handler := handleNotFoundFS(testEmbedFS, "testdata/index.html", "/")
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Hello World")
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))

	// Test fallback to 404 if file missing in embedFS
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request, _ = http.NewRequest("GET", "/", nil)
	c2.Request.Header.Set("Accept", "text/html")

	handler2 := handleNotFoundFS(testEmbedFS, "non_existent.html", "/")
	handler2(c2)

	assert.Equal(t, http.StatusNotFound, w2.Code)
	assert.Equal(t, "404 Not Found", w2.Body.String())
}

func TestEmbedFSRegister_Direct(t *testing.T) {
	f, err := NewEmbedFS(testEmbedFS, "/app")
	require.NoError(t, err)

	r := gin.New()
	err = f.Register(r)
	assert.NoError(t, err)

	// Request an embedded file
	req, _ := http.NewRequest("GET", "/app/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Hello World")

	req, _ = http.NewRequest("GET", "/not-exist-path", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestEmbedFS_Register_WithInjectFileContent(t *testing.T) {
	fe, err := NewEmbedFS(testEmbedFS, "/app",
		WithInjectFileContentByString("Hello World", "Hello World", "index.html"),
	)
	require.NoError(t, err)

	r := gin.New()
	err = fe.Register(r)
	assert.NoError(t, err)

	// Request an embedded file
	req, _ := http.NewRequest("GET", "/app/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Hello World")
}

func TestWithCacheMaxAge(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int
		expected int
	}{
		{
			name:     "positive seconds",
			seconds:  3600,
			expected: 3600,
		},
		{
			name:     "zero seconds",
			seconds:  0,
			expected: 0,
		},
		{
			name:     "negative seconds",
			seconds:  -100,
			expected: 0,
		},
		{
			name:     "one year",
			seconds:  31536000,
			expected: 31536000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := defaultOptions()

			opt := WithCacheMaxAge(tt.seconds)
			opt(o)

			if o.cacheMaxAge != tt.expected {
				t.Fatalf(
					"cacheMaxAge = %d, want %d",
					o.cacheMaxAge,
					tt.expected,
				)
			}
		})
	}
}

func TestHasHashFileName(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     bool
	}{
		{
			name:     "hash js with dot",
			fileName: "app.a1b2c3.js",
			want:     true,
		},
		{
			name:     "hash css with dash",
			fileName: "app-89afde.css",
			want:     true,
		},
		{
			name:     "long numeric hash",
			fileName: "vendor.123456789.js",
			want:     true,
		},
		{
			name:     "uppercase hash",
			fileName: "app.ABCDEF.js",
			want:     true,
		},
		{
			name:     "underline hash",
			fileName: "app.abc_123.js",
			want:     true,
		},
		{
			name:     "normal js",
			fileName: "app.js",
			want:     false,
		},
		{
			name:     "normal css",
			fileName: "main.css",
			want:     false,
		},
		{
			name:     "runtime js",
			fileName: "runtime.js",
			want:     false,
		},
		{
			name:     "config js",
			fileName: "config.js",
			want:     false,
		},
		{
			name:     "short hash",
			fileName: "app.a1b.js",
			want:     false,
		},
		{
			name:     "non hex hash",
			fileName: "app.xyz/123.js",
			want:     false,
		},
		{
			name:     "png image",
			fileName: "logo.png",
			want:     false,
		},
		{
			name:     "woff2 font",
			fileName: "font.woff2",
			want:     false,
		},
		{
			name:     "multiple dots",
			fileName: "app.bundle.a1b2c3.js",
			want:     true,
		},
		{
			name:     "multiple dashes",
			fileName: "app-main-abcdef.css",
			want:     true,
		},
		{
			name:     "mixed separators",
			fileName: "app.main-a1b2c3.js",
			want:     true,
		},
		{
			name:     "empty",
			fileName: "",
			want:     false,
		},
		{
			name:     "no extension",
			fileName: "abcdef",
			want:     false,
		},
		{
			name:     "hidden file",
			fileName: ".a1b2c3",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasHashFileName(tt.fileName)
			if got != tt.want {
				t.Fatalf("hasHashFileName(%q) = %v, want %v",
					tt.fileName, got, tt.want)
			}
		})
	}
}

func TestSetCacheHeader(t *testing.T) {
	tests := []struct {
		name         string
		cacheMaxAge  int
		filePath     string
		wantContains string
		wantEmpty    bool
	}{
		{
			name:         "hashed js",
			cacheMaxAge:  31536000,
			filePath:     "/js/app.a1b2c3.js",
			wantContains: "public, max-age=31536000, immutable",
		},
		{
			name:         "hashed css",
			cacheMaxAge:  86400,
			filePath:     "/css/app-abcdef.css",
			wantContains: "public, max-age=86400, immutable",
		},
		{
			name:        "normal js no cache",
			cacheMaxAge: 31536000,
			filePath:    "/js/app.js",
			wantEmpty:   true,
		},
		{
			name:        "normal css no cache",
			cacheMaxAge: 31536000,
			filePath:    "/css/main.css",
			wantEmpty:   true,
		},
		{
			name:        "html no cache",
			cacheMaxAge: 31536000,
			filePath:    "/index.html",
			wantEmpty:   true,
		},
		{
			name:         "png cache",
			cacheMaxAge:  3600,
			filePath:     "/img/logo.png",
			wantContains: "public, max-age=3600",
		},
		{
			name:         "svg cache",
			cacheMaxAge:  3600,
			filePath:     "/img/logo.svg",
			wantContains: "public, max-age=3600",
		},
		{
			name:         "woff2 cache",
			cacheMaxAge:  3600,
			filePath:     "/fonts/font.woff2",
			wantContains: "public, max-age=3600",
		},
		{
			name:        "cache disabled",
			cacheMaxAge: 0,
			filePath:    "/js/app.a1b2c3.js",
			wantEmpty:   true,
		},
		{
			name:        "negative cache disabled",
			cacheMaxAge: -1,
			filePath:    "/img/logo.png",
			wantEmpty:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := buildCacheControlHeader(
				tt.cacheMaxAge,
				tt.filePath,
			)

			if tt.wantEmpty {
				if header != "" {
					t.Fatalf("expected empty header, got %q", header)
				}
				return
			}

			if header != tt.wantContains {
				t.Fatalf("expected %q, got %q",
					tt.wantContains, header)
			}
		})
	}
}

func buildCacheControlHeader(cacheMaxAge int, filePath string) string {
	if cacheMaxAge <= 0 {
		return ""
	}

	file := strings.ToLower(filepath.Base(filePath))
	ext := strings.ToLower(filepath.Ext(file))

	// never cache html
	if ext == ".html" {
		return ""
	}

	// image/font/media cache
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp",
		".svg", ".ico",
		".woff", ".woff2", ".ttf", ".eot",
		".mp4", ".webm", ".mp3":
		return fmt.Sprintf(
			"public, max-age=%d",
			cacheMaxAge,
		)
	}

	// only cache hashed js/css
	if ext == ".js" || ext == ".css" {
		if hasHashFileName(file) {
			return fmt.Sprintf(
				"public, max-age=%d, immutable",
				cacheMaxAge,
			)
		}
	}

	return ""
}
