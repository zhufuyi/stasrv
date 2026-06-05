package config

import (
	"bytes"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/assets", "/assets"},
		{"assets", "/assets"},
		{"/assets/", "/assets"},
		{"assets/", "/assets"},
		{"/", "/"},
		{"", "/"},
	}

	for _, tt := range tests {
		result := normalizePath(tt.input)
		if result != tt.expected {
			t.Errorf("normalizePath(%s) = %s; want %s", tt.input, result, tt.expected)
		}
	}
}

func TestLocationsValue(t *testing.T) {
	lv := &locationsValue{}

	// Test Set success
	err := lv.Set("/api:/var/www/api")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if len(*lv) != 1 || (*lv)[0].Path != "/api" || (*lv)[0].Root != "/var/www/api" {
		t.Errorf("Unexpected value after Set: %v", *lv)
	}

	// Test Set invalid format
	err = lv.Set("invalid-format")
	if err == nil {
		t.Error("Expected error for invalid format, got nil")
	}

	// Test Set empty parts
	err = lv.Set(":/root")
	if err == nil {
		t.Error("Expected error for empty path, got nil")
	}

	// Test String
	str := lv.String()
	if !strings.Contains(str, "/api") {
		t.Errorf("String() output unexpected: %s", str)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "Empty config",
			config:  &Config{},
			wantErr: true,
		},
		{
			name: "Valid with EmbedFS",
			config: &Config{
				EmbedFSBasePath: "/dist",
			},
			wantErr: false,
		},
		{
			name: "Valid with Locations",
			config: &Config{
				Locations: []location{{Path: "/s", Root: "/r"}},
			},
			wantErr: false,
		},
		{
			name: "Missing path in location",
			config: &Config{
				Locations: []location{{Path: "", Root: "/r"}},
			},
			wantErr: true,
		},
		{
			name: "Duplicate path",
			config: &Config{
				Locations: []location{
					{Path: "/s", Root: "/r1"},
					{Path: "/s", Root: "/r2"},
				},
			},
			wantErr: true,
		},
		{
			name: "Duplicate root",
			config: &Config{
				Locations: []location{
					{Path: "/s1", Root: "/r"},
					{Path: "/s2", Root: "/r"},
				},
			},
			wantErr: true,
		},
		{
			name: "Duplicate path with EmbedFS",
			config: &Config{
				EmbedFSBasePath: "/static",
				Locations: []location{
					{Path: "/static", Root: "/var/www"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsReleaseVersion(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		{"prod", true},
		{"v1.0.0", true},
		{"v12.34.56", true},
		{"1.0.0", false},
		{"dev", false},
		{"v1.0", false},
		{"", false},
	}

	for _, tt := range tests {
		if result := IsReleaseVersion(tt.version); result != tt.expected {
			t.Errorf("IsReleaseVersion(%s) = %v; want %v", tt.version, result, tt.expected)
		}
	}
}

func TestPrintVersion(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintVersion("v1.0.0", "2023-01-01", "abcdef")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "v1.0.0") || !strings.Contains(output, "abcdef") {
		t.Errorf("PrintVersion output missing info: %s", output)
	}

	// Test with empty values
	PrintVersion("", "", "")
}

func TestLoad(t *testing.T) {
	// Since Load uses global flag.CommandLine, we need to reset it for testing
	// and manipulate os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tests := []struct {
		name     string
		args     []string
		wantPort int
		wantVer  bool
		wantErr  bool
	}{
		{
			name:    "Default values (fails validation)",
			args:    []string{"cmd"},
			wantErr: true,
		},
		{
			name:    "Version flag",
			args:    []string{"cmd", "-version"},
			wantVer: true,
			wantErr: false,
		},
		{
			name:     "Valid location",
			args:     []string{"cmd", "--location=/static:/tmp", "--port=9090"},
			wantPort: 9090,
			wantErr:  false,
		},
		{
			name:    "Invalid location format",
			args:    []string{"cmd", "--location=invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			flag.CommandLine = flag.NewFlagSet(tt.args[0], flag.ContinueOnError)
			os.Args = tt.args

			cfg, err := Load()

			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if tt.wantVer && !cfg.ShowVersion {
					t.Error("Expected ShowVersion to be true")
				}
				if tt.wantPort != 0 && cfg.Port != tt.wantPort {
					t.Errorf("Expected port %d, got %d", tt.wantPort, cfg.Port)
				}
			}
		})
	}
}
