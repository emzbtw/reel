package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/emzbtw/reel/internal/models"
)

func TestSearch_MixedResults(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("query"); got != "dune" {
			t.Errorf("query param = %q, want %q", got, "dune")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"page": 1,
			"totalPages": 1,
			"totalResults": 3,
			"results": [
				{"id": 1, "mediaType": "movie", "title": "Dune"},
				{"id": 2, "mediaType": "tv", "name": "Dune: Prophecy"},
				{"id": 3, "mediaType": "person", "profilePath": "/p.jpg"}
			]
		}`))
	})

	got, err := c.Search(context.Background(), "dune", 0)
	if err != nil {
		t.Fatalf("Search() returned error: %v", err)
	}
	if len(got.Results) != 3 {
		t.Fatalf("len(Results) = %d, want 3", len(got.Results))
	}

	r0 := got.Results[0]
	if r0.MediaType != models.MediaMovie || r0.Movie == nil || r0.Movie.Title != "Dune" {
		t.Errorf("Results[0] = %+v, want movie %q", r0, "Dune")
	}

	r1 := got.Results[1]
	if r1.MediaType != models.MediaTV || r1.TV == nil || r1.TV.Name != "Dune: Prophecy" {
		t.Errorf("Results[1] = %+v, want tv %q", r1, "Dune: Prophecy")
	}

	r2 := got.Results[2]
	if r2.MediaType != models.MediaPerson || r2.Person == nil || r2.Person.ProfilePath != "/p.jpg" {
		t.Errorf("Results[2] = %+v, want person %q", r2, "/p.jpg")
	}
}
