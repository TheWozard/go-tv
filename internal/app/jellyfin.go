package app

import (
	"crypto/tls"
	"net/http"
)

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
