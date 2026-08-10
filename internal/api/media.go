package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/emzbtw/reel/internal/models"
)

// mediaDetail is the sliver of MovieDetails/TvDetails reel needs: the
// mediaInfo block carrying library status. The rest of those schemas is
// large and unused, so it is deliberately not modeled.
type mediaDetail struct {
	MediaInfo *models.MediaInfo `json:"mediaInfo"`
}

// MediaStatus returns Seerr's library status for a TMDB ID. It exists for
// items that have no request record and so never appear in ListRequests
// (media the user already owned, or requests an admin has pruned).
//
// Returns nil when Seerr has no library record for the item.
func (c *Client) MediaStatus(ctx context.Context, t models.MediaType, tmdbID int) (*models.MediaInfo, error) {
	var path string
	switch t {
	case models.MediaMovie:
		path = fmt.Sprintf("/movie/%d", tmdbID)
	case models.MediaTV:
		path = fmt.Sprintf("/tv/%d", tmdbID)
	default:
		return nil, fmt.Errorf("seerr: MediaStatus: unsupported media type %q", t)
	}

	var out mediaDetail
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out.MediaInfo, nil
}
