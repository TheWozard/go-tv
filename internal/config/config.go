package config

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	SchedulePath string        `yaml:"schedule"`
	StatePath    string        `yaml:"state"`
	Tailscale    Tailscale     `yaml:"tailscale"`
	Server       Server        `yaml:"server"`
}

// Load reads a YAML config file and returns a Config with defaults applied.
// If the file does not exist it returns the default config without error.
func Load(path string) (*Config, error) {
	cfg := &Config{
		SchedulePath: "schedule.json",
		StatePath:    "state.json",
		Tailscale: Tailscale{
			Dir: "/var/lib/tailscale",
		},
		Server: Server{
			Port: "8080",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	cfg.ApplyEnvOverrides()
	return cfg, nil
}

// ApplyEnvOverrides lets environment variables take precedence over YAML values.
func (c *Config) ApplyEnvOverrides() {
	if v := os.Getenv("PORT"); v != "" {
		c.Server.Port = v
		c.Tailscale.Port = v
	}
	if v := os.Getenv("TS_HOSTNAME"); v != "" {
		c.Tailscale.Hostname = v
	}
}

type ServerListener interface {
	Listen(context.Context, http.Handler) error
}

func (c *Config) GetServerListener() ServerListener {
	if c.Tailscale.Enabled() {
		return c.Tailscale
	}
	return c.Server
}
