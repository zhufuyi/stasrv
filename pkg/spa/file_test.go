package spa

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestOptions(t *testing.T) {
	opts := defaultFileOptions()

	// Test default
	if opts.filter != "" || opts.name != "" || opts.noAbsolutePath != false {
		t.Errorf("defaultFileOptions failed, got: %+v", opts)
	}

	// Test WithPrefix
	opts.apply(WithPrefix("pre_"))
	if opts.filter != prefix || opts.name != "pre_" {
		t.Errorf("WithPrefix failed, got: %+v", opts)
	}

	// Test WithSuffix
	opts.apply(WithSuffix(".go"))
	if opts.filter != suffix || opts.name != ".go" {
		t.Errorf("WithSuffix failed, got: %+v", opts)
	}

	// Test WithContain
	opts.apply(WithContain("middle"))
	if opts.filter != contain || opts.name != "middle" {
		t.Errorf("WithContain failed, got: %+v", opts)
	}

	// Test WithNoAbsolutePath
	opts.apply(WithNoAbsolutePath())
	if !opts.noAbsolutePath {
		t.Errorf("WithNoAbsolutePath failed, got: %+v", opts)
	}
}

func TestMatchers(t *testing.T) {
	t.Run("matchPrefix", func(t *testing.T) {
		matcher := matchPrefix("pre_")
		if !matcher("/path/to/pre_test.go") {
			t.Errorf("matchPrefix expected true")
		}
		if matcher("/path/to/test.go") {
			t.Errorf("matchPrefix expected false")
		}
		if matcher("/path/to/p") { // 测试长度不足的情况
			t.Errorf("matchPrefix with short filename expected false")
		}

		emptyMatcher := matchPrefix("")
		if emptyMatcher("any.go") {
			t.Errorf("matchPrefix with empty string expected false")
		}
	})

	t.Run("matchSuffix", func(t *testing.T) {
		matcher := matchSuffix(".log")
		if !matcher("/path/to/test.log") {
			t.Errorf("matchSuffix expected true")
		}
		if matcher("/path/to/test.txt") {
			t.Errorf("matchSuffix expected false")
		}
		if matcher(".l") { // 测试长度不足的情况
			t.Errorf("matchSuffix with short filename expected false")
		}

		emptyMatcher := matchSuffix("")
		if emptyMatcher("any.go") {
			t.Errorf("matchSuffix with empty string expected false")
		}
	})

	t.Run("matchContain", func(t *testing.T) {
		matcher := matchContain("abc")
		if !matcher("/path/to/test_abc_test.go") {
			t.Errorf("matchContain expected true")
		}
		if matcher("/path/to/test_xyz.go") {
			t.Errorf("matchContain expected false")
		}

		emptyMatcher := matchContain("")
		if emptyMatcher("any.go") {
			t.Errorf("matchContain with empty string expected false")
		}
	})
}

func TestHelpers(t *testing.T) {
	t.Run("GetFilename", func(t *testing.T) {
		if got := GetFilename("/path/to/file.txt"); got != "file.txt" {
			t.Errorf("GetFilename failed, got: %s", got)
		}
		if got := GetFilename("file.txt"); got != "file.txt" {
			t.Errorf("GetFilename failed, got: %s", got)
		}
	})

	t.Run("GetPathDelimiter", func(t *testing.T) {
		delim := GetPathDelimiter()
		if runtime.GOOS == "windows" {
			if delim != "\\" {
				t.Errorf("Expected \\ on Windows, got %s", delim)
			}
		} else {
			if delim != "/" {
				t.Errorf("Expected / on non-Windows, got %s", delim)
			}
		}
	})
}

func setupTestDir(t *testing.T) string {
	dir := t.TempDir()

	files := []string{
		"file1.txt",
		"pre_file2.go",
		"file3_cont_file.txt",
		"file4.log",
		filepath.Join("subdir", "sub1.txt"),
		filepath.Join("subdir", "pre_sub2.go"),
		filepath.Join("subdir", "sub3.log"),
	}

	for _, f := range files {
		fullPath := filepath.Join(dir, f)
		err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(fullPath, []byte("test content"), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func normalizePaths(paths []string) []string {
	var res []string
	for _, p := range paths {
		res = append(res, filepath.ToSlash(p))
	}
	sort.Strings(res)
	return res
}

func extractFilenames(paths []string) []string {
	var res []string
	for _, p := range paths {
		res = append(res, GetFilename(p))
	}
	sort.Strings(res)
	return res
}

func TestListFiles(t *testing.T) {
	baseDir := setupTestDir(t)

	t.Run("No Filter (All files)", func(t *testing.T) {
		files, err := ListFiles(baseDir)
		if err != nil {
			t.Fatalf("ListFiles error: %v", err)
		}
		if len(files) != 7 {
			t.Errorf("Expected 7 files, got %d", len(files))
		}
	})

	t.Run("WithPrefix", func(t *testing.T) {
		files, err := ListFiles(baseDir, WithPrefix("pre_"))
		if err != nil {
			t.Fatalf("ListFiles error: %v", err)
		}
		names := extractFilenames(files)
		expected := []string{"pre_file2.go", "pre_sub2.go"}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("Prefix match failed, expected %v, got %v", expected, names)
		}
	})

	t.Run("WithSuffix", func(t *testing.T) {
		files, err := ListFiles(baseDir, WithSuffix(".log"))
		if err != nil {
			t.Fatalf("ListFiles error: %v", err)
		}
		names := extractFilenames(files)
		expected := []string{"file4.log", "sub3.log"}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("Suffix match failed, expected %v, got %v", expected, names)
		}
	})

	t.Run("WithContain", func(t *testing.T) {
		files, err := ListFiles(baseDir, WithContain("_cont_"))
		if err != nil {
			t.Fatalf("ListFiles error: %v", err)
		}
		names := extractFilenames(files)
		expected := []string{"file3_cont_file.txt"}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("Contain match failed, expected %v, got %v", expected, names)
		}
	})

	t.Run("WithNoAbsolutePath", func(t *testing.T) {
		cwd, _ := os.Getwd()
		defer os.Chdir(cwd)
		os.Chdir(baseDir)

		files, err := ListFiles(".", WithNoAbsolutePath())
		if err != nil {
			t.Fatalf("ListFiles error: %v", err)
		}

		for _, f := range files {
			if strings.HasPrefix(f, baseDir) {
				t.Errorf("Expected relative path, got absolute: %s", f)
			}
		}
	})
}

func TestWalkErrors(t *testing.T) {
	t.Run("Invalid Directory", func(t *testing.T) {
		invalidDir := "path_does_not_exist_12345"

		_, err := ListFiles(invalidDir)
		if err == nil {
			t.Errorf("Expected error for non-existent directory")
		}

		_, err = ListFiles(invalidDir, WithPrefix("test"))
		if err == nil {
			t.Errorf("Expected error for non-existent directory with filter")
		}
	})

	t.Run("Recursive Walk Error", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping recursive permission error test on Windows")
		}

		baseDir := t.TempDir()
		subDir := filepath.Join(baseDir, "noperm")
		err := os.Mkdir(subDir, 0o000)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(subDir, 0o755)

		_, err = ListFiles(baseDir)
		if err == nil {
			t.Errorf("Expected error when reading directory without permissions")
		}

		_, err = ListFiles(baseDir, WithPrefix("x"))
		if err == nil {
			t.Errorf("Expected error when reading directory without permissions (with filter)")
		}
	})
}
