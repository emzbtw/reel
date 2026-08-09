package models

import (
	"encoding/json"
	"testing"
)

func TestSearchResult_MarshalUnmarshalRoundTrip(t *testing.T) {
	original := []byte(`{"id":1,"mediaType":"movie","title":"Dune","releaseDate":"2021-10-22"}`)

	var decoded SearchResult
	if err := json.Unmarshal(original, &decoded); err != nil {
		t.Fatalf("Unmarshal() returned error: %v", err)
	}
	if decoded.Movie == nil || decoded.Movie.Title != "Dune" {
		t.Fatalf("decoded = %+v, want a movie titled Dune", decoded)
	}

	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}

	var roundTripped SearchResult
	if err := json.Unmarshal(encoded, &roundTripped); err != nil {
		t.Fatalf("Unmarshal(Marshal(decoded)) returned error: %v\nencoded: %s", err, encoded)
	}
	if roundTripped.Movie == nil || roundTripped.Movie.Title != "Dune" {
		t.Errorf("round-tripped = %+v, want a movie titled Dune (encoded: %s)", roundTripped, encoded)
	}
}

func TestSearchResult_MarshalIsFlat(t *testing.T) {
	tv := SearchResult{MediaType: MediaTV, TV: &TvResult{ID: 2, Name: "Dune: Prophecy"}}

	encoded, err := json.Marshal(tv)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nencoded: %s", err, encoded)
	}
	if got["name"] != "Dune: Prophecy" {
		t.Errorf("encoded = %s, want a flat object with a top-level \"name\"", encoded)
	}
	if _, ok := got["TV"]; ok {
		t.Errorf("encoded = %s, should not nest under the \"TV\" Go field name", encoded)
	}
}
