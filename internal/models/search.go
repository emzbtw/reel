package models

import (
	"encoding/json"
	"fmt"
)

// MovieResult mirrors the `MovieResult` schema.
type MovieResult struct {
	ID            int        `json:"id"`
	MediaType     MediaType  `json:"mediaType"`
	Title         string     `json:"title"`
	OriginalTitle string     `json:"originalTitle"`
	Overview      string     `json:"overview"`
	PosterPath    string     `json:"posterPath"`
	ReleaseDate   string     `json:"releaseDate"`
	VoteAverage   float64    `json:"voteAverage"`
	VoteCount     int        `json:"voteCount"`
	Popularity    float64    `json:"popularity"`
	MediaInfo     *MediaInfo `json:"mediaInfo"`
}

// TvResult mirrors the `TvResult` schema.
type TvResult struct {
	ID           int        `json:"id"`
	MediaType    MediaType  `json:"mediaType"`
	Name         string     `json:"name"`
	OriginalName string     `json:"originalName"`
	Overview     string     `json:"overview"`
	PosterPath   string     `json:"posterPath"`
	FirstAirDate string     `json:"firstAirDate"`
	VoteAverage  float64    `json:"voteAverage"`
	VoteCount    int        `json:"voteCount"`
	Popularity   float64    `json:"popularity"`
	MediaInfo    *MediaInfo `json:"mediaInfo"`
}

// PersonResult mirrors the `PersonResult` schema.
type PersonResult struct {
	ID          int       `json:"id"`
	MediaType   MediaType `json:"mediaType"`
	ProfilePath string    `json:"profilePath"`
}

// SearchResult is one element of GET /search's results array, which the
// spec types as anyOf(MovieResult, TvResult, PersonResult). Exactly one of
// Movie, TV, or Person is set, matching MediaType.
type SearchResult struct {
	MediaType MediaType
	Movie     *MovieResult
	TV        *TvResult
	Person    *PersonResult
}

// MarshalJSON mirrors UnmarshalJSON: it re-emits whichever of Movie, TV, or
// Person is set, flat and using that type's own tags, so encoding a
// SearchResult reproduces the same shape it was decoded from rather than
// nesting it under Go field names.
func (r SearchResult) MarshalJSON() ([]byte, error) {
	switch r.MediaType {
	case MediaMovie:
		return json.Marshal(r.Movie)
	case MediaTV:
		return json.Marshal(r.TV)
	case MediaPerson:
		return json.Marshal(r.Person)
	default:
		return []byte("null"), nil
	}
}

func (r *SearchResult) UnmarshalJSON(data []byte) error {
	var probe struct {
		MediaType MediaType `json:"mediaType"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}

	r.MediaType = probe.MediaType
	switch probe.MediaType {
	case MediaMovie:
		r.Movie = &MovieResult{}
		return json.Unmarshal(data, r.Movie)
	case MediaTV:
		r.TV = &TvResult{}
		return json.Unmarshal(data, r.TV)
	case MediaPerson:
		r.Person = &PersonResult{}
		return json.Unmarshal(data, r.Person)
	default:
		return fmt.Errorf("models: unknown search result mediaType %q", probe.MediaType)
	}
}
