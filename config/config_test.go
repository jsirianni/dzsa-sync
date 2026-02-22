package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		c       Config
		wantErr bool
	}{
		{
			name: "valid detect_ip true",
			c: Config{
				LogPath:  "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP: true,
				ExternalIP: "",
				Servers: []Server{
					{Name: "main", Port: 2424},
					{Name: "modded", Port: 2324},
				},
			},
			wantErr: false,
		},
		{
			name: "valid detect_ip false with external_ip",
			c: Config{
				LogPath:    "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP:   false,
				ExternalIP: "203.0.113.10",
				Servers:    []Server{{Name: "main", Port: 2424}},
			},
			wantErr: false,
		},
		{
			name: "invalid empty log_path",
			c: Config{
				LogPath:  "",
				DetectIP: true,
				Servers:  []Server{{Name: "main", Port: 2424}},
			},
			wantErr: true,
		},
		{
			name: "invalid detect_ip false without external_ip",
			c: Config{
				LogPath:    "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP:   false,
				ExternalIP: "",
				Servers:    []Server{{Name: "main", Port: 2424}},
			},
			wantErr: true,
		},
		{
			name: "invalid empty servers",
			c: Config{
				LogPath:    "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP:   true,
				ExternalIP: "",
				Servers:    nil,
			},
			wantErr: true,
		},
		{
			name: "invalid empty servers slice",
			c: Config{
				LogPath:  "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP: true,
				Servers:  []Server{},
			},
			wantErr: true,
		},
		{
			name: "invalid duplicate port",
			c: Config{
				LogPath:  "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP: true,
				Servers: []Server{
					{Name: "a", Port: 2424},
					{Name: "b", Port: 2324},
					{Name: "c", Port: 2424},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid server missing name",
			c: Config{
				LogPath:  "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP: true,
				Servers:  []Server{{Name: "", Port: 2424}},
			},
			wantErr: true,
		},
		{
			name: "invalid server missing port",
			c: Config{
				LogPath:  "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP: true,
				Servers:  []Server{{Name: "main", Port: 0}},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewFromFile(t *testing.T) {
	dir := t.TempDir()

	validYAML := []byte(`log_path: /var/log/dzsa-sync/dzsa-sync.log
detect_ip: true
servers:
  - name: main
    port: 2424
  - name: modded
    port: 2324
`)
	validPath := filepath.Join(dir, "valid.yaml")
	if err := os.WriteFile(validPath, validYAML, 0600); err != nil {
		t.Fatal(err)
	}

	invalidYAML := []byte(`detect_ip: true
servers: not a list
`)
	invalidPath := filepath.Join(dir, "invalid.yaml")
	if err := os.WriteFile(invalidPath, invalidYAML, 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid file",
			path:    validPath,
			wantErr: false,
		},
		{
			name:    "file not found",
			path:    filepath.Join(dir, "nonexistent.yaml"),
			wantErr: true,
		},
		{
			name:    "invalid yaml",
			path:    invalidPath,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewFromFile(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("NewFromFile() returned nil config without error")
			}
			if !tt.wantErr && got != nil {
				if got.DetectIP != true || len(got.Servers) != 2 {
					t.Errorf("NewFromFile() config = %+v", got)
				}
			}
		})
	}
}

func TestNewFromFile_Validation(t *testing.T) {
	dir := t.TempDir()

	t.Run("empty servers", func(t *testing.T) {
		path := filepath.Join(dir, "bad.yaml")
		if err := os.WriteFile(path, []byte("log_path: /var/log/dzsa-sync/dzsa-sync.log\ndetect_ip: true\nservers: []\n"), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := NewFromFile(path)
		if err == nil {
			t.Error("NewFromFile() expected validation error for empty servers")
		}
	})

	t.Run("empty log_path", func(t *testing.T) {
		path := filepath.Join(dir, "no_log.yaml")
		if err := os.WriteFile(path, []byte("detect_ip: true\nservers:\n  - name: main\n    port: 2424\n"), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := NewFromFile(path)
		if err == nil {
			t.Error("NewFromFile() expected validation error for empty log_path")
		}
	})
}
