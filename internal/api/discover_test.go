package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/emzbtw/reel/internal/models"
)

func TestDiscoverTrending_MixedResults(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/discover/trending" {
			t.Errorf("path = %q, want %q", got, "/api/v1/discover/trending")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"page": 1,
			"totalPages": 1,
			"totalResults": 2,
			"results": [
				{"id": 1, "mediaType": "movie", "title": "Dune", "popularity": 100},
				{"id": 2, "mediaType": "tv", "name": "Dune: Prophecy", "popularity": 90}
			]
		}`))
	})

	got, err := c.DiscoverTrending(context.Background(), 0)
	if err != nil {
		t.Fatalf("DiscoverTrending() returned error: %v", err)
	}
	if len(got.Results) != 2 {
		t.Fatalf("len(Results) = %d, want 2", len(got.Results))
	}

	r0 := got.Results[0]
	if r0.MediaType != models.MediaMovie || r0.Movie == nil || r0.Movie.Title != "Dune" {
		t.Errorf("Results[0] = %+v, want movie %q", r0, "Dune")
	}

	r1 := got.Results[1]
	if r1.MediaType != models.MediaTV || r1.TV == nil || r1.TV.Name != "Dune: Prophecy" {
		t.Errorf("Results[1] = %+v, want tv %q", r1, "Dune: Prophecy")
	}
}
