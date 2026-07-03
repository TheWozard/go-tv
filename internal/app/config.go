package app

import (
	"github.com/TheWozard/go-yaml-config"
)

type Config struct {
	SeriesDir string           `yaml:"series_dir" env-default:"./series"`
	StatePath string           `yaml:"state" env-default:"state.json"`
	Tailscale config.Tailscale `yaml:"tailscale" env-prefix:"TS_"`
	Server    config.Server    `yaml:"server"`
	Player    Player           `yaml:"player"`
	Jellyfin  Jellyfin         `yaml:"jellyfin"`
	Logger    config.Logger    `yaml:"logger" env-prefix:"LOGGER_"`
}
