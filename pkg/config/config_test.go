package config

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"testing"
)

func resetFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
}

func TestGetEnvStr(t *testing.T) {
	t.Setenv("TEST_STR_ENV", "hello")
	t.Setenv("TEST_EMPTY_ENV", "")

	tests := []struct {
		name     string
		key      string
		fallback string
		want     string
	}{
		{"Exists", "TEST_STR_ENV", "default", "hello"},
		{"Not Exists", "NON_EXISTENT", "default", "default"},
		{"Exists but empty", "TEST_EMPTY_ENV", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getEnvStr(tt.key, tt.fallback); got != tt.want {
				t.Errorf("getEnvStr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	t.Setenv("TEST_INT_ENV", "8081")
	t.Setenv("TEST_INT_INVALID", "not-an-int")
	t.Setenv("TEST_INT_EMPTY", "")

	tests := []struct {
		name     string
		key      string
		fallback int
		want     int
	}{
		{"Exists and valid", "TEST_INT_ENV", 8080, 8081},
		{"Not Exists", "NON_EXISTENT", 8080, 8080},
		{"Exists but empty", "TEST_INT_EMPTY", 8080, 8080},
		{"Exists but invalid", "TEST_INT_INVALID", 8080, 8080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getEnvInt(tt.key, tt.fallback); got != tt.want {
				t.Errorf("getEnvInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsExists(t *testing.T) {
	tempDir := t.TempDir()

	if !isExists(tempDir) {
		t.Errorf("isExists(%s) expected true, got false", tempDir)
	}

	if isExists("/path/that/definitely/does/not/exist/123456") {
		t.Errorf("isExists expected false, got true")
	}
}

func TestPrintVersion(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintVersion("", "", "")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var result map[string]any
	err := json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if result["version"] != "dev" {
		t.Errorf("Expected version 'dev', got %v", result["version"])
	}
	if result["commit"] != "none" {
		t.Errorf("Expected commit 'none', got %v", result["commit"])
	}
	if result["build_time"] != "unknown" {
		t.Errorf("Expected buildTime 'unknown', got %v", result["build_time"])
	}
	if result["go_version"] == "" {
		t.Errorf("Expected goVersion to be populated")
	}
	if result["platform"] == "" {
		t.Errorf("Expected platform to be populated")
	}

	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	PrintVersion("v1.0.0", "2023-01-01", "abc1234")
	w2.Close()
	os.Stdout = oldStdout

	var buf2 bytes.Buffer
	_, _ = io.Copy(&buf2, r2)
	_ = json.Unmarshal(buf2.Bytes(), &result)

	if result["version"] != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got %v", result["version"])
	}
}

func TestLoad_Normal(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tempDir := t.TempDir()

	tests := []struct {
		name     string
		args     []string
		env      map[string]string
		validate func(*testing.T, *Config, error)
	}{
		{
			name: "Show Version Flag",
			args: []string{"cmd", "-version"},
			validate: func(t *testing.T, cfg *Config, err error) {
				if !cfg.ShowVersion {
					t.Errorf("Expected ShowVersion to be true")
				}
			},
		},
		{
			name: "Command Line Args Priority",
			args: []string{"cmd", "-port", "9090", "-base-path", "/api", "-dir", tempDir},
			env: map[string]string{
				"PORT":       "8081",
				"BASE_PATH":  "/env",
				"STATIC_DIR": "/env/dir",
			},
			validate: func(t *testing.T, cfg *Config, err error) {
				if cfg.Port != 9090 {
					t.Errorf("Expected Port 9090, got %d", cfg.Port)
				}
				if cfg.BasePath != "/api" {
					t.Errorf("Expected BasePath '/api', got '%s'", cfg.BasePath)
				}
				if cfg.StaticDir != tempDir {
					t.Errorf("Expected StaticDir '%s', got '%s'", tempDir, cfg.StaticDir)
				}
			},
		},
		{
			name: "Environment Variables Fallback",
			args: []string{"cmd"},
			env: map[string]string{
				"PORT":       "7070",
				"BASE_PATH":  "/envpath",
				"STATIC_DIR": tempDir,
			},
			validate: func(t *testing.T, cfg *Config, err error) {
				if cfg.Port != 7070 {
					t.Errorf("Expected Port 7070, got %d", cfg.Port)
				}
				if cfg.BasePath != "/envpath" {
					t.Errorf("Expected BasePath '/envpath', got '%s'", cfg.BasePath)
				}
				if cfg.StaticDir != tempDir {
					t.Errorf("Expected StaticDir '%s', got '%s'", tempDir, cfg.StaticDir)
				}
			},
		},
		{
			name: "Default Values",
			args: []string{"cmd", "-dir", tempDir},
			env:  map[string]string{},
			validate: func(t *testing.T, cfg *Config, err error) {
				if cfg.Port != defaultPort {
					t.Errorf("Expected default Port %d, got %d", defaultPort, cfg.Port)
				}
				if cfg.BasePath != defaultBasePath {
					t.Errorf("Expected default BasePath '%s', got '%s'", defaultBasePath, cfg.BasePath)
				}
			},
		},
		{
			name: "Flag Dir Not Exists, Fallback to Env Dir",
			args: []string{"cmd", "-dir", "/invalid/fake/path/123"},
			env: map[string]string{
				"STATIC_DIR": tempDir,
			},
			validate: func(t *testing.T, cfg *Config, err error) {
				if cfg.StaticDir != tempDir {
					t.Errorf("Expected fallback to env dir '%s', got '%s'", tempDir, cfg.StaticDir)
				}
			},
		},
		{
			name: "Flag Dir empty",
			args: []string{"cmd", "-dir", ""},
			env:  map[string]string{},
			validate: func(t *testing.T, cfg *Config, err error) {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			},
		},
		{
			name: "Flag Dir Not Exists, Fallback to Env Dir Not Exists",
			args: []string{"cmd", "-dir", "/invalid/fake/path/123"},
			env:  map[string]string{},
			validate: func(t *testing.T, cfg *Config, err error) {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			},
		},
		{
			name: "Enable json log format",
			args: []string{"cmd", "-dir", tempDir},
			env: map[string]string{
				"JSON_LOG": "true",
			},
			validate: func(t *testing.T, cfg *Config, err error) {
				if !cfg.EnableJSONLog {
					t.Errorf("Expected json log format is false, got true")
				}
			},
		},
		{
			name: "Flag cache age",
			args: []string{"cmd", "-dir", tempDir, "-cache-age", "3600"},
			env: map[string]string{
				"CACHE_MAX_AGE": "7200",
			},
			validate: func(t *testing.T, cfg *Config, err error) {
				if cfg.CacheMaxAge != 3600 {
					t.Errorf("Expected cache age 3600, got %d", cfg.CacheMaxAge)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			os.Args = tt.args

			os.Clearenv()
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()
			tt.validate(t, cfg, err)
		})
	}
}

func TestIsReleaseVersion(t *testing.T) {
	versions := []string{
		"v1.0.0",
		"v1.0.0-rc1",
		"v1.0.0-beta",
		"v1.0.0-alpha",
		"v1.0.0-alpha.1",
		"v1.0.0-alpha.2",
		"prod",
	}
	for _, v := range versions {
		if IsReleaseVersion(v) {
			t.Logf("%s is release version", v)
		} else {
			t.Logf("%s is not release version", v)
		}
	}
}
