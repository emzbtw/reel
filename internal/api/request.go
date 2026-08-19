package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/emzbtw/reel/internal/models"
)

// CreateRequestInput is the body of POST /request. Seasons/AllSeasons are
// TV-only and ignored for movie requests. AllSeasons takes precedence over
// Seasons when both are set.
type CreateRequestInput struct {
	MediaType  models.MediaType
	MediaID    int
	AllSeasons bool
	Seasons    []int
	// ServerID picks which configured Sonarr/Radarr instance handles this
	// request (Seerr Settings → Services), overriding Seerr's own default.
	// A pointer because Seerr's server IDs are 0-indexed array positions —
	// 0 is a real, valid ID, so a plain int couldn't distinguish "use
	// server 0" from "no override". nil omits the field entirely.
	ServerID *int
}

type createRequestBody struct {
	MediaType string `json:"mediaType"`
	MediaID   int    `json:"mediaId"`
	// Seasons is either "all" or a []int, matching the spec's
	// oneOf(number[], "all").
	Seasons  any  `json:"seasons,omitempty"`
	ServerID *int `json:"serverId,omitempty"`
}

// CreateRequest submits a new media request.
func (c *Client) CreateRequest(ctx context.Context, in CreateRequestInput) (*models.MediaRequest, error) {
	body := createRequestBody{
		MediaType: string(in.MediaType),
		MediaID:   in.MediaID,
		ServerID:  in.ServerID,
	}
	switch {
	case in.AllSeasons:
		body.Seasons = "all"
	case len(in.Seasons) > 0:
		body.Seasons = in.Seasons
	}

	var out models.MediaRequest
	if err := c.do(ctx, http.MethodPost, "/request", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListRequestsOptions are the optional query parameters of GET /request.
// Zero values are omitted from the request.
type ListRequestsOptions struct {
	Take   int
	Skip   int
	Filter string // e.g. "pending", "approved" — see the spec for the full enum
}

// RequestList is the response body of GET /request.
type RequestList struct {
	PageInfo models.PageInfo       `json:"pageInfo"`
	Results  []models.MediaRequest `json:"results"`
}

// ListRequests returns request history/status, optionally filtered/paged.
func (c *Client) ListRequests(ctx context.Context, opts ListRequestsOptions) (*RequestList, error) {
	q := url.Values{}
	if opts.Take > 0 {
		q.Set("take", strconv.Itoa(opts.Take))
	}
	if opts.Skip > 0 {
		q.Set("skip", strconv.Itoa(opts.Skip))
	}
	if opts.Filter != "" {
		q.Set("filter", opts.Filter)
	}

	var out RequestList
	if err := c.do(ctx, http.MethodGet, "/request", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRequest returns a single request by its local request ID (not TMDB ID).
func (c *Client) GetRequest(ctx context.Context, id int) (*models.MediaRequest, error) {
	var out models.MediaRequest
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/request/%d", id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteRequest removes a request by its local request ID. Per the spec,
// removing anything other than a pending request requires the
// MANAGE_REQUESTS permission; without it, Seerr returns 403.
func (c *Client) DeleteRequest(ctx context.Context, id int) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/request/%d", id), nil, nil, nil)
}
