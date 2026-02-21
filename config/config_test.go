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
				LogPath:    "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP:   true,
				ExternalIP: "",
				Ports:      []int{2424, 2324},
			},
			wantErr: false,
		},
		{
			name: "valid detect_ip false with external_ip",
			c: Config{
				LogPath:    "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP:   false,
				ExternalIP: "203.0.113.10",
				Ports:      []int{2424},
			},
			wantErr: false,
		},
		{
			name: "invalid empty log_path",
			c: Config{
				LogPath:    "",
				DetectIP:   true,
				Ports:      []int{2424},
			},
			wantErr: true,
		},
		{
			name: "invalid detect_ip false without external_ip",
			c: Config{
				LogPath:    "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP:   false,
				ExternalIP: "",
				Ports:      []int{2424},
			},
			wantErr: true,
		},
		{
			name: "invalid empty ports",
			c: Config{
				LogPath:    "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP:   true,
				ExternalIP: "",
				Ports:      nil,
			},
			wantErr: true,
		},
		{
			name: "invalid empty ports slice",
			c: Config{
				LogPath:  "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP: true,
				Ports:    []int{},
			},
			wantErr: true,
		},
		{
			name: "invalid duplicate port",
			c: Config{
				LogPath:  "/var/log/dzsa-sync/dzsa-sync.log",
				DetectIP: true,
				Ports:    []int{2424, 2324, 2424},
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
ports:
  - 2424
  - 2324
`)
	validPath := filepath.Join(dir, "valid.yaml")
	if err := os.WriteFile(validPath, validYAML, 0600); err != nil {
		t.Fatal(err)
	}

	invalidYAML := []byte(`detect_ip: true
ports: not a list
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
				if got.DetectIP != true || len(got.Ports) != 2 {
					t.Errorf("NewFromFile() config = %+v", got)
				}
			}
		})
	}
}

func TestNewFromFile_Validation(t *testing.T) {
	dir := t.TempDir()

	t.Run("empty ports", func(t *testing.T) {
		path := filepath.Join(dir, "bad.yaml")
		if err := os.WriteFile(path, []byte("log_path: /var/log/dzsa-sync/dzsa-sync.log\ndetect_ip: true\nports: []\n"), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := NewFromFile(path)
		if err == nil {
			t.Error("NewFromFile() expected validation error for empty ports")
		}
	})

	t.Run("empty log_path", func(t *testing.T) {
		path := filepath.Join(dir, "no_log.yaml")
		if err := os.WriteFile(path, []byte("detect_ip: true\nports: [2424]\n"), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := NewFromFile(path)
		if err == nil {
			t.Error("NewFromFile() expected validation error for empty log_path")
		}
	})
}
