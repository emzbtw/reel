// Package models contains Go types mirroring schemas from Seerr's OpenAPI
// spec (github.com/sct/overseerr, overseerr-api.yml).
package models

// MediaType mirrors the mediaType discriminator used throughout the spec.
type MediaType string

const (
	MediaMovie  MediaType = "movie"
	MediaTV     MediaType = "tv"
	MediaPerson MediaType = "person"
)

// MediaStatus mirrors MediaInfo.status.
type MediaStatus int

const (
	MediaStatusUnknown            MediaStatus = 1
	MediaStatusPending            MediaStatus = 2
	MediaStatusProcessing         MediaStatus = 3
	MediaStatusPartiallyAvailable MediaStatus = 4
	MediaStatusAvailable          MediaStatus = 5
	MediaStatusBlocklisted        MediaStatus = 6
	MediaStatusDeleted            MediaStatus = 7
)

// MediaInfo mirrors the `MediaInfo` schema.
type MediaInfo struct {
	ID       int            `json:"id"`
	TmdbID   int            `json:"tmdbId"`
	TvdbID   int            `json:"tvdbId"`
	Status   MediaStatus    `json:"status"`
	Requests []MediaRequest `json:"requests"`
}

// PageInfo mirrors the `PageInfo` schema, used by GET /request.
type PageInfo struct {
	Page    int `json:"page"`
	Pages   int `json:"pages"`
	Results int `json:"results"`
}
