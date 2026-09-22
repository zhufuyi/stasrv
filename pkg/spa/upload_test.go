package spa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	oneKB       = 1 << 10
	uploadMaxMB = 8 * oneKB
)

type uploadPart struct {
	name    string
	content string
}

// buildMultipart builds a multipart/form-data body carrying files under the given form field.
func buildMultipart(t *testing.T, field string, parts []uploadPart) (string, []byte) {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, p := range parts {
		fw, err := w.CreateFormFile(field, p.name)
		require.NoError(t, err)
		_, err = fw.Write([]byte(p.content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())

	return w.FormDataContentType(), buf.Bytes()
}

func postUpload(t *testing.T, h *server.Hertz, route, contentType string, body []byte) *ut.ResponseRecorder {
	t.Helper()

	return ut.PerformRequest(h.Engine, http.MethodPost, route,
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType})
}

func newUploadServer(t *testing.T, basePath string, maxBytes int64) (*server.Hertz, *Server, string) {
	t.Helper()

	dir := newTmpDir(t)
	srv, err := NewLocal(basePath, dir, With404ToHome(true), WithUploadMaxSize(maxBytes))
	require.NoError(t, err)

	h := server.New()
	require.NoError(t, srv.Register(h))

	return h, srv, dir
}

func decodeUploadResult(t *testing.T, body []byte) uploadResult {
	t.Helper()

	var res uploadResult
	require.NoError(t, json.Unmarshal(body, &res))
	return res
}

func Test_uploadRoute(t *testing.T) {
	tests := []struct {
		basePath string
		want     string
	}{
		{"/", "/upload"},
		{"/docs", "/docs/upload"},
		{"/app/assets", "/app/assets/upload"},
	}
	for _, tt := range tests {
		t.Run(tt.basePath, func(t *testing.T) {
			assert.Equal(t, tt.want, uploadRoute(tt.basePath))
		})
	}
}

func Test_sanitizeUploadName(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"plain json", "data.json", "data.json", false},
		{"plain pdf", "data.pdf", "data.pdf", false},
		{"plain image", "picture.jpg", "picture.jpg", false},
		{"chinese name", "报表 2026.xlsx", "报表 2026.xlsx", false},
		{"unix traversal", "../../etc/passwd", "passwd", false},
		{"windows path", `C:\Users\admin\Desktop\picture.jpg`, "picture.jpg", false},
		{"windows traversal", `..\..\Windows\system.ini`, "system.ini", false},
		{"absolute path", "/var/spool/cron/data.json", "data.json", false},
		{"trailing slash", "backup/", "backup", false},
		{"surrounding spaces", "  data.json  ", "data.json", false},
		{"empty", "", "", true},
		{"spaces only", "   ", "", true},
		{"dot dot", "..", "", true},
		{"dot", ".", "", true},
		{"drive letter left", "a:b", "", true},
		{"control char", "data\n.json", "", true},
		{"null byte", "data\x00.json", "", true},
		{"too long", strings.Repeat("a", maxUploadFileNameLen) + ".json", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, uerr := sanitizeUploadName(tt.raw)
			if tt.wantErr {
				assert.NotNil(t, uerr)
				assert.Empty(t, got)
				return
			}
			require.Nil(t, uerr)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestUpload_files_land_in_data_dir verifies the whole flow: the route derived from the
// location path, the storage under '<root>/data', and that the stored files are served back.
func TestUpload_files_land_in_data_dir(t *testing.T) {
	h, srv, dir := newUploadServer(t, "/docs", uploadMaxMB)

	assert.Equal(t, "/docs/upload", srv.GetUploadRoute())
	assert.Equal(t, filepath.Join(dir, "data"), srv.GetUploadDir())

	parts := []uploadPart{
		{"data.json", `{"key":"value"}`},
		{"report.pdf", "%PDF-1.4 fake pdf content"},
		{"picture.jpg", "\xff\xd8\xff\xe0 jpeg bytes"},
	}
	contentType, body := buildMultipart(t, uploadFormKey, parts)

	w := postUpload(t, h, "/docs/upload", contentType, body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	res := decodeUploadResult(t, w.Body.Bytes())
	require.Len(t, res.Files, len(parts))
	for i, item := range res.Files {
		assert.Equal(t, parts[i].name, item.Name)
		assert.Equal(t, "/docs/data/"+item.Name, item.URL)
		assert.Equal(t, int64(len(parts[i].content)), item.Size)

		saved, err := os.ReadFile(filepath.Join(dir, uploadDirName, parts[i].name))
		require.NoError(t, err)
		assert.Equal(t, parts[i].content, string(saved))
	}

	// the returned url is a static route of the same location, so the file is served back
	// from '<root>/data' by the existing catch-all handler
	assert.Equal(t, "/docs/data/report.pdf", res.Files[1].URL)
}

func TestUpload_root_location(t *testing.T) {
	h, srv, dir := newUploadServer(t, "/", uploadMaxMB)

	assert.Equal(t, "/upload", srv.GetUploadRoute())

	contentType, body := buildMultipart(t, uploadFormKey, []uploadPart{{"data.json", "{}"}})
	w := postUpload(t, h, "/upload", contentType, body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	res := decodeUploadResult(t, w.Body.Bytes())
	require.Len(t, res.Files, 1)
	assert.Equal(t, "/data/data.json", res.Files[0].URL)

	_, err := os.Stat(filepath.Join(dir, uploadDirName, "data.json"))
	assert.NoError(t, err)
}

func TestUpload_traversal_stays_in_data_dir(t *testing.T) {
	h, _, dir := newUploadServer(t, "/docs", uploadMaxMB)

	contentType, body := buildMultipart(t, uploadFormKey, []uploadPart{
		{"../../escape.txt", "escaped"},
	})
	w := postUpload(t, h, "/docs/upload", contentType, body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	res := decodeUploadResult(t, w.Body.Bytes())
	require.Len(t, res.Files, 1)
	assert.Equal(t, "escape.txt", res.Files[0].Name)

	// only the base name is kept, and it is written inside '<root>/data'
	_, err := os.Stat(filepath.Join(dir, "escape.txt"))
	assert.True(t, os.IsNotExist(err), "the file must not be written outside of the data directory")
	_, err = os.Stat(filepath.Join(dir, uploadDirName, "escape.txt"))
	assert.NoError(t, err)
}

func TestUpload_rejects_invalid_names(t *testing.T) {
	h, _, dir := newUploadServer(t, "/docs", uploadMaxMB)

	// a name whose base part is empty must be refused instead of silently renamed
	contentType, body := buildMultipart(t, uploadFormKey, []uploadPart{{"..", "payload"}})
	w := postUpload(t, h, "/docs/upload", contentType, body)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	names, err := os.ReadDir(filepath.Join(dir, uploadDirName))
	require.NoError(t, err)
	assert.Empty(t, names, "nothing must be stored for a rejected file name")
}

func TestUpload_bad_requests(t *testing.T) {
	h, _, _ := newUploadServer(t, "/docs", uploadMaxMB)

	t.Run("not a multipart request", func(t *testing.T) {
		body := []byte("data.json")
		w := postUpload(t, h, "/docs/upload", "text/plain", body)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("multipart without the file field", func(t *testing.T) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		fw, err := w.CreateFormField("another")
		require.NoError(t, err)
		_, err = fw.Write([]byte("value"))
		require.NoError(t, err)
		require.NoError(t, w.Close())

		rec := postUpload(t, h, "/docs/upload", w.FormDataContentType(), buf.Bytes())
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("empty multipart body", func(t *testing.T) {
		rec := ut.PerformRequest(h.Engine, http.MethodPost, "/docs/upload",
			&ut.Body{Body: strings.NewReader("garbage"), Len: len("garbage")},
			ut.Header{Key: "Content-Type", Value: "multipart/form-data; boundary=abc"})
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestUpload_oversize_file(t *testing.T) {
	h, _, dir := newUploadServer(t, "/docs", oneKB)

	content := strings.Repeat("a", 2*oneKB)
	contentType, body := buildMultipart(t, uploadFormKey, []uploadPart{{"data.json", content}})

	w := postUpload(t, h, "/docs/upload", contentType, body)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "maximum size")

	_, err := os.Stat(filepath.Join(dir, uploadDirName, "data.json"))
	assert.True(t, os.IsNotExist(err), "an oversize file must not be stored")
}

// TestUpload_oversize_rejects_the_whole_request covers that no file of a multi file request
// is stored when one of them is refused.
func TestUpload_oversize_rejects_the_whole_request(t *testing.T) {
	h, _, dir := newUploadServer(t, "/docs", oneKB)

	contentType, body := buildMultipart(t, uploadFormKey, []uploadPart{
		{"small.json", "{}"},
		{"large.json", strings.Repeat("a", 2*oneKB)},
	})
	w := postUpload(t, h, "/docs/upload", contentType, body)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	names, err := os.ReadDir(filepath.Join(dir, uploadDirName))
	require.NoError(t, err)
	assert.Empty(t, names, "the accepted file must not be stored either")
}

// TestUpload_coexists_with_static_routes is the route conflict guard: the static file routes of a
// location are registered for GET and HEAD only, so 'POST <path>/upload' never collides with the
// '<path>/*filepath' catch-all, and a GET on the upload path is still handled by the static server.
func TestUpload_coexists_with_static_routes(t *testing.T) {
	h, _, _ := newUploadServer(t, "/docs", uploadMaxMB)

	// GET on the upload path is served by the static catch-all, there is no such file
	w := ut.PerformRequest(h.Engine, http.MethodGet, "/docs/upload", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	contentType, body := buildMultipart(t, uploadFormKey, []uploadPart{{"data.json", "{}"}})
	w = postUpload(t, h, "/docs/upload", contentType, body)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestUpload_with_nested_locations covers locations whose paths overlap, e.g. '/docs' and
// '/docs/upload': both upload APIs and both static routes must be registered without conflict.
func TestUpload_with_nested_locations(t *testing.T) {
	outer := newTmpDir(t)
	inner := newTmpDir(t)

	srvOuter, err := NewLocal("/docs", outer, With404ToHome(true), WithUploadMaxSize(uploadMaxMB))
	require.NoError(t, err)
	srvInner, err := NewLocal("/docs/upload", inner, With404ToHome(true), WithUploadMaxSize(uploadMaxMB))
	require.NoError(t, err)

	h := server.New()
	require.NoError(t, srvOuter.Register(h))
	require.NoError(t, srvInner.Register(h))

	assert.Equal(t, "/docs/upload", srvOuter.GetUploadRoute())
	assert.Equal(t, "/docs/upload/upload", srvInner.GetUploadRoute())

	contentType, body := buildMultipart(t, uploadFormKey, []uploadPart{{"data.json", "{}"}})

	w := postUpload(t, h, "/docs/upload", contentType, body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	_, err = os.Stat(filepath.Join(outer, uploadDirName, "data.json"))
	assert.NoError(t, err)

	w = postUpload(t, h, "/docs/upload/upload", contentType, body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	_, err = os.Stat(filepath.Join(inner, uploadDirName, "data.json"))
	assert.NoError(t, err)
}

// TestUpload_without_options is the guard for the removed enable switch: a location
// serves the upload and delete APIs even when no upload option was passed.
func TestUpload_without_options(t *testing.T) {
	dir := newTmpDir(t)
	srv, err := NewLocal("/docs", dir, With404ToHome(true))
	require.NoError(t, err)

	h := server.New()
	require.NoError(t, srv.Register(h))

	assert.Equal(t, "/docs/upload", srv.GetUploadRoute())
	assert.Equal(t, "/docs/delete/<filename>", srv.GetDeleteRoute())
	assert.Equal(t, filepath.Join(dir, uploadDirName), srv.GetUploadDir())

	contentType, body := buildMultipart(t, uploadFormKey, []uploadPart{{"data.json", "{}"}})
	w := postUpload(t, h, "/docs/upload", contentType, body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	res := decodeUploadResult(t, w.Body.Bytes())
	require.Len(t, res.Files, 1)
	assert.Equal(t, "/docs/data/data.json", res.Files[0].URL)

	w = ut.PerformRequest(h.Engine, http.MethodDelete, "/docs/delete/data.json", nil)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestUpload_defaultMaxSize checks the cap applied when the caller does not tune it.
func TestUpload_defaultMaxSize(t *testing.T) {
	srv, err := NewLocal("/docs", newTmpDir(t), With404ToHome(true))
	require.NoError(t, err)
	assert.Equal(t, int64(defaultUploadMaxSize), srv.uploadMaxSize)
}

// TestUpload_embedFS_stores_next_to_the_binary covers the '-fs-base-path' type: the embedded
// files are read-only, so the uploaded files land in the 'data' directory of the executable
// and are served back from there.
func TestUpload_embedFS_stores_next_to_the_binary(t *testing.T) {
	srv, err := NewEmbedFS("/app", testEmbedFS)
	require.NoError(t, err)

	exe, err := os.Executable()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(filepath.Dir(exe), uploadDirName), srv.GetUploadDir())

	// write into a disposable directory instead of the one next to the test binary
	srv.uploadDir = t.TempDir()

	h := server.New()
	require.NoError(t, srv.Register(h))

	assert.Equal(t, "/app/upload", srv.GetUploadRoute())
	assert.Equal(t, "/app/delete/<filename>", srv.GetDeleteRoute())

	contentType, body := buildMultipart(t, uploadFormKey, []uploadPart{{"data.json", `{"uploaded":true}`}})
	w := postUpload(t, h, "/app/upload", contentType, body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	res := decodeUploadResult(t, w.Body.Bytes())
	require.Len(t, res.Files, 1)
	assert.Equal(t, "/app/data/data.json", res.Files[0].URL)

	saved, err := os.ReadFile(filepath.Join(srv.uploadDir, "data.json"))
	require.NoError(t, err)
	assert.Equal(t, `{"uploaded":true}`, string(saved))

	// the embedded files never contain it, so this proves it is served from disk
	w = ut.PerformRequest(h.Engine, http.MethodGet, "/app/data/data.json", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, `{"uploaded":true}`, w.Body.String())
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	w = ut.PerformRequest(h.Engine, http.MethodDelete, "/app/delete/data.json", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = ut.PerformRequest(h.Engine, http.MethodGet, "/app/data/data.json", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Test_serveUploadedFile covers the path shapes the embed.FS handler delegates to disk.
func Test_serveUploadedFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0o644))
	s := &Server{basePath: "/app", uploadDir: dir}

	tests := []struct {
		name     string
		filePath string
		served   bool
	}{
		{"uploaded file", "data/data.json", true},
		{"not the data directory", "index.html", false},
		{"the data directory itself", "data", false},
		{"empty name", "data/", false},
		{"nested path", "data/sub/data.json", false},
		{"traversal", "data/../index.html", false},
		{"missing file", "data/nope.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := app.NewContext(0)
			assert.Equal(t, tt.served, s.serveUploadedFile(c, tt.filePath))
			if tt.served {
				assert.Equal(t, "{}", string(c.Response.Body()))
			}
		})
	}
}

func TestUpload_directory_created_when_missing(t *testing.T) {
	dir := newTmpDir(t)
	srv, err := NewLocal("/docs", dir, With404ToHome(true), WithUploadMaxSize(uploadMaxMB))
	require.NoError(t, err)

	// the data directory does not exist yet, the server must create it on registration
	_, err = os.Stat(filepath.Join(dir, uploadDirName))
	require.True(t, os.IsNotExist(err))

	h := server.New()
	require.NoError(t, srv.Register(h))

	fi, err := os.Stat(srv.GetUploadDir())
	require.NoError(t, err)
	assert.True(t, fi.IsDir())
}

func TestUploadURL(t *testing.T) {
	tests := []struct {
		basePath string
		fileName string
		want     string
	}{
		{"/docs", "data.json", "/docs/data/data.json"},
		{"/", "data.json", "/data/data.json"},
		{"/docs", "a b.jpg", "/docs/data/a%20b.jpg"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s", tt.basePath, tt.fileName), func(t *testing.T) {
			s := &Server{basePath: normalizeBasePath(tt.basePath)}
			assert.Equal(t, tt.want, s.uploadURL(tt.fileName))
		})
	}
}
