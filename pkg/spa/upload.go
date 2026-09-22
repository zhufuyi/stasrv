package spa

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const (
	// uploadRouteSuffix is appended to the location path to form the upload API route.
	uploadRouteSuffix = "/upload"

	// uploadDirName is the directory that receives the uploaded files.
	uploadDirName = "data"

	// uploadFormKey is the multipart form field that carries the files.
	uploadFormKey = "file"

	// maxUploadFileNameLen is the maximum length of an accepted file name.
	maxUploadFileNameLen = 255

	// uploadDirPerm matches the permissions used for the served directories.
	uploadDirPerm = 0o755

	// defaultUploadMaxSize is the default cap of a single uploaded file, in bytes.
	defaultUploadMaxSize = 32 << 20
)

// uploadItem describes a file that was stored by the upload API.
type uploadItem struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

// uploadResult is the response body of a successful upload.
type uploadResult struct {
	Files []uploadItem `json:"files"`
}

// uploadError carries the HTTP status to return together with the failure reason.
// It deliberately does not implement error, a nil *uploadError must never end up in an error interface.
type uploadError struct {
	status  int
	message string
}

// uploadRoute returns the URL path of the upload API for a normalized base path.
// The location '/' maps to '/upload', the location '/docs' maps to '/docs/upload'.
func uploadRoute(basePath string) string {
	return strings.TrimSuffix(basePath, "/") + uploadRouteSuffix
}

// binaryUploadDir returns the '<dir of the executable>/data' directory, used by an
// embed.FS location, whose own files are read-only.
func binaryUploadDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find the executable path for the upload directory error: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), uploadDirName), nil
}

// fileAPIRegister creates the storage directory and mounts the upload and delete APIs
// of the location. The routes use the location path, so they never collide with the
// static routes: those are registered for GET and HEAD only.
func (s *Server) fileAPIRegister(h *server.Hertz) error {
	if err := os.MkdirAll(s.uploadDir, uploadDirPerm); err != nil {
		return fmt.Errorf("create upload directory '%s' error: %w", s.uploadDir, err)
	}
	h.POST(uploadRoute(s.basePath), func(_ context.Context, c *app.RequestContext) {
		s.handleUpload(c)
	})
	h.DELETE(deleteRoute(s.basePath), func(_ context.Context, c *app.RequestContext) {
		s.handleDelete(c)
	})
	return nil
}

func (s *Server) handleUpload(c *app.RequestContext) {
	form, err := c.MultipartForm()
	if err != nil || form == nil {
		abortUpload(c, &uploadError{
			consts.StatusBadRequest,
			fmt.Sprintf("a multipart/form-data request is required, the files must be sent in the '%s' form field", uploadFormKey),
		})
		return
	}

	headers := form.File[uploadFormKey]
	if len(headers) == 0 {
		abortUpload(c, &uploadError{
			consts.StatusBadRequest,
			fmt.Sprintf("no file found in the '%s' form field", uploadFormKey),
		})
		return
	}

	// check the whole request first, so a rejected file cannot leave the others stored
	names := make([]string, len(headers))
	for i, header := range headers {
		name, uerr := s.checkUpload(header)
		if uerr != nil {
			abortUpload(c, uerr)
			return
		}
		names[i] = name
	}

	items := make([]uploadItem, 0, len(headers))
	for i, header := range headers {
		item, uerr := s.writeUpload(c, header, names[i])
		if uerr != nil {
			abortUpload(c, uerr)
			return
		}
		items = append(items, item)
	}

	c.JSON(consts.StatusOK, uploadResult{Files: items})
}

// checkUpload validates the file name and size of one uploaded file and returns its stored name.
func (s *Server) checkUpload(header *multipart.FileHeader) (string, *uploadError) {
	name, uerr := sanitizeUploadName(header.Filename)
	if uerr != nil {
		return "", uerr
	}
	if s.uploadMaxSize > 0 && header.Size > s.uploadMaxSize {
		return "", &uploadError{
			consts.StatusRequestEntityTooLarge,
			fmt.Sprintf("file '%s' is %d bytes, the maximum size is %d bytes", name, header.Size, s.uploadMaxSize),
		}
	}
	return name, nil
}

// writeUpload stores one uploaded file, overwriting a previous file of the same name.
func (s *Server) writeUpload(c *app.RequestContext, header *multipart.FileHeader, name string) (uploadItem, *uploadError) {
	if err := c.SaveUploadedFile(header, filepath.Join(s.uploadDir, name)); err != nil {
		return uploadItem{}, &uploadError{
			consts.StatusInternalServerError,
			fmt.Sprintf("save file '%s' error: %v", name, err),
		}
	}
	return uploadItem{Name: name, Size: header.Size, URL: s.uploadURL(name)}, nil
}

// uploadURL builds the public URL an uploaded file is served from, which is the
// static route of the same location: '<basePath>/data/<file>'.
func (s *Server) uploadURL(name string) string {
	return strings.TrimSuffix(s.basePath, "/") + "/" + uploadDirName + "/" + url.PathEscape(name)
}

// serveUploadedFile answers a '<basePath>/data/<file>' request from the upload directory
// and reports whether it wrote the response. An embed.FS location needs it because its
// embedded files are read-only, so the uploaded ones live outside of them.
func (s *Server) serveUploadedFile(c *app.RequestContext, filePath string) bool {
	name, ok := uploadedName(filePath)
	if !ok {
		return false
	}
	content, err := os.ReadFile(filepath.Join(s.uploadDir, name))
	if err != nil {
		return false
	}
	s.setCacheHeader(c, name)
	c.SetContentType(getMimeType(filepath.Ext(name)))
	_, _ = c.Write(content)
	return true
}

// uploadedName extracts the stored file name of a '<uploadDirName>/<file>' request path,
// and refuses anything that addresses more than one plain file of the upload directory.
func uploadedName(filePath string) (string, bool) {
	prefix := uploadDirName + "/"
	if !strings.HasPrefix(filePath, prefix) {
		return "", false
	}
	requested := strings.TrimPrefix(filePath, prefix)

	name, uerr := sanitizeUploadName(requested)
	if uerr != nil || name != requested {
		return "", false
	}
	return name, true
}

// sanitizeUploadName turns a client supplied file name into a plain base name, so that
// an upload can never write outside of the upload directory.
func sanitizeUploadName(raw string) (string, *uploadError) {
	// the separators are handled here rather than with filepath.Base, whose result
	// depends on the OS the server runs on, not on the client that sent the name
	name := uploadBaseName(raw)
	name = strings.TrimSpace(name)

	switch {
	case name == "" || name == "." || name == "..":
		return "", &uploadError{consts.StatusBadRequest, "the file name is empty"}
	case len(name) > maxUploadFileNameLen:
		return "", &uploadError{
			consts.StatusBadRequest,
			fmt.Sprintf("the file name '%s' is longer than %d bytes", name, maxUploadFileNameLen),
		}
	case strings.ContainsRune(name, ':'):
		// a colon is a path separator on some systems and part of a drive letter on windows
		return "", &uploadError{consts.StatusBadRequest, fmt.Sprintf("invalid file name '%s'", name)}
	case strings.ContainsFunc(name, unicode.IsControl):
		return "", &uploadError{consts.StatusBadRequest, fmt.Sprintf("invalid file name '%s'", name)}
	}

	return name, nil
}

// uploadBaseName returns the last element of a '/' or '\' separated path.
func uploadBaseName(raw string) string {
	name := strings.ReplaceAll(raw, `\`, "/")
	name = strings.TrimRight(name, "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

func abortUpload(c *app.RequestContext, uerr *uploadError) {
	c.AbortWithStatusJSON(uerr.status, map[string]any{"error": uerr.message})
}
