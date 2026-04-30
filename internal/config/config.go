package config

import (
	"context"
	"crypto/tls"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that unmarshals from YAML strings like "10s" or "1m30s".
type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	dur, err := time.ParseDuration(value.Value)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

func (d Duration) Duration() time.Duration { return time.Duration(d) }

type Player struct {
	SkipInterval Duration `yaml:"skip_interval"`
	ProgressRate Duration `yaml:"progress_rate"`
}

type Jellyfin struct {
	URL             string `yaml:"url"`
	APIKey          string `yaml:"api_key"`
	InsecureSkipTLS bool   `yaml:"insecure_skip_tls"`
	Proxy           bool   `yaml:"proxy"`
}

// StreamURL returns the HLS URL for the given item.
// When Proxy is enabled, returns a local proxy path so the API key never reaches the frontend.
// Returns an empty string when Jellyfin is not configured.
func (j Jellyfin) StreamURL(itemID string) string {
	if j.URL == "" || itemID == "" {
		return ""
	}
	if j.Proxy {
		return "/api/stream/" + itemID + "/master.m3u8?MediaSourceId=" + itemID + "&VideoCodec=h264&AudioCodec=aac"
	}
	return j.URL + "/Videos/" + itemID + "/master.m3u8?MediaSourceId=" + itemID + "&VideoCodec=h264&AudioCodec=aac&api_key=" + j.APIKey
}

// HTTPClient returns an HTTP client configured for this Jellyfin instance.
// When InsecureSkipTLS is true, TLS certificate verification is disabled.
func (j Jellyfin) HTTPClient() *http.Client {
	if !j.InsecureSkipTLS {
		return http.DefaultClient
	}
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
}


type Config struct {
	SchedulePath string    `yaml:"schedule"`
	StatePath    string    `yaml:"state"`
	Tailscale    Tailscale `yaml:"tailscale"`
	Server       Server    `yaml:"server"`
	Player       Player    `yaml:"player"`
	Jellyfin     Jellyfin  `yaml:"jellyfin"`
}

// Load reads a YAML config file and returns a Config with defaults applied.
// If the file does not exist it returns the default config without error.
func Load(path string) (*Config, error) {
	cfg := &Config{
		SchedulePath: "schedule.json",
		StatePath:    "state.json",
		Tailscale: Tailscale{
			Dir:  "/var/lib/tailscale",
			Port: "443",
		},
		Server: Server{
			Port: "8080",
		},
		Player: Player{
			SkipInterval: Duration(10 * time.Second),
	ProgressRate: Duration(10 * time.Second),
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			cfg.ApplyEnvOverrides()
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
