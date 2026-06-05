// Package config is the stasrv configuration
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const defaultPort = 8080

// Config Defines the common configuration of Web services
type Config struct {
	Locations []location `json:"locations"`

	Port            int    `json:"port"`
	EmbedFSBasePath string `json:"embed_fs_base_path"`
	CacheMaxAge     int    `json:"cache_max_age"`
	EnableListFiles bool   `json:"enable_list_files"`
	ShowVersion     bool   `json:"-"`
}

// Load Parses and loads service configuration
func Load() (*Config, error) {
	var (
		showVersion   bool
		locationFlags locationsValue

		cfg = &Config{}
	)

	// Example: --location="/assets:/var/www/assets" --location="/js:/var/www/js"
	flag.Var(&locationFlags, "location", "Static assets mapping in 'path:root' format (can be multiple)")

	flag.BoolVar(&showVersion, "version", false, "Print version")
	flag.IntVar(&cfg.Port, "port", defaultPort, "Server listen port")
	flag.BoolVar(&cfg.EnableListFiles, "enable-list-files", false, "Allow access to file list")
	flag.IntVar(&cfg.CacheMaxAge, "cache-age", 0, "Cache JS, CSS, and image static asset, unit is second, 0 means no cache")
	flag.StringVar(&cfg.EmbedFSBasePath, "fs-base-path", "", "Embed FS base path. Copy files to ./embedded_dir and run 'make build'")

	flag.Parse() //nolint

	if showVersion {
		cfg.ShowVersion = showVersion
		return cfg, nil
	}

	cfg.Locations = locationFlags

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// check the effectiveness of flags
func (c *Config) validate() error {
	if len(c.Locations) == 0 {
		if c.EmbedFSBasePath == "" {
			return fmt.Errorf("no static file mapping found, usage: ./stasrv --location=/assets:/var/www/assets")
		}
		return nil
	}

	seenPaths := make(map[string]bool)
	seenRoots := make(map[string]bool)

	if c.EmbedFSBasePath != "" {
		c.EmbedFSBasePath = normalizePath(c.EmbedFSBasePath)
		seenPaths[c.EmbedFSBasePath] = true
	}

	for _, sc := range c.Locations {
		if sc.Path == "" || sc.Root == "" {
			return fmt.Errorf("both path and root must be provided in location")
		}

		if seenPaths[sc.Path] {
			return fmt.Errorf("duplicate location path found: %s", sc.Path)
		}
		seenPaths[sc.Path] = true

		if seenRoots[sc.Root] {
			return fmt.Errorf("duplicate local root directory found: %s", sc.Root)
		}
		seenRoots[sc.Root] = true
	}

	return nil
}

// ----------------------------------------------------------

// location nginx location simplified configuration
type location struct {
	Path string `json:"path"` // URL access path, e.g."/static/"
	Root string `json:"root"` // Local file directory, e.g."/var/www/static"
}

type locationsValue []location

func (s *locationsValue) String() string {
	return fmt.Sprintf("%v", *s)
}

// Set resolve the format of 'path:root'
func (s *locationsValue) Set(value string) error {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid format '%s', expected 'path:root'", value)
	}

	*s = append(*s, location{
		Path: normalizePath(parts[0]),
		Root: parts[1],
	})
	return nil
}

func normalizePath(basePath string) string {
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	basePath = strings.TrimSuffix(basePath, "/")
	if basePath != "" {
		basePath = filepath.ToSlash(filepath.Clean(basePath))
	}
	if basePath == "" {
		basePath = "/"
	}
	return basePath
}

// --------------------------------------------------------

// PrintVersion print version info
func PrintVersion(version, buildTime, commit string) {
	if version == "" {
		version = "dev"
	}
	if buildTime == "" {
		buildTime = "unknown"
	}
	if commit == "" {
		commit = "none"
	}

	info := struct {
		Version   string `json:"version"`
		Commit    string `json:"commit"`
		BuildTime string `json:"build_time"`
		GoVersion string `json:"go_version"`
		Platform  string `json:"platform"`
	}{
		Version:   version,
		Commit:    commit,
		BuildTime: buildTime,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	jsonData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		fmt.Println("{}")
		return
	}
	fmt.Println(string(jsonData))
}

// IsReleaseVersion check is release version
func IsReleaseVersion(version string) bool {
	if version == "prod" {
		return true
	}
	pattern := `^v\d+\.\d+\.\d+$`
	match, _ := regexp.MatchString(pattern, version)
	return match
}
