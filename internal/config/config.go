package config

import (
	"errors"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Port         string        `yaml:"port"`
	SchedulePath string        `yaml:"schedule"`
	StatePath    string        `yaml:"state"`
	Tailscale    Tailscale     `yaml:"tailscale"`
	Integrations []Integration `yaml:"integrations"`
}

type Tailscale struct {
	Hostname string `yaml:"hostname"`
	Dir      string `yaml:"dir"`
	Port     string `yaml:"port"`
}

func (t Tailscale) Enabled() bool { return t.Hostname != "" }

type Integration struct {
	Name      string `yaml:"name"`
	URL       string `yaml:"url"`
	Token     string `yaml:"token"`
	EntityID  string `yaml:"entity_id"`
	MediaType string `yaml:"media_type"` // passed to HA play_media; defaults to "url"
}

// Load reads a YAML config file and returns a Config with defaults applied.
// If the file does not exist it returns the default config without error.
func Load(path string) (*Config, error) {
	cfg := &Config{
		Port:         "8080",
		SchedulePath: "schedule.json",
		StatePath:    "state.json",
		Tailscale: Tailscale{
			Dir:  "/var/lib/tailscale",
			Port: "443",
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

	return cfg, nil
}

// ApplyEnvOverrides lets environment variables take precedence over YAML values.
func (c *Config) ApplyEnvOverrides() {
	if v := os.Getenv("PORT"); v != "" {
		c.Port = v
	}
	if v := os.Getenv("TS_HOSTNAME"); v != "" {
		c.Tailscale.Hostname = v
	}
}
