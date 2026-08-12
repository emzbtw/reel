package tui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/emzbtw/reel/internal/api"
	"github.com/emzbtw/reel/internal/config"
	"github.com/emzbtw/reel/internal/models"
)

func testClient(t *testing.T, handler http.HandlerFunc) *api.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return api.NewClient(&config.Config{SeerrURL: srv.URL, SeerrAPIKey: "test-key"})
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func testItem() browseItem {
	return browseItem{id: 42, mediaType: models.MediaMovie, title: "Dune", year: "2021"}
}

func testRequestItem() requestItem {
	return requestItem{id: 9, requestStatus: 1, mediaType: models.MediaMovie, tmdbID: 42, mediaStatus: models.MediaStatusPending, createdAt: "2024-01-15T10:30:00.000Z"}
}

// fetchPageCmd/submitRequestCmd

func TestFetchPageCmd_Movies(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/discover/movies" {
			t.Errorf("path = %q, want /api/v1/discover/movies", r.URL.Path)
		}
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Errorf("page param = %q, want %q", got, "2")
		}
		w.Write([]byte(`{
			"page": 2, "totalPages": 50, "totalResults": 1000,
			"results": [{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"}]
		}`))
	})

	msg := fetchPageCmd(context.Background(), client, sourceDiscover, models.MediaMovie, "", 2, 7)()
	loaded, ok := msg.(pageLoadedMsg)
	if !ok {
		t.Fatalf("msg = %#v, want pageLoadedMsg", msg)
	}
	if loaded.seq != 7 || loaded.page != 2 || loaded.totalPages != 50 {
		t.Errorf("loaded = %+v, want seq=7 page=2 totalPages=50", loaded)
	}
	if len(loaded.items) != 1 || loaded.items[0].title != "Dune" || loaded.items[0].mediaType != models.MediaMovie {
		t.Errorf("loaded.items = %+v, want a single movie Dune", loaded.items)
	}
}

func TestFetchPageCmd_TV(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/discover/tv" {
			t.Errorf("path = %q, want /api/v1/discover/tv", r.URL.Path)
		}
		w.Write([]byte(`{
			"page": 1, "totalPages": 10, "totalResults": 200,
			"results": [{"id": 5, "mediaType": "tv", "name": "Chernobyl", "firstAirDate": "2019-05-06"}]
		}`))
	})

	msg := fetchPageCmd(context.Background(), client, sourceDiscover, models.MediaTV, "", 1, 0)()
	loaded, ok := msg.(pageLoadedMsg)
	if !ok {
		t.Fatalf("msg = %#v, want pageLoadedMsg", msg)
	}
	if len(loaded.items) != 1 || loaded.items[0].title != "Chernobyl" || loaded.items[0].mediaType != models.MediaTV {
		t.Errorf("loaded.items = %+v, want a single tv Chernobyl", loaded.items)
	}
}

func TestFetchPageCmd_Error(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"nope"}`))
	})

	msg := fetchPageCmd(context.Background(), client, sourceDiscover, models.MediaMovie, "", 1, 3)()
	errored, ok := msg.(errMsg)
	if !ok {
		t.Fatalf("msg = %#v, want errMsg", msg)
	}
	if errored.seq != 3 {
		t.Errorf("errored.seq = %d, want 3", errored.seq)
	}
	if !errors.Is(errored.err, api.ErrUnauthorized) {
		t.Errorf("errored.err = %v, want it to wrap api.ErrUnauthorized", errored.err)
	}
}

func TestFetchPageCmd_Search(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Errorf("path = %q, want /api/v1/search", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != "dune" {
			t.Errorf("query param = %q, want %q", got, "dune")
		}
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Errorf("page param = %q, want %q", got, "2")
		}
		w.Write([]byte(`{
			"page": 2, "totalPages": 3, "totalResults": 50,
			"results": [
				{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"},
				{"id": 2, "mediaType": "tv", "name": "Dune: Prophecy", "firstAirDate": "2024-11-17"},
				{"id": 3, "mediaType": "person", "profilePath": "/x.jpg"}
			]
		}`))
	})

	msg := fetchPageCmd(context.Background(), client, sourceSearch, models.MediaMovie, "dune", 2, 5)()
	loaded, ok := msg.(pageLoadedMsg)
	if !ok {
		t.Fatalf("msg = %#v, want pageLoadedMsg", msg)
	}
	if loaded.seq != 5 || loaded.page != 2 || loaded.totalPages != 3 {
		t.Errorf("loaded = %+v, want seq=5 page=2 totalPages=3", loaded)
	}
	// Person result must be dropped; only the movie and TV results remain.
	if len(loaded.items) != 2 {
		t.Fatalf("len(loaded.items) = %d, want 2 (person result dropped)", len(loaded.items))
	}
	if loaded.items[0].title != "Dune" || loaded.items[0].mediaType != models.MediaMovie {
		t.Errorf("loaded.items[0] = %+v, want movie Dune", loaded.items[0])
	}
	if loaded.items[1].title != "Dune: Prophecy" || loaded.items[1].mediaType != models.MediaTV {
		t.Errorf("loaded.items[1] = %+v, want tv Dune: Prophecy", loaded.items[1])
	}
}

// searchItems / promoteDominant

// TestSearchItems_PromotesDominantMatch is the "cher" scenario: a short,
// ambiguous query where Seerr's relevance ordering ranks a couple of
// low-signal partial matches ahead of a well-known result. Chernobyl's vote
// count and popularity dominate both of the weak matches ahead of it (well
// past the 10x bar), so it should climb to the top despite starting last.
func TestSearchItems_PromotesDominantMatch(t *testing.T) {
	results := []models.SearchResult{
		{MediaType: models.MediaMovie, Movie: &models.MovieResult{ID: 1, Title: "Cher Ami", VoteCount: 5, Popularity: 2.0}},
		{MediaType: models.MediaMovie, Movie: &models.MovieResult{ID: 2, Title: "Cheryl's Diary", VoteCount: 3, Popularity: 1.0}},
		{MediaType: models.MediaTV, TV: &models.TvResult{ID: 3, Name: "Chernobyl", VoteCount: 5000, Popularity: 120.0}},
	}

	items := searchItems(results)

	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	if items[0].title != "Chernobyl" {
		t.Errorf("items[0].title = %q, want %q (should be promoted to the top)", items[0].title, "Chernobyl")
	}
}

// TestSearchItems_PreciseQueryUnaffected checks the case the promotion logic
// must not disturb: a precise query where Seerr already ranks the intended
// match first, with a second candidate that's close but not dominant (well
// under the 10x bar). Order must be left exactly as Seerr returned it.
func TestSearchItems_PreciseQueryUnaffected(t *testing.T) {
	results := []models.SearchResult{
		{MediaType: models.MediaMovie, Movie: &models.MovieResult{ID: 1, Title: "The Matrix", VoteCount: 25000, Popularity: 80.0}},
		{MediaType: models.MediaMovie, Movie: &models.MovieResult{ID: 2, Title: "The Matrix Reloaded", VoteCount: 15000, Popularity: 60.0}},
	}

	items := searchItems(results)

	if len(items) != 2 || items[0].title != "The Matrix" || items[1].title != "The Matrix Reloaded" {
		t.Errorf("items = %+v, want unchanged [The Matrix, The Matrix Reloaded]", items)
	}
}

// TestSearchItems_NoSignalNoReorder checks that results with no vote/
// popularity signal at all (e.g. unreleased titles) are never reordered —
// dominatesForSearch's zero-popularity guard must not misfire into treating
// "0 >= 10*0" as a win.
func TestSearchItems_NoSignalNoReorder(t *testing.T) {
	results := []models.SearchResult{
		{MediaType: models.MediaMovie, Movie: &models.MovieResult{ID: 1, Title: "Unreleased A"}},
		{MediaType: models.MediaMovie, Movie: &models.MovieResult{ID: 2, Title: "Unreleased B"}},
	}

	items := searchItems(results)

	if len(items) != 2 || items[0].title != "Unreleased A" || items[1].title != "Unreleased B" {
		t.Errorf("items = %+v, want unchanged [Unreleased A, Unreleased B]", items)
	}
}

func TestSubmitRequestCmd(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/request" {
			t.Errorf("path = %q, want /api/v1/request", r.URL.Path)
		}
		w.Write([]byte(`{"id": 9, "status": 1, "media": {"id": 1, "tmdbId": 42}}`))
	})

	msg := submitRequestCmd(context.Background(), client, testItem())()
	result, ok := msg.(requestResultMsg)
	if !ok {
		t.Fatalf("msg = %#v, want requestResultMsg", msg)
	}
	if result.err != nil {
		t.Fatalf("result.err = %v, want nil", result.err)
	}
	if result.req == nil || result.req.ID != 9 {
		t.Errorf("result.req = %+v, want ID 9", result.req)
	}
}

// TestRequestItem_TitleTagsMovieVsTV checks Title's "(Movie)"/"(TV)" suffix
// tracks mediaType, both with a resolved title and with the TMDB-ID
// fallback.
func TestRequestItem_TitleTagsMovieVsTV(t *testing.T) {
	movie := requestItem{tmdbID: 438631, mediaType: models.MediaMovie, title: "Dune"}
	if got := movie.Title(); got != "Dune (Movie)" {
		t.Errorf("Title() = %q, want %q", got, "Dune (Movie)")
	}

	tv := requestItem{tmdbID: 87108, mediaType: models.MediaTV, title: "Chernobyl"}
	if got := tv.Title(); got != "Chernobyl (TV)" {
		t.Errorf("Title() = %q, want %q", got, "Chernobyl (TV)")
	}

	unresolved := requestItem{tmdbID: 87108, mediaType: models.MediaTV}
	if got := unresolved.Title(); got != "TMDB 87108 (TV)" {
		t.Errorf("Title() = %q, want %q (fallback + tag)", got, "TMDB 87108 (TV)")
	}
}

// TestRequestItem_TitleIncludesYear checks the year, once resolved, is
// woven into the same tag as the movie/tv label ("(2021 · Movie)") rather
// than a separate tag, and is simply omitted (not left as a blank) when
// unresolved.
func TestRequestItem_TitleIncludesYear(t *testing.T) {
	withYear := requestItem{tmdbID: 438631, mediaType: models.MediaMovie, title: "Dune", year: "2021"}
	if got := withYear.Title(); got != "Dune (2021 · Movie)" {
		t.Errorf("Title() = %q, want %q", got, "Dune (2021 · Movie)")
	}

	withoutYear := requestItem{tmdbID: 438631, mediaType: models.MediaMovie, title: "Dune"}
	if got := withoutYear.Title(); got != "Dune (Movie)" {
		t.Errorf("Title() = %q, want %q (no year, no dangling separator)", got, "Dune (Movie)")
	}
}

// TestRequestItem_NameHasNoEmbeddedANSI is the regression check: name()
// must stay plain text, with none of Title's embedded ANSI color codes —
// callers that pass it through fmt's %q (e.g. requestResultView's "Deleted
// %q.") depend on that, since %q escapes control characters (including the
// raw ESC byte in an ANSI code) into literal "\x1b[...m" text instead of
// letting the terminal render it as color.
// TestRequestItem_NameHasNoEmbeddedANSI forces a real color profile for the
// duration of the test (and restores it after): lipgloss auto-detects "no
// TTY" under `go test` and silently strips all styling, which would let
// this test pass even with the bug present — Title() only actually embeds
// ANSI when lipgloss believes it's writing to a color-capable terminal,
// exactly the condition under which the reported bug showed up live.
func TestRequestItem_NameHasNoEmbeddedANSI(t *testing.T) {
	orig := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(orig) })

	it := requestItem{tmdbID: 438631, mediaType: models.MediaMovie, title: "Dune", year: "2021"}

	name := it.name()
	if name != "Dune" {
		t.Errorf("name() = %q, want %q", name, "Dune")
	}
	if strings.Contains(name, "\x1b") {
		t.Errorf("name() = %q, contains a raw ESC byte — must stay plain text", name)
	}

	// Title(), by contrast, does embed one (in its muted tag) — confirming
	// this test would actually catch a caller that regresses back to using
	// Title() somewhere a plain string is required.
	if !strings.Contains(it.Title(), "\x1b") {
		t.Errorf("Title() = %q, want it to contain an ANSI escape (sanity check that name/Title actually differ under a real color profile)", it.Title())
	}

	// The bug as it actually manifested: %q-ing an ANSI-containing string
	// escapes the raw ESC byte into literal "\x1b[...m" text.
	if quoted := fmt.Sprintf("%q", it.Title()); !strings.Contains(quoted, `\x1b`) {
		t.Errorf("fmt.Sprintf(%%q, Title()) = %s, want it to demonstrate the escaping (sanity check)", quoted)
	}
	if quoted := fmt.Sprintf("%q", it.name()); strings.Contains(quoted, `\x1b`) {
		t.Errorf("fmt.Sprintf(%%q, name()) = %s, want no escaped ANSI — this is the actual fix", quoted)
	}
}

// TestRequestItemOf_CarriesSeasons checks a TV request's per-season status
// carries straight through from models.MediaRequest into requestItem
// (movies never have any, so there's nothing to check on that side beyond
// the zero value already covering it).
func TestRequestItemOf_CarriesSeasons(t *testing.T) {
	r := models.MediaRequest{
		ID: 9, Status: 5, Type: models.MediaTV,
		Media: models.MediaInfo{TmdbID: 87108, Status: models.MediaStatusAvailable},
		Seasons: []models.RequestSeason{
			{ID: 1, SeasonNumber: 1, Status: models.MediaStatusAvailable},
			{ID: 2, SeasonNumber: 2, Status: models.MediaStatusProcessing},
		},
	}

	got := requestItemOf(r)

	if len(got.seasons) != 2 {
		t.Fatalf("len(seasons) = %d, want 2", len(got.seasons))
	}
	if got.seasons[0].SeasonNumber != 1 || got.seasons[0].Status != models.MediaStatusAvailable {
		t.Errorf("seasons[0] = %+v, want season 1, Available", got.seasons[0])
	}
	if got.seasons[1].SeasonNumber != 2 || got.seasons[1].Status != models.MediaStatusProcessing {
		t.Errorf("seasons[1] = %+v, want season 2, Processing", got.seasons[1])
	}
}

// fetchRequestsCmd/deleteRequestCmd

func TestFetchRequestsCmd_Success(t *testing.T) {
	var movieCalls int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/request":
			if got := r.URL.Query().Get("take"); got != "20" {
				t.Errorf("take param = %q, want %q", got, "20")
			}
			if got := r.URL.Query().Get("skip"); got != "20" {
				t.Errorf("skip param = %q, want %q (page 2)", got, "20")
			}
			w.Write([]byte(`{
				"pageInfo": {"page": 2, "pages": 5, "results": 100},
				"results": [
					{"id": 9, "status": 1, "type": "movie", "media": {"id": 1, "tmdbId": 42, "status": 2}, "createdAt": "2024-01-15T10:30:00.000Z"}
				]
			}`))
		case "/api/v1/movie/42":
			movieCalls++
			w.Write([]byte(`{"title": "Dune", "mediaInfo": {"id": 1, "tmdbId": 42, "status": 2}}`))
		default:
			t.Errorf("path = %q, want /api/v1/request or /api/v1/movie/42", r.URL.Path)
		}
	})

	msg := fetchRequestsCmd(context.Background(), client, newTitleCache(), 2, 7)()
	loaded, ok := msg.(requestsPageLoadedMsg)
	if !ok {
		t.Fatalf("msg = %#v, want requestsPageLoadedMsg", msg)
	}
	if loaded.seq != 7 || loaded.page != 2 || loaded.totalPages != 5 {
		t.Errorf("loaded = %+v, want seq=7 page=2 totalPages=5", loaded)
	}
	if len(loaded.items) != 1 || loaded.items[0].id != 9 || loaded.items[0].tmdbID != 42 {
		t.Errorf("loaded.items = %+v, want a single request id=9 tmdbID=42", loaded.items)
	}
	if loaded.items[0].title != "Dune" {
		t.Errorf("loaded.items[0].title = %q, want %q (resolved via MediaTitle)", loaded.items[0].title, "Dune")
	}
	if movieCalls != 1 {
		t.Errorf("movie detail was fetched %d times, want exactly 1", movieCalls)
	}
}

func TestFetchRequestsCmd_Error(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"nope"}`))
	})

	msg := fetchRequestsCmd(context.Background(), client, newTitleCache(), 1, 3)()
	errored, ok := msg.(errMsg)
	if !ok {
		t.Fatalf("msg = %#v, want errMsg", msg)
	}
	if errored.seq != 3 {
		t.Errorf("errored.seq = %d, want 3", errored.seq)
	}
	if !errors.Is(errored.err, api.ErrUnauthorized) {
		t.Errorf("errored.err = %v, want it to wrap api.ErrUnauthorized", errored.err)
	}
}

// resolveTitles

func TestResolveTitles_CacheHitSkipsFetch(t *testing.T) {
	var calls int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"title": "Dune", "mediaInfo": {"id": 1, "tmdbId": 42, "status": 2}}`))
	})
	cache := newTitleCache()
	cache.set(42, api.MediaSummary{Title: "Dune (cached)", Year: "1984"})

	items := []requestItem{{id: 9, mediaType: models.MediaMovie, tmdbID: 42}}
	resolveTitles(context.Background(), client, cache, items)

	if items[0].title != "Dune (cached)" || items[0].year != "1984" {
		t.Errorf("items[0] title/year = %q/%q, want %q/%q (from cache)", items[0].title, items[0].year, "Dune (cached)", "1984")
	}
	if calls != 0 {
		t.Errorf("MediaSummary was fetched %d times, want 0 (cache hit)", calls)
	}
}

func TestResolveTitles_CacheMissFetchesAndPopulatesCache(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tv/87108" {
			t.Errorf("path = %q, want /api/v1/tv/87108", r.URL.Path)
		}
		w.Write([]byte(`{"name": "Chernobyl", "firstAirDate": "2019-05-06", "mediaInfo": {"id": 1, "tmdbId": 87108, "status": 5}}`))
	})
	cache := newTitleCache()

	items := []requestItem{{id: 9, mediaType: models.MediaTV, tmdbID: 87108}}
	resolveTitles(context.Background(), client, cache, items)

	if items[0].title != "Chernobyl" || items[0].year != "2019" {
		t.Errorf("items[0] title/year = %q/%q, want %q/%q", items[0].title, items[0].year, "Chernobyl", "2019")
	}
	if got, ok := cache.get(87108); !ok || got.Title != "Chernobyl" || got.Year != "2019" {
		t.Errorf("cache.get(87108) = %+v, %v, want Title=Chernobyl Year=2019, true (populated for next time)", got, ok)
	}
}

// TestResolveTitles_FailedLookupDoesNotFailThePage checks a single item's
// failed title lookup (e.g. a since-deleted TMDB entry) doesn't block the
// rest of the page: it just leaves that item's title empty, falling back
// to its TMDB ID via requestItem.Title, while unrelated items still
// resolve normally.
func TestResolveTitles_FailedLookupDoesNotFailThePage(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/movie/1" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"not found"}`))
			return
		}
		w.Write([]byte(`{"title": "Dune", "mediaInfo": {"id": 1, "tmdbId": 42, "status": 2}}`))
	})
	cache := newTitleCache()

	items := []requestItem{
		{id: 8, mediaType: models.MediaMovie, tmdbID: 1},
		{id: 9, mediaType: models.MediaMovie, tmdbID: 42},
	}
	resolveTitles(context.Background(), client, cache, items)

	if items[0].title != "" {
		t.Errorf("items[0].title = %q, want empty (lookup failed)", items[0].title)
	}
	if got := items[0].Title(); !strings.HasPrefix(got, "TMDB 1 ") {
		t.Errorf("items[0].Title() = %q, want it to start with the fallback %q", got, "TMDB 1 ")
	}
	if items[1].title != "Dune" {
		t.Errorf("items[1].title = %q, want %q (unaffected by items[0]'s failure)", items[1].title, "Dune")
	}
}

func TestDeleteRequestCmd_Success(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/request/9" {
			t.Errorf("method/path = %s %s, want DELETE /api/v1/request/9", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	msg := deleteRequestCmd(context.Background(), client, 9)()
	result, ok := msg.(deleteResultMsg)
	if !ok {
		t.Fatalf("msg = %#v, want deleteResultMsg", msg)
	}
	if result.id != 9 || result.err != nil {
		t.Errorf("result = %+v, want id=9 err=nil", result)
	}
}

func TestDeleteRequestCmd_Error(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"nope"}`))
	})

	msg := deleteRequestCmd(context.Background(), client, 9)()
	result, ok := msg.(deleteResultMsg)
	if !ok {
		t.Fatalf("msg = %#v, want deleteResultMsg", msg)
	}
	if result.id != 9 || result.err == nil {
		t.Errorf("result = %+v, want id=9 non-nil err", result)
	}
}

// Update: async message handling

func TestUpdate_PageLoaded_SetsListAndClearsLoading(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.loading = true

	updated, _ := m.Update(pageLoadedMsg{
		seq: 0, page: 4, totalPages: 20,
		items: []browseItem{testItem()},
	})
	got := updated.(model)

	if got.loading {
		t.Error("loading = true, want false after pageLoadedMsg")
	}
	if got.page != 4 || got.totalPages != 20 {
		t.Errorf("page/totalPages = %d/%d, want 4/20", got.page, got.totalPages)
	}
	if len(got.list.Items()) != 1 {
		t.Errorf("len(list.Items()) = %d, want 1", len(got.list.Items()))
	}
}

func TestUpdate_PageLoaded_StaleSeqDiscarded(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.loading = true
	m.loadSeq = 2

	updated, _ := m.Update(pageLoadedMsg{seq: 1, page: 99, totalPages: 99})
	got := updated.(model)

	if !got.loading {
		t.Error("loading = false, want true: a stale pageLoadedMsg should be ignored")
	}
	if got.page == 99 {
		t.Error("page was updated from a stale pageLoadedMsg")
	}
}

func TestUpdate_ErrMsg_StaleSeqDiscarded(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.loading = true
	m.loadSeq = 2

	wantErr := errors.New("boom")
	updated, _ := m.Update(errMsg{seq: 1, err: wantErr})
	got := updated.(model)

	if !got.loading || got.err != nil {
		t.Errorf("loading/err = %v/%v, want true/nil: a stale errMsg should be ignored", got.loading, got.err)
	}
}

func TestUpdate_ErrMsg_CurrentSeqSets(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.loading = true

	wantErr := errors.New("boom")
	updated, _ := m.Update(errMsg{seq: 0, err: wantErr})
	got := updated.(model)

	if got.loading {
		t.Error("loading = true, want false after errMsg")
	}
	if got.err != wantErr {
		t.Errorf("err = %v, want %v", got.err, wantErr)
	}
}

func TestUpdate_RequestsPageLoaded_SetsListAndClearsLoading(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.loading = true

	updated, _ := m.Update(requestsPageLoadedMsg{
		seq: 0, page: 2, totalPages: 5,
		items: []requestItem{testRequestItem()},
	})
	got := updated.(model)

	if got.loading {
		t.Error("loading = true, want false after requestsPageLoadedMsg")
	}
	if got.requestsPage != 2 || got.requestsTotalPages != 5 {
		t.Errorf("requestsPage/requestsTotalPages = %d/%d, want 2/5", got.requestsPage, got.requestsTotalPages)
	}
	if len(got.list.Items()) != 1 {
		t.Errorf("len(list.Items()) = %d, want 1", len(got.list.Items()))
	}
}

func TestUpdate_RequestsPageLoaded_StaleSeqDiscarded(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.loading = true
	m.loadSeq = 2

	updated, _ := m.Update(requestsPageLoadedMsg{seq: 1, page: 99, totalPages: 99})
	got := updated.(model)

	if !got.loading {
		t.Error("loading = false, want true: a stale requestsPageLoadedMsg should be ignored")
	}
	if got.requestsPage == 99 {
		t.Error("requestsPage was updated from a stale requestsPageLoadedMsg")
	}
}

func TestUpdate_DeleteResult_SuccessTransitionsToRequestResult(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequestConfirm
	m.loading = true

	updated, _ := m.Update(deleteResultMsg{id: 9, err: nil})
	got := updated.(model)

	if got.mode != modeRequestResult {
		t.Errorf("mode = %v, want modeRequestResult", got.mode)
	}
	if got.loading {
		t.Error("loading = true, want false after deleteResultMsg")
	}
	if got.deletedRequestID != 9 {
		t.Errorf("deletedRequestID = %d, want 9", got.deletedRequestID)
	}
}

func TestUpdate_DeleteResult_ErrorTransitionsToRequestResult(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequestConfirm
	m.loading = true

	wantErr := errors.New("boom")
	updated, _ := m.Update(deleteResultMsg{id: 9, err: wantErr})
	got := updated.(model)

	if got.mode != modeRequestResult {
		t.Errorf("mode = %v, want modeRequestResult", got.mode)
	}
	if got.err != wantErr {
		t.Errorf("err = %v, want %v", got.err, wantErr)
	}
	if got.deletedRequestID != 0 {
		t.Errorf("deletedRequestID = %d, want 0 (unset on failure)", got.deletedRequestID)
	}
}

// Update: window resize

func TestUpdate_WindowSizeMsg_ResizesListAndReflowsEveryScreen(t *testing.T) {
	longOverview := strings.Repeat("word ", 60)

	for _, tt := range []struct {
		name string
		m    func() model
		// expectExactHeight is false for cases whose content is
		// deliberately taller than the 20-row viewport (they exist to
		// stress-test wrapping/no-bleed, not anchoring): the footer can't
		// stay pinned below content that doesn't fit, the same way a list
		// longer than the terminal can't either. Anchoring under realistic
		// content sizes is covered separately by
		// TestView_FooterAnchoredToBottomInEveryMode.
		expectExactHeight bool
	}{
		{"browsing", func() model {
			m := newModel(context.Background(), nil)
			updated, _ := m.Update(pageLoadedMsg{items: []browseItem{testItem()}})
			return updated.(model)
		}, true},
		{"search", func() model {
			// searchInput is a single-line scrolling field, so even a very
			// long value never grows past one row.
			m := newModel(context.Background(), nil)
			m.mode = modeSearch
			m.searchInput.SetValue(longOverview)
			return m
		}, true},
		{"detail", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeDetail
			m.selected = browseItem{title: "Dune", overview: longOverview}
			return m
		}, false},
		{"confirm", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeConfirm
			m.selected = browseItem{title: "Dune", overview: longOverview}
			return m
		}, false},
		{"result", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeResult
			m.err = errors.New(strings.Repeat("failure ", 60))
			return m
		}, false},
		{"requests", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeRequests
			updated, _ := m.Update(requestsPageLoadedMsg{items: []requestItem{testRequestItem()}})
			return updated.(model)
		}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.m()

			updated, _ := m.Update(tea.WindowSizeMsg{Width: 30, Height: 20})
			narrow := updated.(model)

			if narrow.width != 30 || narrow.height != 20 {
				t.Fatalf("width/height = %d/%d, want 30/20", narrow.width, narrow.height)
			}
			wantListWidth := 30 - listStyle.GetHorizontalFrameSize()
			if got := narrow.list.Width(); got != wantListWidth {
				t.Errorf("list width = %d, want %d (list should track window size, minus listStyle's border+padding)", got, wantListWidth)
			}
			if got := lipgloss.Width(narrow.View()); got > 30 {
				t.Errorf("View() at width 30 renders %d cells wide, want <= 30:\n%s", got, narrow.View())
			}
			if tt.expectExactHeight {
				if got := lipgloss.Height(narrow.View()); got != 20 {
					t.Errorf("View() at height 20 renders %d lines, want exactly 20 (footer anchored)", got)
				}
			}

			// Widening re-applies the new size rather than caching the
			// first wrap.
			updatedWide, _ := narrow.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
			wide := updatedWide.(model)
			if got := lipgloss.Width(wide.View()); got > 100 {
				t.Errorf("View() at width 100 renders %d cells wide, want <= 100", got)
			}
			if tt.expectExactHeight {
				if got := lipgloss.Height(wide.View()); got != 20 {
					t.Errorf("View() at height 20 renders %d lines, want exactly 20 (footer anchored)", got)
				}
			}
		})
	}
}

// Update: browsing mode

func TestUpdate_Browsing_EnterOpensDetail(t *testing.T) {
	m := newModel(context.Background(), nil)
	updated, _ := m.Update(pageLoadedMsg{items: []browseItem{testItem()}})
	m = updated.(model)

	updated, _ = m.Update(keyMsg("enter"))
	got := updated.(model)

	if got.mode != modeDetail {
		t.Errorf("mode = %v, want modeDetail", got.mode)
	}
	if got.selected.title != "Dune" {
		t.Errorf("selected = %+v, want Dune", got.selected)
	}
}

func TestUpdate_Browsing_TabTogglesTypeAndFetches(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/discover/tv" {
			t.Errorf("path = %q, want /api/v1/discover/tv (tab should switch to TV)", r.URL.Path)
		}
		w.Write([]byte(`{"page": 1, "totalPages": 1, "totalResults": 0, "results": []}`))
	})
	m := newModel(context.Background(), client)
	m.page = 3

	updated, cmd := m.Update(keyMsg("tab"))
	got := updated.(model)

	if got.mediaType != models.MediaTV {
		t.Errorf("mediaType = %v, want MediaTV", got.mediaType)
	}
	if got.page != 1 {
		t.Errorf("page = %d, want 1 (reset on type toggle)", got.page)
	}
	if !got.loading {
		t.Error("loading = false, want true")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want a fetch command")
	}
	if _, ok := cmd().(pageLoadedMsg); !ok {
		t.Error("cmd() did not hit /discover/tv as expected")
	}
}

func TestUpdate_Browsing_PageNavigation(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.page, m.totalPages = 2, 5

	// "p" (prev) and "n" (next) both fetch when in range.
	if _, cmd := m.Update(keyMsg("n")); cmd == nil {
		t.Error("\"n\" with page < totalPages: cmd = nil, want a fetch command")
	}
	if _, cmd := m.Update(keyMsg("p")); cmd == nil {
		t.Error("\"p\" with page > 1: cmd = nil, want a fetch command")
	}

	// Bounds: can't go below page 1 or past totalPages.
	atStart := m
	atStart.page = 1
	if _, cmd := atStart.Update(keyMsg("p")); cmd != nil {
		t.Error("\"p\" at page 1: cmd != nil, want no-op")
	}
	atEnd := m
	atEnd.page = 5
	if _, cmd := atEnd.Update(keyMsg("n")); cmd != nil {
		t.Error("\"n\" at last page: cmd != nil, want no-op")
	}

	// Already loading: further page requests are ignored.
	loading := m
	loading.loading = true
	if _, cmd := loading.Update(keyMsg("n")); cmd != nil {
		t.Error("\"n\" while loading: cmd != nil, want no-op")
	}
}

// Update: search mode

func TestUpdate_Browsing_SlashEntersSearchAndClearsStalePriorInput(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.searchInput.SetValue("leftover from a previous search")

	updated, _ := m.Update(keyMsg("/"))
	got := updated.(model)

	if got.mode != modeSearch {
		t.Errorf("mode = %v, want modeSearch", got.mode)
	}
	if got.searchInput.Value() != "" {
		t.Errorf("searchInput.Value() = %q, want empty: stale text from a previous search must not carry over", got.searchInput.Value())
	}
	if !got.searchInput.Focused() {
		t.Error("searchInput.Focused() = false, want true")
	}
}

func TestUpdate_Search_TypingEntersText(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeSearch
	m.searchInput.Focus()

	updated, _ := m.Update(keyMsg("d"))
	updated, _ = updated.(model).Update(keyMsg("u"))
	got := updated.(model)

	if got.searchInput.Value() != "du" {
		t.Errorf("searchInput.Value() = %q, want %q", got.searchInput.Value(), "du")
	}
}

func TestUpdate_Search_EnterWithEmptyQueryIsNoOp(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeSearch

	updated, cmd := m.Update(keyMsg("enter"))
	got := updated.(model)

	if got.mode != modeSearch {
		t.Errorf("mode = %v, want modeSearch (empty query shouldn't submit)", got.mode)
	}
	if cmd != nil {
		t.Error("cmd != nil, want no fetch for an empty query")
	}
}

func TestUpdate_Search_EnterSubmitsAndFetches(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Errorf("path = %q, want /api/v1/search", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != "dune" {
			t.Errorf("query param = %q, want %q", got, "dune")
		}
		w.Write([]byte(`{"page": 1, "totalPages": 1, "totalResults": 1,
			"results": [{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"}]}`))
	})
	m := newModel(context.Background(), client)
	m.mode = modeSearch
	m.searchInput.SetValue("  dune  ")

	updated, cmd := m.Update(keyMsg("enter"))
	got := updated.(model)

	if got.mode != modeBrowsing {
		t.Errorf("mode = %v, want modeBrowsing", got.mode)
	}
	if got.source != sourceSearch {
		t.Errorf("source = %v, want sourceSearch", got.source)
	}
	if got.query != "dune" {
		t.Errorf("query = %q, want %q (trimmed)", got.query, "dune")
	}
	if !got.loading {
		t.Error("loading = false, want true")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want a fetch command")
	}
	if _, ok := cmd().(pageLoadedMsg); !ok {
		t.Error("cmd() did not hit /search as expected")
	}
}

func TestUpdate_Search_EscCancelsWithoutChangingWhatWasShown(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.source, m.query = sourceDiscover, ""
	m.mode = modeSearch
	m.searchInput.SetValue("dune")

	updated, cmd := m.Update(keyMsg("esc"))
	got := updated.(model)

	if got.mode != modeBrowsing {
		t.Errorf("mode = %v, want modeBrowsing", got.mode)
	}
	if got.source != sourceDiscover || got.query != "" {
		t.Errorf("source/query = %v/%q, want unchanged (sourceDiscover/\"\")", got.source, got.query)
	}
	if cmd != nil {
		t.Error("cmd != nil, want no fetch: cancelling a search shouldn't refetch anything")
	}
}

func TestUpdate_Browsing_TabIsNoOpWhileSourceSearch(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.source, m.query = sourceSearch, "dune"
	m.mediaType = models.MediaMovie

	updated, cmd := m.Update(keyMsg("tab"))
	got := updated.(model)

	if got.mediaType != models.MediaMovie {
		t.Errorf("mediaType = %v, want unchanged MediaMovie", got.mediaType)
	}
	if cmd != nil {
		t.Error("cmd != nil, want no-op: nothing to toggle in mixed search results")
	}
}

func TestUpdate_Browsing_EscClearsSearchBackToDiscover(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/discover/movies" {
			t.Errorf("path = %q, want /api/v1/discover/movies", r.URL.Path)
		}
		w.Write([]byte(`{"page": 1, "totalPages": 1, "totalResults": 0, "results": []}`))
	})
	m := newModel(context.Background(), client)
	m.source, m.query, m.page = sourceSearch, "dune", 2

	updated, cmd := m.Update(keyMsg("esc"))
	got := updated.(model)

	if got.source != sourceDiscover {
		t.Errorf("source = %v, want sourceDiscover", got.source)
	}
	if got.query != "" {
		t.Errorf("query = %q, want cleared", got.query)
	}
	if got.page != 1 {
		t.Errorf("page = %d, want reset to 1", got.page)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want a fetch command")
	}
	if _, ok := cmd().(pageLoadedMsg); !ok {
		t.Error("cmd() did not hit /discover/movies as expected")
	}
}

// TestUpdate_Search_SecondSearchDoesNotBleedIntoFirst is the specific
// regression this was checked for before implementing: search, submit,
// then search again for something else — the second query must fully
// replace the first rather than merging with or racing against it.
func TestUpdate_Search_SecondSearchDoesNotBleedIntoFirst(t *testing.T) {
	var gotQueries []string
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQueries = append(gotQueries, r.URL.Query().Get("query"))
		w.Write([]byte(`{"page": 1, "totalPages": 1, "totalResults": 0, "results": []}`))
	})
	m := newModel(context.Background(), client)

	// First search: "dune".
	updated, _ := m.Update(keyMsg("/"))
	m = updated.(model)
	for _, r := range "dune" {
		updated, _ = m.Update(keyMsg(string(r)))
		m = updated.(model)
	}
	updated, cmd1 := m.Update(keyMsg("enter"))
	m = updated.(model)
	if _, ok := cmd1().(pageLoadedMsg); !ok {
		t.Fatal("first search's cmd did not resolve to pageLoadedMsg")
	}
	if m.query != "dune" {
		t.Fatalf("query after first search = %q, want %q", m.query, "dune")
	}

	// Second search, without ever clearing via esc: "heat".
	updated, _ = m.Update(keyMsg("/"))
	m = updated.(model)
	if m.searchInput.Value() != "" {
		t.Fatalf("searchInput.Value() = %q, want empty when starting a new search", m.searchInput.Value())
	}
	for _, r := range "heat" {
		updated, _ = m.Update(keyMsg(string(r)))
		m = updated.(model)
	}
	updated, cmd2 := m.Update(keyMsg("enter"))
	m = updated.(model)
	if _, ok := cmd2().(pageLoadedMsg); !ok {
		t.Fatal("second search's cmd did not resolve to pageLoadedMsg")
	}

	if m.query != "heat" {
		t.Errorf("query after second search = %q, want %q", m.query, "heat")
	}
	if len(gotQueries) != 2 || gotQueries[0] != "dune" || gotQueries[1] != "heat" {
		t.Errorf("queries sent to Seerr = %v, want [dune heat]", gotQueries)
	}
}

// TestUpdate_Search_StaleFirstSearchResponseDiscardedAfterSecondSearch
// covers the race variant: the first search's HTTP response is still
// in-flight (simulated by holding its pageLoadedMsg rather than feeding it
// immediately) when the second search is submitted. Feeding the stale
// response into Update afterward must not clobber the second search's
// state.
func TestUpdate_Search_StaleFirstSearchResponseDiscardedAfterSecondSearch(t *testing.T) {
	m := newModel(context.Background(), nil)

	updated, _ := m.Update(keyMsg("/"))
	m = updated.(model)
	m.searchInput.SetValue("dune")
	updated, cmd := m.Update(keyMsg("enter"))
	m = updated.(model)
	staleSeq := m.loadSeq
	_ = cmd // first search's request is never "delivered" — simulating it arriving late, below

	updated, _ = m.Update(keyMsg("/"))
	m = updated.(model)
	m.searchInput.SetValue("heat")
	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(model)

	if m.query != "heat" {
		t.Fatalf("query = %q, want %q before the stale response arrives", m.query, "heat")
	}

	// The first search's (now stale) response finally arrives.
	updated, _ = m.Update(pageLoadedMsg{
		seq: staleSeq, page: 1, totalPages: 1,
		items: []browseItem{{title: "Dune", mediaType: models.MediaMovie}},
	})
	got := updated.(model)

	if got.query != "heat" {
		t.Errorf("query = %q after a stale response arrived, want it to remain %q", got.query, "heat")
	}
	if len(got.list.Items()) == 1 {
		if it, ok := got.list.Items()[0].(browseItem); ok && it.title == "Dune" {
			t.Error("list was overwritten by the stale first search's results")
		}
	}
}

func TestUpdate_Search_QDoesNotQuitButCtrlCDoes(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeSearch
	m.searchInput.Focus()

	updated, cmd := m.Update(keyMsg("q"))
	got := updated.(model)
	if cmd != nil {
		if _, isQuit := cmd().(tea.QuitMsg); isQuit {
			t.Fatal("\"q\" quit the program while typing a search query")
		}
	}
	if got.searchInput.Value() != "q" {
		t.Errorf("searchInput.Value() = %q, want %q: \"q\" should be typed, not treated as quit", got.searchInput.Value(), "q")
	}

	_, cmd = m.Update(keyMsg("ctrl+c"))
	if cmd == nil {
		t.Fatal("ctrl+c: cmd = nil, want tea.Quit even while typing")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c did not quit while in modeSearch")
	}
}

// Update: detail/confirm/result modes

func TestUpdate_Detail_Transitions(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeDetail
	m.selected = testItem()

	updated, _ := m.Update(keyMsg("r"))
	if got := updated.(model); got.mode != modeConfirm {
		t.Errorf("after \"r\": mode = %v, want modeConfirm", got.mode)
	}

	updated, _ = m.Update(keyMsg("esc"))
	if got := updated.(model); got.mode != modeBrowsing {
		t.Errorf("after \"esc\": mode = %v, want modeBrowsing", got.mode)
	}
}

func TestUpdate_Confirm_YSubmits(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id": 11, "status": 1, "media": {"id": 1, "tmdbId": 42}}`))
	})
	m := newModel(context.Background(), client)
	m.mode = modeConfirm
	m.selected = testItem()

	updated, cmd := m.Update(keyMsg("y"))
	got := updated.(model)

	if !got.loading {
		t.Error("loading = false, want true after confirming")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want the request-submitting command")
	}
	result, ok := cmd().(requestResultMsg)
	if !ok || result.req == nil || result.req.ID != 11 {
		t.Errorf("cmd() = %#v, want requestResultMsg with request #11", cmd())
	}

	// Feeding the result back in should land on modeResult.
	updated, _ = got.Update(result)
	final := updated.(model)
	if final.mode != modeResult {
		t.Errorf("mode = %v, want modeResult", final.mode)
	}
	if final.lastRequest == nil || final.lastRequest.ID != 11 {
		t.Errorf("lastRequest = %+v, want request #11", final.lastRequest)
	}
}

// TestUpdate_Confirm_DefaultIsNo checks the [y/N] convention shared with the
// CLI's request/delete commands: only "y" submits; "n"/"esc"/a bare "enter"
// (taking the shown default) all cancel back to detail; anything else is a
// no-op.
func TestUpdate_Confirm_DefaultIsNo(t *testing.T) {
	for _, k := range []string{"n", "esc", "enter"} {
		m := newModel(context.Background(), nil)
		m.mode = modeConfirm
		m.selected = testItem()

		updated, cmd := m.Update(keyMsg(k))
		got := updated.(model)
		if got.mode != modeDetail {
			t.Errorf("key %q: mode = %v, want modeDetail", k, got.mode)
		}
		if cmd != nil {
			t.Errorf("key %q: cmd != nil, want no request submitted", k)
		}
	}

	// A key that's neither y/n/esc doesn't submit either.
	m := newModel(context.Background(), nil)
	m.mode = modeConfirm
	m.selected = testItem()
	updated, cmd := m.Update(keyMsg("x"))
	got := updated.(model)
	if got.mode != modeConfirm {
		t.Errorf("key \"x\": mode = %v, want modeConfirm (unchanged)", got.mode)
	}
	if cmd != nil {
		t.Error("key \"x\": cmd != nil, want no request submitted")
	}
}

func TestUpdate_Result_AnyKeyReturnsToBrowsing(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeResult
	m.err = errors.New("boom")
	m.lastRequest = &models.MediaRequest{ID: 1}

	updated, _ := m.Update(keyMsg("x"))
	got := updated.(model)

	if got.mode != modeBrowsing {
		t.Errorf("mode = %v, want modeBrowsing", got.mode)
	}
	if got.err != nil || got.lastRequest != nil {
		t.Errorf("err/lastRequest = %v/%v, want both cleared", got.err, got.lastRequest)
	}
}

// Update: requests mode

func TestUpdate_Browsing_SEntersRequestsAndFetches(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/request" {
			t.Errorf("path = %q, want /api/v1/request", r.URL.Path)
		}
		w.Write([]byte(`{"pageInfo": {"page": 1, "pages": 1, "results": 0}, "results": []}`))
	})
	m := newModel(context.Background(), client)

	updated, cmd := m.Update(keyMsg("s"))
	got := updated.(model)

	if got.mode != modeRequests {
		t.Errorf("mode = %v, want modeRequests", got.mode)
	}
	if !got.loading {
		t.Error("loading = false, want true")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want a fetch command")
	}
	if _, ok := cmd().(requestsPageLoadedMsg); !ok {
		t.Error("cmd() did not fetch requests as expected")
	}
}

func TestUpdate_Requests_EnterOpensRequestDetail(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequests
	updated, _ := m.Update(requestsPageLoadedMsg{items: []requestItem{testRequestItem()}})
	m = updated.(model)

	updated, _ = m.Update(keyMsg("enter"))
	got := updated.(model)

	if got.mode != modeRequestDetail {
		t.Errorf("mode = %v, want modeRequestDetail", got.mode)
	}
	if got.selectedRequest.id != 9 {
		t.Errorf("selectedRequest = %+v, want id 9", got.selectedRequest)
	}
}

func TestUpdate_Requests_PageNavigation(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequests
	m.requestsPage, m.requestsTotalPages = 2, 5

	// "p" (prev) and "n" (next) both fetch when in range.
	if _, cmd := m.Update(keyMsg("n")); cmd == nil {
		t.Error("\"n\" with page < totalPages: cmd = nil, want a fetch command")
	}
	if _, cmd := m.Update(keyMsg("p")); cmd == nil {
		t.Error("\"p\" with page > 1: cmd = nil, want a fetch command")
	}

	// Bounds: can't go below page 1 or past totalPages.
	atStart := m
	atStart.requestsPage = 1
	if _, cmd := atStart.Update(keyMsg("p")); cmd != nil {
		t.Error("\"p\" at page 1: cmd != nil, want no-op")
	}
	atEnd := m
	atEnd.requestsPage = 5
	if _, cmd := atEnd.Update(keyMsg("n")); cmd != nil {
		t.Error("\"n\" at last page: cmd != nil, want no-op")
	}

	// Already loading: further page requests are ignored.
	loading := m
	loading.loading = true
	if _, cmd := loading.Update(keyMsg("n")); cmd != nil {
		t.Error("\"n\" while loading: cmd != nil, want no-op")
	}
}

// TestUpdate_Requests_EscReturnsToBrowsingAndRefetches proves two things at
// once: leaving modeRequests refetches Discover/Search (m.list was just
// overwritten with requests, so the old browse contents are gone), and it
// does so using whatever source/query/page was active before "s" was
// pressed — not a reset to plain Discover.
func TestUpdate_Requests_EscReturnsToBrowsingAndRefetches(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Errorf("path = %q, want /api/v1/search (should resume the search that was active before \"s\")", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != "dune" {
			t.Errorf("query param = %q, want %q", got, "dune")
		}
		w.Write([]byte(`{"page": 1, "totalPages": 1, "totalResults": 0, "results": []}`))
	})
	m := newModel(context.Background(), client)
	m.source, m.query, m.page = sourceSearch, "dune", 3
	m.mode = modeRequests

	updated, cmd := m.Update(keyMsg("esc"))
	got := updated.(model)

	if got.mode != modeBrowsing {
		t.Errorf("mode = %v, want modeBrowsing", got.mode)
	}
	if !got.loading {
		t.Error("loading = false, want true: leaving requests should refetch, showing the loading state")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want a fetch command")
	}
	if _, ok := cmd().(pageLoadedMsg); !ok {
		t.Error("cmd() did not refetch search \"dune\" as expected")
	}
}

func TestUpdate_RequestDetail_DOpensConfirm(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequestDetail
	m.selectedRequest = testRequestItem()

	updated, _ := m.Update(keyMsg("d"))
	if got := updated.(model); got.mode != modeRequestConfirm {
		t.Errorf("after \"d\": mode = %v, want modeRequestConfirm", got.mode)
	}

	updated, _ = m.Update(keyMsg("esc"))
	if got := updated.(model); got.mode != modeRequests {
		t.Errorf("after \"esc\": mode = %v, want modeRequests", got.mode)
	}
}

// TestUpdate_RequestDetail_EnterDoesNotDelete is the deliberate safety
// check called out when this feature was designed: unlike modeDetail
// (where both "r" and "enter" trigger the request), modeRequestDetail only
// binds "d" to delete — a destructive action shouldn't also fire on a
// navigation key like enter.
func TestUpdate_RequestDetail_EnterDoesNotDelete(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequestDetail
	m.selectedRequest = testRequestItem()

	updated, cmd := m.Update(keyMsg("enter"))
	got := updated.(model)

	if got.mode != modeRequestDetail {
		t.Errorf("mode = %v, want modeRequestDetail (unchanged)", got.mode)
	}
	if cmd != nil {
		t.Error("cmd != nil, want no-op: \"enter\" must not trigger delete")
	}
}

func TestUpdate_RequestConfirm_YSubmitsDelete(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/request/9" {
			t.Errorf("method/path = %s %s, want DELETE /api/v1/request/9", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	m := newModel(context.Background(), client)
	m.mode = modeRequestConfirm
	m.selectedRequest = testRequestItem()

	updated, cmd := m.Update(keyMsg("y"))
	got := updated.(model)

	if !got.loading {
		t.Error("loading = false, want true after confirming")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want the delete command")
	}
	result, ok := cmd().(deleteResultMsg)
	if !ok || result.id != 9 || result.err != nil {
		t.Errorf("cmd() = %#v, want deleteResultMsg{id: 9, err: nil}", cmd())
	}

	// Feeding the result back in should land on modeRequestResult.
	updated, _ = got.Update(result)
	final := updated.(model)
	if final.mode != modeRequestResult {
		t.Errorf("mode = %v, want modeRequestResult", final.mode)
	}
	if final.deletedRequestID != 9 {
		t.Errorf("deletedRequestID = %d, want 9", final.deletedRequestID)
	}
}

// TestUpdate_RequestConfirm_DefaultIsNo mirrors TestUpdate_Confirm_DefaultIsNo's
// [y/N] convention check, for delete instead of request.
func TestUpdate_RequestConfirm_DefaultIsNo(t *testing.T) {
	// "enter" is included deliberately: on a [y/N] prompt, a bare enter
	// should take the shown default (N), not be a no-op.
	for _, k := range []string{"n", "esc", "enter"} {
		m := newModel(context.Background(), nil)
		m.mode = modeRequestConfirm
		m.selectedRequest = testRequestItem()

		updated, cmd := m.Update(keyMsg(k))
		got := updated.(model)
		if got.mode != modeRequestDetail {
			t.Errorf("key %q: mode = %v, want modeRequestDetail", k, got.mode)
		}
		if cmd != nil {
			t.Errorf("key %q: cmd != nil, want no delete submitted", k)
		}
	}

	// A key that's neither y/n/esc doesn't submit either.
	m := newModel(context.Background(), nil)
	m.mode = modeRequestConfirm
	m.selectedRequest = testRequestItem()
	updated, cmd := m.Update(keyMsg("x"))
	got := updated.(model)
	if got.mode != modeRequestConfirm {
		t.Errorf("key \"x\": mode = %v, want modeRequestConfirm (unchanged)", got.mode)
	}
	if cmd != nil {
		t.Error("key \"x\": cmd != nil, want no delete submitted")
	}
}

// TestUpdate_RequestResult_AnyKeyReturnsToRequestsAndRefetches is the other
// deliberate difference from the create-request flow: updateResult doesn't
// refetch on its way back to browsing (a created request is still valid to
// browse), but a deleted request must not keep showing in the requests
// list, so this one does.
func TestUpdate_RequestResult_AnyKeyReturnsToRequestsAndRefetches(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/request" {
			t.Errorf("path = %q, want /api/v1/request", r.URL.Path)
		}
		w.Write([]byte(`{"pageInfo": {"page": 1, "pages": 1, "results": 0}, "results": []}`))
	})
	m := newModel(context.Background(), client)
	m.mode = modeRequestResult
	m.deletedRequestID = 9

	updated, cmd := m.Update(keyMsg("x"))
	got := updated.(model)

	if got.mode != modeRequests {
		t.Errorf("mode = %v, want modeRequests", got.mode)
	}
	if got.deletedRequestID != 0 {
		t.Errorf("deletedRequestID = %d, want 0 (cleared)", got.deletedRequestID)
	}
	if !got.loading {
		t.Error("loading = false, want true: returning should refetch so the deleted request doesn't keep showing")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want a fetch command")
	}
	if _, ok := cmd().(requestsPageLoadedMsg); !ok {
		t.Error("cmd() did not refetch requests as expected")
	}
}

// Update: global quit

func TestUpdate_QuitFromEveryMode(t *testing.T) {
	for _, mode := range []mode{
		modeBrowsing, modeDetail, modeConfirm, modeResult,
		modeRequests, modeRequestDetail, modeRequestConfirm, modeRequestResult,
	} {
		for _, k := range []string{"q", "ctrl+c"} {
			m := newModel(context.Background(), nil)
			m.mode = mode

			_, cmd := m.Update(keyMsg(k))
			if cmd == nil {
				t.Fatalf("mode %v key %q: cmd = nil, want tea.Quit", mode, k)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("mode %v key %q: cmd() = %#v, want tea.QuitMsg", mode, k, cmd())
			}
		}
	}
}
