package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/emzbtw/reel/internal/models"
)

func TestMediaStatus_Movie(t *testing.T) {
	var gotMethod, gotPath string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Write([]byte(`{"mediaInfo": {"id": 10, "tmdbId": 329865, "status": 5}}`))
	})

	got, err := c.MediaStatus(context.Background(), models.MediaMovie, 329865)
	if err != nil {
		t.Fatalf("MediaStatus() returned error: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/v1/movie/329865" {
		t.Errorf("method/path = %s %s, want GET /api/v1/movie/329865", gotMethod, gotPath)
	}
	if got == nil || got.Status != models.MediaStatusAvailable {
		t.Errorf("got = %+v, want status=Available", got)
	}
}

func TestMediaStatus_TV(t *testing.T) {
	var gotPath string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"mediaInfo": {"id": 11, "tmdbId": 1399, "status": 4}}`))
	})

	got, err := c.MediaStatus(context.Background(), models.MediaTV, 1399)
	if err != nil {
		t.Fatalf("MediaStatus() returned error: %v", err)
	}
	if gotPath != "/api/v1/tv/1399" {
		t.Errorf("path = %s, want /api/v1/tv/1399", gotPath)
	}
	if got == nil || got.Status != models.MediaStatusPartiallyAvailable {
		t.Errorf("got = %+v, want status=PartiallyAvailable", got)
	}
}

func TestMediaStatus_NullMediaInfo(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"mediaInfo": null}`))
	})

	got, err := c.MediaStatus(context.Background(), models.MediaMovie, 1)
	if err != nil {
		t.Fatalf("MediaStatus() returned error: %v", err)
	}
	if got != nil {
		t.Errorf("got = %+v, want nil", got)
	}
}

func TestMediaStatus_NoMediaInfoKey(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id": 1, "title": "Dune"}`))
	})

	got, err := c.MediaStatus(context.Background(), models.MediaMovie, 1)
	if err != nil {
		t.Fatalf("MediaStatus() returned error: %v", err)
	}
	if got != nil {
		t.Errorf("got = %+v, want nil", got)
	}
}

func TestMediaStatus_UnsupportedMediaType(t *testing.T) {
	called := false
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte(`{}`))
	})

	_, err := c.MediaStatus(context.Background(), models.MediaPerson, 1)
	if err == nil {
		t.Fatal("MediaStatus() returned nil error, want error for person media type")
	}
	if called {
		t.Error("MediaStatus() issued an HTTP request for an unsupported media type")
	}
}

func TestMediaStatus_NotFound(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"not found"}`))
	})

	_, err := c.MediaStatus(context.Background(), models.MediaMovie, 999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestMediaSummary_Movie(t *testing.T) {
	var gotPath string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"title": "Dune", "releaseDate": "2021-09-15", "mediaInfo": {"id": 10, "tmdbId": 438631, "status": 5}}`))
	})

	got, err := c.MediaSummary(context.Background(), models.MediaMovie, 438631)
	if err != nil {
		t.Fatalf("MediaSummary() returned error: %v", err)
	}
	if gotPath != "/api/v1/movie/438631" {
		t.Errorf("path = %s, want /api/v1/movie/438631", gotPath)
	}
	if got.Title != "Dune" || got.Year != "2021" {
		t.Errorf("got = %+v, want Title=Dune Year=2021", got)
	}
}

func TestMediaSummary_TV(t *testing.T) {
	var gotPath string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"name": "Chernobyl", "firstAirDate": "2019-05-06", "mediaInfo": {"id": 11, "tmdbId": 87108, "status": 5}}`))
	})

	got, err := c.MediaSummary(context.Background(), models.MediaTV, 87108)
	if err != nil {
		t.Fatalf("MediaSummary() returned error: %v", err)
	}
	if gotPath != "/api/v1/tv/87108" {
		t.Errorf("path = %s, want /api/v1/tv/87108", gotPath)
	}
	if got.Title != "Chernobyl" || got.Year != "2019" {
		t.Errorf("got = %+v, want Title=Chernobyl Year=2019", got)
	}
}

func TestMediaSummary_UnsupportedMediaType(t *testing.T) {
	called := false
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte(`{}`))
	})

	_, err := c.MediaSummary(context.Background(), models.MediaPerson, 1)
	if err == nil {
		t.Fatal("MediaSummary() returned nil error, want error for person media type")
	}
	if called {
		t.Error("MediaSummary() issued an HTTP request for an unsupported media type")
	}
}
