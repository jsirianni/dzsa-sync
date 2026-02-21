// Package config provides configuration for dzsa-sync.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration.
type Config struct {
	// DetectIP when true, use https://ifconfig.net/json to detect external IP.
	DetectIP bool `yaml:"detect_ip"`
	// ExternalIP is required when DetectIP is false.
	ExternalIP string `yaml:"external_ip"`
	// Ports is the list of server query ports to register with DZSA launcher.
	Ports []int `yaml:"ports"`
	// LogPath is the path to the log file (JSON, rotated via lumberjack). Empty uses the default.
	LogPath string `yaml:"log_path"`
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
	if len(c.Ports) == 0 {
		return fmt.Errorf("ports must not be empty")
	}
	seen := make(map[int]bool)
	for _, p := range c.Ports {
		if seen[p] {
			return fmt.Errorf("duplicate port: %d", p)
		}
		seen[p] = true
	}
	return nil
}
