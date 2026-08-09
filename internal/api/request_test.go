package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/emzbtw/reel/internal/models"
)

func TestCreateRequest_Movie(t *testing.T) {
	var gotBody map[string]any
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &gotBody)

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 42, "status": 1}`))
	})

	got, err := c.CreateRequest(context.Background(), CreateRequestInput{
		MediaType: models.MediaMovie,
		MediaID:   123,
	})
	if err != nil {
		t.Fatalf("CreateRequest() returned error: %v", err)
	}
	if got.ID != 42 {
		t.Errorf("ID = %d, want 42", got.ID)
	}
	if gotBody["mediaType"] != "movie" || gotBody["mediaId"] != float64(123) {
		t.Errorf("request body = %+v", gotBody)
	}
	if _, ok := gotBody["seasons"]; ok {
		t.Errorf("request body has seasons for a movie request: %+v", gotBody)
	}
}

func TestCreateRequest_TVWithSeasons(t *testing.T) {
	var gotBody map[string]any
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 7, "status": 1}`))
	})

	_, err := c.CreateRequest(context.Background(), CreateRequestInput{
		MediaType: models.MediaTV,
		MediaID:   456,
		Seasons:   []int{1, 2},
	})
	if err != nil {
		t.Fatalf("CreateRequest() returned error: %v", err)
	}
	seasons, ok := gotBody["seasons"].([]any)
	if !ok || len(seasons) != 2 {
		t.Errorf("request body seasons = %+v, want [1, 2]", gotBody["seasons"])
	}
}

func TestListRequests(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("filter"); got != "pending" {
			t.Errorf("filter param = %q, want %q", got, "pending")
		}
		w.Write([]byte(`{
			"pageInfo": {"page": 1, "pages": 1, "results": 1},
			"results": [{"id": 1, "status": 1}]
		}`))
	})

	got, err := c.ListRequests(context.Background(), ListRequestsOptions{Filter: "pending"})
	if err != nil {
		t.Fatalf("ListRequests() returned error: %v", err)
	}
	if len(got.Results) != 1 || got.Results[0].ID != 1 {
		t.Errorf("Results = %+v", got.Results)
	}
}
