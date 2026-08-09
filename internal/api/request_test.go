package api

import (
	"context"
	"encoding/json"
	"errors"
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

func TestCreateRequest_TVAllSeasons(t *testing.T) {
	var gotBody map[string]any
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 8, "status": 1}`))
	})

	_, err := c.CreateRequest(context.Background(), CreateRequestInput{
		MediaType:  models.MediaTV,
		MediaID:    456,
		AllSeasons: true,
	})
	if err != nil {
		t.Fatalf("CreateRequest() returned error: %v", err)
	}
	if gotBody["seasons"] != "all" {
		t.Errorf("request body seasons = %+v, want %q", gotBody["seasons"], "all")
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

func TestGetRequest(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/request/5" {
			t.Errorf("method/path = %s %s, want GET /api/v1/request/5", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"id": 5, "status": 2, "media": {"id": 10, "tmdbId": 1234, "status": 4}}`))
	})

	got, err := c.GetRequest(context.Background(), 5)
	if err != nil {
		t.Fatalf("GetRequest() returned error: %v", err)
	}
	if got.ID != 5 || got.Media.TmdbID != 1234 {
		t.Errorf("got = %+v, want id=5 tmdbId=1234", got)
	}
}

func TestGetRequest_NotFound(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"not found"}`))
	})

	_, err := c.GetRequest(context.Background(), 999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteRequest(t *testing.T) {
	var gotMethod, gotPath string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.DeleteRequest(context.Background(), 5); err != nil {
		t.Fatalf("DeleteRequest() returned error: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/v1/request/5" {
		t.Errorf("method/path = %s %s, want DELETE /api/v1/request/5", gotMethod, gotPath)
	}
}

func TestDeleteRequest_Forbidden(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"forbidden"}`))
	})

	err := c.DeleteRequest(context.Background(), 5)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}
