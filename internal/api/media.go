package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/emzbtw/reel/internal/models"
)

// mediaDetail is the sliver of MovieDetails/TvDetails reel needs: the
// mediaInfo block carrying library status, plus title/release date — Title/
// ReleaseDate for a movie, Name/FirstAirDate for a tv show (mirroring
// MovieResult/TvResult's same asymmetry). The rest of those schemas is
// large and unused, so it is deliberately not modeled.
type mediaDetail struct {
	MediaInfo    *models.MediaInfo `json:"mediaInfo"`
	Title        string            `json:"title"`
	Name         string            `json:"name"`
	ReleaseDate  string            `json:"releaseDate"`
	FirstAirDate string            `json:"firstAirDate"`
}

// titleOrName returns Title if set, else Name — whichever the endpoint
// actually populated depends on movie vs tv, never both.
func (d mediaDetail) titleOrName() string {
	if d.Title != "" {
		return d.Title
	}
	return d.Name
}

// releaseOrFirstAirDate is titleOrName's counterpart for the date fields.
func (d mediaDetail) releaseOrFirstAirDate() string {
	if d.ReleaseDate != "" {
		return d.ReleaseDate
	}
	return d.FirstAirDate
}

// year truncates a YYYY-MM-DD date down to its year. Mirrors internal/tui's
// own identical helper for browse/search results; kept separate rather
// than shared since this package doesn't depend on internal/tui.
func year(date string) string {
	if len(date) < 4 {
		return ""
	}
	return date[:4]
}

// mediaDetailPath builds the /movie/{id} or /tv/{id} path MediaStatus and
// MediaTitle both fetch from, sharing this one path/type switch.
func mediaDetailPath(t models.MediaType, tmdbID int) (string, error) {
	switch t {
	case models.MediaMovie:
		return fmt.Sprintf("/movie/%d", tmdbID), nil
	case models.MediaTV:
		return fmt.Sprintf("/tv/%d", tmdbID), nil
	default:
		return "", fmt.Errorf("seerr: unsupported media type %q", t)
	}
}

func (c *Client) fetchMediaDetail(ctx context.Context, t models.MediaType, tmdbID int) (*mediaDetail, error) {
	path, err := mediaDetailPath(t, tmdbID)
	if err != nil {
		return nil, err
	}
	var out mediaDetail
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MediaStatus returns Seerr's library status for a TMDB ID. It exists for
// items that have no request record and so never appear in ListRequests
// (media the user already owned, or requests an admin has pruned).
//
// Returns nil when Seerr has no library record for the item.
func (c *Client) MediaStatus(ctx context.Context, t models.MediaType, tmdbID int) (*models.MediaInfo, error) {
	d, err := c.fetchMediaDetail(ctx, t, tmdbID)
	if err != nil {
		return nil, err
	}
	return d.MediaInfo, nil
}

// MediaSummary is a TMDB ID's display title and release year — Title
// (movie)/Name (tv show) and the year prefix of ReleaseDate/FirstAirDate.
// Year is "" when the date is missing or too short to have one.
type MediaSummary struct {
	Title string
	Year  string
}

// MediaSummary resolves a TMDB ID's title and release year. Used to fill in
// a request's display title/year, since its own GET /request response
// doesn't include either.
func (c *Client) MediaSummary(ctx context.Context, t models.MediaType, tmdbID int) (MediaSummary, error) {
	d, err := c.fetchMediaDetail(ctx, t, tmdbID)
	if err != nil {
		return MediaSummary{}, err
	}
	return MediaSummary{Title: d.titleOrName(), Year: year(d.releaseOrFirstAirDate())}, nil
}
