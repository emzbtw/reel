package api

import (
	"context"
	"net/http"
)

// SonarrServer is one configured Sonarr instance, as returned by
// GET /service/sonarr (Seerr Settings → Services). Trimmed to what reel
// needs to resolve a server by name — Seerr's SonarrSettings also carries
// hostname/apiKey/profile details reel doesn't use.
type SonarrServer struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

// ListSonarrServers returns every Sonarr instance configured in Seerr.
func (c *Client) ListSonarrServers(ctx context.Context) ([]SonarrServer, error) {
	var out []SonarrServer
	if err := c.do(ctx, http.MethodGet, "/service/sonarr", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
