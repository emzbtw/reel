package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/emzbtw/reel/internal/models"
)

// SearchResults is the response body of GET /search.
type SearchResults struct {
	Page         int                   `json:"page"`
	TotalPages   int                   `json:"totalPages"`
	TotalResults int                   `json:"totalResults"`
	Results      []models.SearchResult `json:"results"`
}

// Search performs a combined movie/TV/person search. page defaults to 1
// when <= 0.
func (c *Client) Search(ctx context.Context, query string, page int) (*SearchResults, error) {
	q := url.Values{"query": {query}}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}

	var out SearchResults
	if err := c.do(ctx, http.MethodGet, "/search", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
