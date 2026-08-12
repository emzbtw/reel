package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"

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

// TitleCache is an in-session, concurrency-safe TMDB-ID -> MediaSummary
// cache shared by anything that lists requests (internal/tui's requests
// view, internal/cli's status/delete tables): the same TMDB ID looked up
// twice within the cache's lifetime is resolved once and reused instead of
// refetched. Purely in-memory — nothing here is written to disk.
//
// The mutex is needed because ResolveTitles resolves cache misses with
// concurrent goroutines: without it, two of its own goroutines writing to
// the same map at once — or two overlapping ResolveTitles calls sharing a
// cache — would race.
type TitleCache struct {
	mu sync.Mutex
	m  map[int]MediaSummary
}

// NewTitleCache returns an empty TitleCache ready to use.
func NewTitleCache() *TitleCache {
	return &TitleCache{m: make(map[int]MediaSummary)}
}

func (c *TitleCache) Get(tmdbID int) (MediaSummary, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.m[tmdbID]
	return s, ok
}

func (c *TitleCache) Set(tmdbID int, s MediaSummary) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[tmdbID] = s
}

// TitleQuery identifies one item ResolveTitles should resolve a title for.
type TitleQuery struct {
	MediaType models.MediaType
	TmdbID    int
}

// ResolveTitles resolves a MediaSummary for each query, in the same order:
// a cache hit is used directly (no request), a miss is looked up via
// MediaSummary and cached for next time. Misses are resolved concurrently —
// up to len(queries) at once, since each is an independent HTTP call to a
// different TMDB ID with no shared state beyond the cache, which is already
// safe for concurrent access; callers passing a large batch should bound it
// themselves. A failed lookup (e.g. a since-deleted TMDB entry) leaves that
// query's result as the zero MediaSummary rather than failing the batch.
func ResolveTitles(ctx context.Context, client *Client, cache *TitleCache, queries []TitleQuery) []MediaSummary {
	out := make([]MediaSummary, len(queries))
	var wg sync.WaitGroup
	for i, q := range queries {
		if s, ok := cache.Get(q.TmdbID); ok {
			out[i] = s
			continue
		}
		wg.Add(1)
		go func(i int, q TitleQuery) {
			defer wg.Done()
			s, err := client.MediaSummary(ctx, q.MediaType, q.TmdbID)
			if err != nil || s.Title == "" {
				return
			}
			out[i] = s
			cache.Set(q.TmdbID, s)
		}(i, q)
	}
	wg.Wait()
	return out
}
