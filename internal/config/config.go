package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents June's configuration file
type Config struct {
	MCPServers map[string]MCPServer `yaml:"mcpServers"`
}

// MCPServer represents an MCP server configuration
type MCPServer struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env,omitempty"`
}

// LoadConfig loads the config from ~/.june/config.yaml
// Returns empty config (not error) if file doesn't exist
func LoadConfig() (*Config, error) {
	cfg := &Config{
		MCPServers: make(map[string]MCPServer),
	}

	configPath := filepath.Join(DataDir(), "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // No config file is fine
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Ensure map is initialized even if YAML had no servers
	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]MCPServer)
	}

	return cfg, nil
}
