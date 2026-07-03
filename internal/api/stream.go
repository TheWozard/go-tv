package api

import (
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"

	"go-tv/internal/app"
)

// StreamHandler proxies HLS requests to Jellyfin, keeping the API key server-side.
type StreamHandler struct {
	jellyfin app.Jellyfin
	client   *http.Client
}

func (h *StreamHandler) Mount(r chi.Router) {
	r.Get("/stream/{itemID}/*", h.proxyHandler)
}

func (h *StreamHandler) proxyHandler(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemID")
	subPath := chi.URLParam(r, "*")

	upstream := h.jellyfin.URL + "/Videos/" + itemID + "/" + subPath

	q := r.URL.Query()
	q.Set("api_key", h.jellyfin.APIKey)
	upstream += "?" + q.Encode()

	resp, err := h.client.Get(upstream)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	ct := resp.Header.Get("Content-Type")
	if ct != "" {
		w.Header().Set("Content-Type", ct)
	}

	// For .m3u8 playlists, strip api_key from any URLs Jellyfin embeds in the response.
	if isPlaylist(subPath, ct) {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadGateway)
			return
		}
		cleaned := stripAPIKey(string(body), h.jellyfin.APIKey)
		w.WriteHeader(resp.StatusCode)
		_, _ = io.WriteString(w, cleaned)
		return
	}

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// isPlaylist returns true if the response is an HLS playlist.
func isPlaylist(subPath, contentType string) bool {
	return strings.Contains(contentType, "mpegurl") ||
		path.Ext(subPath) == ".m3u8"
}

// stripAPIKey removes api_key query parameters from URLs in playlist content.
func stripAPIKey(body, apiKey string) string {
	// Handle both orderings: &api_key=... and api_key=...&
	body = strings.ReplaceAll(body, "&api_key="+apiKey, "")
	body = strings.ReplaceAll(body, "api_key="+apiKey+"&", "")
	body = strings.ReplaceAll(body, "api_key="+apiKey, "")
	return body
}
