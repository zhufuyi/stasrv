package spa

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	prefix  = "prefix"
	suffix  = "suffix"
	contain = "contain"
)

var defaultFilterType = "" // with prefix, suffix, contain, no filter by default

type fileFileOptions struct {
	filter string
	name   string

	noAbsolutePath bool
}

func defaultFileOptions() *fileFileOptions {
	return &fileFileOptions{
		filter: defaultFilterType,
	}
}

// FileOption set the file fileFileOptions.
type FileOption func(*fileFileOptions)

func (o *fileFileOptions) apply(opts ...FileOption) {
	for _, opt := range opts {
		opt(o)
	}
}

// WithSuffix set suffix matching
func WithSuffix(name string) FileOption {
	return func(o *fileFileOptions) {
		o.filter = suffix
		o.name = name
	}
}

// WithPrefix set prefix matching
func WithPrefix(name string) FileOption {
	return func(o *fileFileOptions) {
		o.filter = prefix
		o.name = name
	}
}

// WithContain set contain matching
func WithContain(name string) FileOption {
	return func(o *fileFileOptions) {
		o.filter = contain
		o.name = name
	}
}

// WithNoAbsolutePath set no absolute path
func WithNoAbsolutePath() FileOption {
	return func(o *fileFileOptions) {
		o.noAbsolutePath = true
	}
}

// ----------------------------------------------------------------

// ListFiles iterates over all files in the specified directory, returning the absolute path to the file
func ListFiles(dirPath string, opts ...FileOption) ([]string, error) {
	files := []string{}
	err := error(nil)

	o := defaultFileOptions()
	o.apply(opts...)

	if !o.noAbsolutePath {
		dirPath, err = filepath.Abs(dirPath)
		if err != nil {
			return files, err
		}
	}

	switch o.filter {
	case prefix:
		return files, walkDirWithFilter(dirPath, &files, matchPrefix(o.name))
	case suffix:
		return files, walkDirWithFilter(dirPath, &files, matchSuffix(o.name))
	case contain:
		return files, walkDirWithFilter(dirPath, &files, matchContain(o.name))
	}

	return files, walkDir(dirPath, &files)
}

type filterFn func(string) bool

func GetPathDelimiter() string {
	delimiter := "/"
	if runtime.GOOS == "windows" {
		delimiter = "\\"
	}

	return delimiter
}

func walkDirWithFilter(dirPath string, allFiles *[]string, filter filterFn) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		deepFile := dirPath + GetPathDelimiter() + file.Name()
		if file.IsDir() {
			err = walkDirWithFilter(deepFile, allFiles, filter)
			if err != nil {
				return err
			}
			continue
		}
		if filter(deepFile) {
			*allFiles = append(*allFiles, deepFile)
		}
	}

	return nil
}

func walkDir(dirPath string, allFiles *[]string) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		deepFile := dirPath + GetPathDelimiter() + file.Name()
		if file.IsDir() {
			err = walkDir(deepFile, allFiles)
			if err != nil {
				return err
			}
			continue
		}
		*allFiles = append(*allFiles, deepFile)
	}

	return nil
}

func matchPrefix(prefixName string) filterFn {
	return func(filePath string) bool {
		if prefixName == "" {
			return false
		}
		filename := GetFilename(filePath)
		size := len(filename) - len(prefixName)
		if size >= 0 && filename[:len(prefixName)] == prefixName {
			return true
		}
		return false
	}
}

func matchSuffix(suffixName string) filterFn {
	return func(filename string) bool {
		if suffixName == "" {
			return false
		}

		size := len(filename) - len(suffixName)
		if size >= 0 && filename[size:] == suffixName {
			return true
		}
		return false
	}
}

func matchContain(containName string) filterFn {
	return func(filePath string) bool {
		if containName == "" {
			return false
		}
		filename := GetFilename(filePath)
		return strings.Contains(filename, containName)
	}
}

func GetFilename(filePath string) string {
	_, name := filepath.Split(filePath)
	return name
}
