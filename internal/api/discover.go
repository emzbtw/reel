package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/emzbtw/reel/internal/models"
)

// MovieResults is the response body of GET /discover/movies.
type MovieResults struct {
	Page         int                  `json:"page"`
	TotalPages   int                  `json:"totalPages"`
	TotalResults int                  `json:"totalResults"`
	Results      []models.MovieResult `json:"results"`
}

// TvResults is the response body of GET /discover/tv.
type TvResults struct {
	Page         int               `json:"page"`
	TotalPages   int               `json:"totalPages"`
	TotalResults int               `json:"totalResults"`
	Results      []models.TvResult `json:"results"`
}

// DiscoverMovies returns popular/trending movies. page defaults to 1 when
// <= 0.
func (c *Client) DiscoverMovies(ctx context.Context, page int) (*MovieResults, error) {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}

	var out MovieResults
	if err := c.do(ctx, http.MethodGet, "/discover/movies", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DiscoverTV returns popular/trending TV shows. page defaults to 1 when
// <= 0.
func (c *Client) DiscoverTV(ctx context.Context, page int) (*TvResults, error) {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}

	var out TvResults
	if err := c.do(ctx, http.MethodGet, "/discover/tv", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TrendingResults is the response body of GET /discover/trending. Results
// mix movies and TV shows (never people), so it reuses models.SearchResult
// as the anyOf(MovieResult, TvResult) union.
type TrendingResults struct {
	Page         int                   `json:"page"`
	TotalPages   int                   `json:"totalPages"`
	TotalResults int                   `json:"totalResults"`
	Results      []models.SearchResult `json:"results"`
}

// DiscoverTrending returns trending movies and TV shows combined, sorted by
// popularity. page defaults to 1 when <= 0.
func (c *Client) DiscoverTrending(ctx context.Context, page int) (*TrendingResults, error) {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}

	var out TrendingResults
	if err := c.do(ctx, http.MethodGet, "/discover/trending", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
