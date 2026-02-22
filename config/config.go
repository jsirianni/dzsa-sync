// Package config provides configuration for dzsa-sync.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// APIConfig configures the HTTP API server (metrics and /api/v1/servers).
type APIConfig struct {
	// Host is the listen address for the API server. Empty means all interfaces.
	Host string `yaml:"host"`
	// Port is the listen port (1-65535). Default 8888 when api is omitted.
	Port int `yaml:"port"`
}

// Server is a single DayZ server to register with the DZSA launcher.
type Server struct {
	// Name is a label for the server (e.g. for metrics and API).
	Name string `yaml:"name"`
	// Port is the server query port (1-65535).
	Port int `yaml:"port"`
}

// Config is the root configuration.
type Config struct {
	// DetectIP when true, use https://ifconfig.net/json to detect external IP.
	DetectIP bool `yaml:"detect_ip"`
	// ExternalIP is required when DetectIP is false.
	ExternalIP string `yaml:"external_ip"`
	// Servers is the list of servers to register with the DZSA launcher (replaces Ports).
	Servers []Server `yaml:"servers"`
	// LogPath is the path to the log file (JSON, rotated via lumberjack). Empty uses the default.
	LogPath string `yaml:"log_path"`
	// API configures the HTTP server for /metrics and /api/v1/servers. When nil or zero, defaults to host "" and port 8888.
	API *APIConfig `yaml:"api"`
}

// NewFromFile reads configuration from a YAML file.
func NewFromFile(path string) (*Config, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- path is user-configured
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	c := &Config{}
	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return c, c.Validate()
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.LogPath == "" {
		return fmt.Errorf("log_path is required")
	}
	if !c.DetectIP && c.ExternalIP == "" {
		return fmt.Errorf("external_ip is required when detect_ip is false")
	}
	if len(c.Servers) == 0 {
		return fmt.Errorf("servers must not be empty")
	}
	seenPort := make(map[int]bool)
	for i, s := range c.Servers {
		if s.Name == "" {
			return fmt.Errorf("servers[%d]: name is required", i)
		}
		if s.Port == 0 {
			return fmt.Errorf("servers[%d]: port is required", i)
		}
		if s.Port < 1 || s.Port > 65535 {
			return fmt.Errorf("servers[%d]: port must be 1-65535, got %d", i, s.Port)
		}
		if seenPort[s.Port] {
			return fmt.Errorf("duplicate port: %d", s.Port)
		}
		seenPort[s.Port] = true
	}
	if c.API != nil && c.API.Port != 0 {
		if c.API.Port < 1 || c.API.Port > 65535 {
			return fmt.Errorf("api.port must be 1-65535, got %d", c.API.Port)
		}
	}
	return nil
}
