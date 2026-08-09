package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/emzbtw/reel/internal/models"
)

// CreateRequestInput is the body of POST /request. Seasons is TV-only and
// ignored for movie requests.
type CreateRequestInput struct {
	MediaType models.MediaType
	MediaID   int
	Seasons   []int
}

type createRequestBody struct {
	MediaType string `json:"mediaType"`
	MediaID   int    `json:"mediaId"`
	Seasons   []int  `json:"seasons,omitempty"`
}

// CreateRequest submits a new media request.
func (c *Client) CreateRequest(ctx context.Context, in CreateRequestInput) (*models.MediaRequest, error) {
	body := createRequestBody{
		MediaType: string(in.MediaType),
		MediaID:   in.MediaID,
		Seasons:   in.Seasons,
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
