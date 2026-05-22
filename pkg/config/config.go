// Package config is the configuration package
package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
)

const (
	defaultPort        = 8080
	defaultBasePath    = "/"
	defaultCacheMaxAge = 31536000 // 1 year
)

// Config Defines the common configuration of Web services
type Config struct {
	Port        int
	BasePath    string
	StaticDir   string
	CacheMaxAge *int

	ShowVersion   bool
	EnableJSONLog bool
}

// Load Parses and loads service configuration
// Priority: Command line parameters (Flag) > Environment variables (Env) > Default
func Load() (*Config, error) {
	cfg := &Config{}
	var showVersion bool

	flag.BoolVar(&showVersion, "version", false, "Print version")
	flag.IntVar(&cfg.Port, "port", 0, "Server listen port")
	flag.StringVar(&cfg.BasePath, "base-path", "", "Base path for URL")
	flag.StringVar(&cfg.StaticDir, "dir", "", "Web static file directory")
	flag.BoolVar(&cfg.EnableJSONLog, "json-log", false, "Enable JSON log format")
	flag.Var(intPtrValue{target: &cfg.CacheMaxAge}, "cache-age", "Static file cache max age seconds, default 31536000 seconds, 0 means disable cache")

	flag.Parse() //nolint

	if showVersion {
		cfg.ShowVersion = showVersion
		return cfg, nil
	}

	if cfg.Port == 0 {
		cfg.Port = getEnvInt("PORT", defaultPort)
	}
	if cfg.BasePath == "" {
		cfg.BasePath = getEnvStr("BASE_PATH", defaultBasePath)
		fmt.Println(cfg.BasePath)
	}
	if cfg.StaticDir == "" {
		cfg.StaticDir = getEnvStr("STATIC_DIR", "")
	} else {
		if !isExists(cfg.StaticDir) {
			cfg.StaticDir = getEnvStr("STATIC_DIR", cfg.StaticDir)
		}
	}

	if cfg.CacheMaxAge == nil {
		cfg.CacheMaxAge = new(int)
		*cfg.CacheMaxAge = getEnvInt("CACHE_AGE", defaultCacheMaxAge)
	}

	if !cfg.EnableJSONLog {
		cfg.EnableJSONLog = getEnvBool("JSON_LOG", false)
	}

	if cfg.StaticDir == "" {
		return nil, errors.New("web static file directory '--dir' or env 'STATIC_DIR' is required")
	}
	if !isExists(cfg.StaticDir) {
		return nil, errors.New("web static file directory '" + cfg.StaticDir + "' is not exists")
	}

	return cfg, nil
}

func getEnvStr(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			return p
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return fallback
}

func isExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

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
		BuildTime string `json:"buildTime"`
		GoVersion string `json:"goVersion"`
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

type intPtrValue struct {
	target **int
}

func (i intPtrValue) String() string {
	if i.target == nil || *i.target == nil {
		return ""
	}
	return strconv.Itoa(**i.target)
}

func (i intPtrValue) Set(value string) error {
	val, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	*i.target = &val
	return nil
}
