package spa

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deleteRequest(t *testing.T, h *server.Hertz, route string) *ut.ResponseRecorder {
	t.Helper()

	return ut.PerformRequest(h.Engine, http.MethodDelete, route, nil)
}

// uploadOne stores a single file through the upload API of the location and returns its name.
func uploadOne(t *testing.T, h *server.Hertz, route, name, content string) string {
	t.Helper()

	contentType, body := buildMultipart(t, uploadFormKey, []uploadPart{{name, content}})
	w := postUpload(t, h, route, contentType, body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	res := decodeUploadResult(t, w.Body.Bytes())
	require.Len(t, res.Files, 1)

	return res.Files[0].Name
}

func Test_deleteRoute(t *testing.T) {
	tests := []struct {
		basePath string
		pattern  string
		display  string
	}{
		{"/", "/delete/:filename", "/delete/<filename>"},
		{"/docs", "/docs/delete/:filename", "/docs/delete/<filename>"},
		{"/app/assets", "/app/assets/delete/:filename", "/app/assets/delete/<filename>"},
	}
	for _, tt := range tests {
		t.Run(tt.basePath, func(t *testing.T) {
			s := &Server{basePath: normalizeBasePath(tt.basePath)}
			assert.Equal(t, tt.pattern, deleteRoute(tt.basePath))
			assert.Equal(t, tt.display, s.GetDeleteRoute())
		})
	}
}

// TestDelete_removes_the_uploaded_file is the whole cycle: upload, delete, and the
// confirmation that the public url of the file is gone.
func TestDelete_removes_the_uploaded_file(t *testing.T) {
	h, _, dir := newUploadServer(t, "/docs", uploadMaxMB)

	name := uploadOne(t, h, "/docs/upload", "data.json", `{"key":"value"}`)
	fullPath := filepath.Join(dir, uploadDirName, name)
	require.FileExists(t, fullPath)

	w := deleteRequest(t, h, "/docs/delete/data.json")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var res deleteResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, "data.json", res.Name)
	assert.True(t, res.Deleted)

	assert.False(t, pathExists(fullPath), "the file must be removed from '<root>/data'")

	w = ut.PerformRequest(h.Engine, http.MethodGet, "/docs/data/data.json", nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "the url of a deleted file must not answer anymore")

	// deleting twice is not a silent success
	w = deleteRequest(t, h, "/docs/delete/data.json")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
}

// TestDelete_only_touches_the_data_directory covers that whatever the client sends, only a
// plain file of the upload directory can be removed.
func TestDelete_only_touches_the_data_directory(t *testing.T) {
	h, _, dir := newUploadServer(t, "/docs", uploadMaxMB)

	// an upload of a path-like name is stored as a plain file inside '<root>/data'
	uploadOne(t, h, "/docs/upload", "../../index.html", "stored in data")
	require.FileExists(t, filepath.Join(dir, uploadDirName, "index.html"))

	w := deleteRequest(t, h, "/docs/delete/index.html")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.False(t, pathExists(filepath.Join(dir, uploadDirName, "index.html")))
	assert.FileExists(t, filepath.Join(dir, "index.html"), "the served directory must stay untouched")

	for _, route := range []string{
		"/docs/delete/..%2Findex.html",
		"/docs/delete/..%2F..%2Fetc%2Fpasswd",
		"/docs/delete/sub/index.html",
	} {
		w = deleteRequest(t, h, route)
		assert.NotContains(t, w.Body.String(), `"deleted"`, route)
	}
	assert.FileExists(t, filepath.Join(dir, "index.html"))
}

func TestDelete_missing_file(t *testing.T) {
	h, _, _ := newUploadServer(t, "/docs", uploadMaxMB)

	w := deleteRequest(t, h, "/docs/delete/nope.json")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "nope.json")
}

func TestDelete_directory_refused(t *testing.T) {
	h, _, dir := newUploadServer(t, "/docs", uploadMaxMB)

	require.NoError(t, os.Mkdir(filepath.Join(dir, uploadDirName, "sub"), uploadDirPerm))

	w := deleteRequest(t, h, "/docs/delete/sub")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "directory")

	_, err := os.Stat(filepath.Join(dir, uploadDirName, "sub"))
	assert.NoError(t, err, "a directory must survive a delete request")
}

func TestDelete_invalid_names(t *testing.T) {
	h, _, _ := newUploadServer(t, "/docs", uploadMaxMB)

	tests := []struct {
		name  string
		route string
	}{
		{"spaces only", "/docs/delete/%20%20"},
		{"drive letter", "/docs/delete/a%3Ab.json"},
		{"newline", "/docs/delete/a%0Ab.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := deleteRequest(t, h, tt.route)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.NotContains(t, w.Body.String(), `"deleted"`)
		})
	}
}

// TestDelete_coexists_with_static_routes is the route conflict guard for the delete API:
// the static catch-all of a location is a GET/HEAD route, so the DELETE route of the same
// prefix is a separate tree, and a GET on a delete path is not a deletion. An uploaded file
// is served by a plain read, so it can be read back and deleted one after the other.
func TestDelete_coexists_with_static_routes(t *testing.T) {
	h, _, dir := newUploadServer(t, "/docs", uploadMaxMB)

	uploadOne(t, h, "/docs/upload", "data.json", "{}")

	w := ut.PerformRequest(h.Engine, http.MethodGet, "/docs/delete/data.json", nil)
	assert.NotEqual(t, http.StatusOK, w.Code)
	require.FileExists(t, filepath.Join(dir, uploadDirName, "data.json"))

	w = ut.PerformRequest(h.Engine, http.MethodGet, "/docs/data/data.json", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "{}", w.Body.String())

	w = deleteRequest(t, h, "/docs/delete/data.json")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = ut.PerformRequest(h.Engine, http.MethodGet, "/docs/data/data.json", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDelete_with_nested_locations registers '/docs' and '/docs/delete' together, which puts a
// ':filename' parameter and a static segment at the same position of the DELETE tree. Hertz
// keeps both, so each location must delete from its own data directory.
func TestDelete_with_nested_locations(t *testing.T) {
	outer := newTmpDir(t)
	inner := newTmpDir(t)

	srvOuter, err := NewLocal("/docs", outer, With404ToHome(true))
	require.NoError(t, err)
	srvInner, err := NewLocal("/docs/delete", inner, With404ToHome(true))
	require.NoError(t, err)

	h := server.New()
	require.NoError(t, srvOuter.Register(h))
	require.NoError(t, srvInner.Register(h))

	uploadOne(t, h, "/docs/upload", "data.json", "outer")
	uploadOne(t, h, "/docs/delete/upload", "data.json", "inner")

	w := deleteRequest(t, h, "/docs/delete/data.json")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.False(t, pathExists(filepath.Join(outer, uploadDirName, "data.json")))
	assert.FileExists(t, filepath.Join(inner, uploadDirName, "data.json"))
}

// Test_binaryUploadDir checks the directory an embed.FS location falls back to.
func Test_binaryUploadDir(t *testing.T) {
	got, err := binaryUploadDir()
	require.NoError(t, err)

	exe, err := os.Executable()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(filepath.Dir(exe), uploadDirName), got)
	assert.Equal(t, uploadDirName, filepath.Base(got))
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
