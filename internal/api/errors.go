package api

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors that callers can match with errors.Is, regardless of the
// underlying *APIError.
var (
	ErrUnauthorized = errors.New("seerr: unauthorized (check API key)")
	ErrForbidden    = errors.New("seerr: forbidden")
	ErrNotFound     = errors.New("seerr: not found")
	ErrRateLimited  = errors.New("seerr: rate limited")
)

// APIError represents a non-2xx response from the Seerr API.
type APIError struct {
	StatusCode int
	Status     string
	Body       []byte

	sentinel error
}

func newAPIError(statusCode int, status string, body []byte) *APIError {
	e := &APIError{StatusCode: statusCode, Status: status, Body: body}
	switch statusCode {
	case http.StatusUnauthorized:
		e.sentinel = ErrUnauthorized
	case http.StatusForbidden:
		e.sentinel = ErrForbidden
	case http.StatusNotFound:
		e.sentinel = ErrNotFound
	case http.StatusTooManyRequests:
		e.sentinel = ErrRateLimited
	}
	return e
}

func (e *APIError) Error() string {
	return fmt.Sprintf("seerr: %s: %s", e.Status, string(e.Body))
}

func (e *APIError) Unwrap() error {
	return e.sentinel
}
