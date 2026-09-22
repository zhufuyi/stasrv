package spa

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const (
	// deleteRouteSuffix is appended to the location path to form the delete API route.
	deleteRouteSuffix = "/delete"

	// deleteFileParam is the name of the path parameter carrying the file to delete.
	deleteFileParam = "filename"
)

// deleteResult is the response body of a successful deletion.
type deleteResult struct {
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
}

// deleteRoutePrefix is the URL path the delete API is mounted under, e.g. '/docs/delete'.
func deleteRoutePrefix(basePath string) string {
	return strings.TrimSuffix(basePath, "/") + deleteRouteSuffix
}

// deleteRoute returns the Hertz pattern of the delete API, e.g. '/docs/delete/:filename'.
// A single path parameter is used on purpose: it cannot match a slash, so the request can
// never address a path deeper than the upload directory.
func deleteRoute(basePath string) string {
	return deleteRoutePrefix(basePath) + "/:" + deleteFileParam
}

// handleDelete removes one file from the upload directory of the location.
func (s *Server) handleDelete(c *app.RequestContext) {
	name, uerr := sanitizeUploadName(c.Param(deleteFileParam))
	if uerr != nil {
		abortUpload(c, uerr)
		return
	}

	fullPath := filepath.Join(s.uploadDir, name)
	fi, err := os.Stat(fullPath)
	switch {
	case os.IsNotExist(err):
		abortUpload(c, &uploadError{
			consts.StatusNotFound,
			fmt.Sprintf("file '%s' not found", name),
		})
		return
	case err != nil:
		abortUpload(c, &uploadError{
			consts.StatusInternalServerError,
			fmt.Sprintf("read file '%s' error: %v", name, err),
		})
		return
	case fi.IsDir():
		abortUpload(c, &uploadError{
			consts.StatusBadRequest,
			fmt.Sprintf("'%s' is a directory, only files can be deleted", name),
		})
		return
	}

	if err := os.Remove(fullPath); err != nil {
		abortUpload(c, &uploadError{
			consts.StatusInternalServerError,
			fmt.Sprintf("delete file '%s' error: %v", name, err),
		})
		return
	}

	c.JSON(consts.StatusOK, deleteResult{Name: name, Deleted: true})
}
