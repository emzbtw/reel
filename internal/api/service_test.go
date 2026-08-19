package api

import (
	"context"
	"net/http"
	"testing"
)

func TestListSonarrServers(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/service/sonarr" {
			t.Errorf("path = %s, want /api/v1/service/sonarr", r.URL.Path)
		}
		w.Write([]byte(`[
			{"id": 0, "name": "Sonarr", "isDefault": true},
			{"id": 1, "name": "Sonarr Anime", "isDefault": false}
		]`))
	})

	got, err := c.ListSonarrServers(context.Background())
	if err != nil {
		t.Fatalf("ListSonarrServers() returned error: %v", err)
	}
	want := []SonarrServer{
		{ID: 0, Name: "Sonarr", IsDefault: true},
		{ID: 1, Name: "Sonarr Anime", IsDefault: false},
	}
	if len(got) != len(want) {
		t.Fatalf("ListSonarrServers() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("server[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
