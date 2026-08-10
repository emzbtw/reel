package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emzbtw/reel/internal/api"
	"github.com/emzbtw/reel/internal/models"
)

// stubClient is a Client that serves canned results, so the decision table
// can be exercised without an HTTP server.
type stubClient struct {
	search   map[string][]models.SearchResult
	requests []models.MediaRequest
	media    map[int]*models.MediaInfo

	created  []api.CreateRequestInput
	createID int
	nextID   int
}

func (s *stubClient) Search(_ context.Context, query string, _ int) (*api.SearchResults, error) {
	return &api.SearchResults{Results: s.search[query]}, nil
}

func (s *stubClient) CreateRequest(_ context.Context, in api.CreateRequestInput) (*models.MediaRequest, error) {
	s.created = append(s.created, in)
	s.nextID++
	return &models.MediaRequest{ID: s.createID + s.nextID}, nil
}

func (s *stubClient) ListRequests(_ context.Context, _ api.ListRequestsOptions) (*api.RequestList, error) {
	return &api.RequestList{Results: s.requests}, nil
}

func (s *stubClient) MediaStatus(_ context.Context, _ models.MediaType, tmdbID int) (*models.MediaInfo, error) {
	return s.media[tmdbID], nil
}

func movieResult(id int, title, date string) models.SearchResult {
	return models.SearchResult{
		MediaType: models.MediaMovie,
		Movie:     &models.MovieResult{ID: id, MediaType: models.MediaMovie, Title: title, ReleaseDate: date},
	}
}

func tvResult(id int, name, date string) models.SearchResult {
	return models.SearchResult{
		MediaType: models.MediaTV,
		TV:        &models.TvResult{ID: id, MediaType: models.MediaTV, Name: name, FirstAirDate: date},
	}
}

// withScore attaches the ranking signals to a search result.
func withScore(r models.SearchResult, voteCount int, popularity float64) models.SearchResult {
	switch r.MediaType {
	case models.MediaMovie:
		r.Movie.VoteCount, r.Movie.Popularity = voteCount, popularity
	case models.MediaTV:
		r.TV.VoteCount, r.TV.Popularity = voteCount, popularity
	}
	return r
}

// TestResolve_Dominance calibrates the matching rule against the result sets
// a real Seerr instance returns. TMDB carries many same-titled, low-relevance
// entries for common words, so requiring a single exact title match reported
// almost every line as ambiguous. Vote counts here approximate real TMDB
// values; the candidate lists and TMDB IDs are from an actual dry run.
func TestResolve_Dominance(t *testing.T) {
	// "Arrival": nine exact matches, one of which is the film anyone means.
	arrival := []models.SearchResult{
		withScore(movieResult(329865, "Arrival", "2016-11-10"), 18000, 52.1),
		withScore(movieResult(683330, "Arrival", "1986-01-01"), 12, 1.4),
		withScore(movieResult(696430, "Arrival", "2018-01-01"), 3, 0.8),
		withScore(movieResult(1100190, "Arrival", "1957-01-01"), 1, 0.3),
		withScore(movieResult(1453473, "arrival", "2025-01-01"), 0, 1.1),
		withScore(movieResult(637776, "Arrival", "2019-01-01"), 2, 0.5),
		withScore(movieResult(472349, "Arrival", "2016-01-01"), 4, 0.6),
		withScore(movieResult(247620, "Arrival", "2013-01-01"), 6, 0.4),
		withScore(movieResult(1738637, "Arrival", "2026-01-01"), 0, 2.3),
	}
	// "Heat": the Mann film plus two same-titled TV series.
	heat := []models.SearchResult{
		withScore(movieResult(949, "Heat", "1995-12-15"), 7000, 41.0),
		withScore(tvResult(86973, "HEAT", "2015-01-01"), 40, 3.2),
		withScore(tvResult(230529, "HEAT", "2023-01-01"), 8, 5.6),
	}
	// "Dune": the genuine collision — two adaptations with real audiences.
	dune := []models.SearchResult{
		withScore(movieResult(438631, "Dune", "2021-09-15"), 11000, 210.0),
		withScore(movieResult(841, "Dune", "1984-12-14"), 2700, 31.0),
		withScore(movieResult(697620, "Dune", "2020-01-01"), 5, 1.2),
		withScore(movieResult(627150, "Dune", "1989-01-01"), 2, 0.4),
	}

	tests := []struct {
		name    string
		query   string
		year    string
		results []models.SearchResult
		want    int // TMDB ID, or 0 meaning "must stay ambiguous"
	}{
		{"arrival clears catalogue noise", "Arrival", "", arrival, 329865},
		{"heat outweighs same-titled TV", "Heat", "", heat, 949},
		{"dune is a real collision and stays ambiguous", "Dune", "", dune, 0},
		{"year hint resolves dune", "Dune", "1984", dune, 841},
		{"year hint overrides the dominant pick", "Arrival", "1986", arrival, 683330},
		{
			// An explicit year matching nothing must be reported, not
			// silently replaced with the dominant candidate.
			name: "year hint matching nothing is reported", query: "Arrival", year: "1999",
			results: arrival, want: 0,
		},
		{
			// Punctuation differences must not defeat matching.
			name: "punctuation is normalized away", query: "Alien Romulus", year: "",
			results: []models.SearchResult{
				withScore(movieResult(1064213, "Alien: Romulus", "2024-08-13"), 4000, 180.0),
			},
			want: 1064213,
		},
		{
			// Unreleased titles have no votes; popularity is the only signal.
			name: "upcoming title resolves on popularity", query: "Untitled", year: "",
			results: []models.SearchResult{
				withScore(movieResult(111, "Untitled", "2027-01-01"), 0, 90.0),
				withScore(movieResult(222, "Untitled", "1998-01-01"), 3, 0.5),
			},
			want: 111,
		},
		{
			// Two candidates with no signal at all cannot dominate.
			name: "two unknowns stay ambiguous", query: "Obscure", year: "",
			results: []models.SearchResult{
				withScore(movieResult(333, "Obscure", "2001-01-01"), 0, 0),
				withScore(movieResult(444, "Obscure", "2002-01-01"), 0, 0),
			},
			want: 0,
		},
		{
			name: "no match at all", query: "Nonexistent", year: "",
			results: []models.SearchResult{movieResult(555, "Something Else", "2001-01-01")},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &stubClient{search: map[string][]models.SearchResult{tt.query: tt.results}}
			got, err := resolve(context.Background(), c, tt.query, tt.year)
			if err != nil {
				t.Fatalf("resolve() returned error: %v", err)
			}
			if tt.want == 0 {
				if got.Picked != nil {
					t.Fatalf("resolved to tmdb %d, want it reported as ambiguous", got.Picked.TmdbID)
				}
				if got.Reason == "" {
					t.Error("ambiguous result carries no reason to show the user")
				}
				return
			}
			if got.Picked == nil {
				t.Fatalf("reported as ambiguous (%s), want tmdb %d", got.Reason, tt.want)
			}
			if got.Picked.TmdbID != tt.want {
				t.Errorf("resolved to tmdb %d, want %d", got.Picked.TmdbID, tt.want)
			}
		})
	}
}

func TestResolve_ReportedCandidatesAreCappedAndRanked(t *testing.T) {
	var results []models.SearchResult
	for i := 0; i < 9; i++ {
		results = append(results, withScore(movieResult(100+i, "Common", "2000-01-01"), i, float64(i)))
	}
	c := &stubClient{search: map[string][]models.SearchResult{"Common": results}}

	got, err := resolve(context.Background(), c, "Common", "")
	if err != nil {
		t.Fatalf("resolve() returned error: %v", err)
	}
	if got.Picked != nil {
		t.Fatalf("resolved to tmdb %d, want ambiguous", got.Picked.TmdbID)
	}
	capped := topN(got.Ranked, maxReported)
	if len(capped) != maxReported {
		t.Fatalf("reported %d candidates, want %d", len(capped), maxReported)
	}
	for i := 1; i < len(capped); i++ {
		if capped[i-1].VoteCount < capped[i].VoteCount {
			t.Errorf("candidates not ranked by vote count: %+v", capped)
		}
	}
}

// syncVault writes contents to Movies.md inside a fresh vault and returns
// the note path, with an mtime old enough that WriteEdits will not refuse.
func syncVault(t *testing.T, contents string) string {
	t.Helper()
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vault, "Movies.md")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	return path
}

func itemFor(t *testing.T, p *Plan, title string) Item {
	t.Helper()
	for _, it := range p.Items {
		if it.Title == title {
			return it
		}
	}
	t.Fatalf("no plan item for title %q", title)
	return Item{}
}

func TestBuildPlan_UnboundLineIsRequested(t *testing.T) {
	path := syncVault(t, "- [ ] Arrival\n")
	c := &stubClient{search: map[string][]models.SearchResult{
		"Arrival": {movieResult(329865, "Arrival", "2016-11-10")},
	}}

	plan, err := BuildPlan(context.Background(), c, path)
	if err != nil {
		t.Fatalf("BuildPlan() returned error: %v", err)
	}
	it := itemFor(t, plan, "Arrival")
	if it.Action != ActionRequest {
		t.Fatalf("action = %v, want ActionRequest", it.Action)
	}
	if it.Binding.TmdbID != 329865 {
		t.Errorf("bound tmdbID = %d, want 329865", it.Binding.TmdbID)
	}
	// The plan has to be able to name what it resolved to, or the
	// confirmation prompt can't say what it is about to request.
	if it.Matched == nil {
		t.Fatal("Matched is nil, so the plan cannot show what this line resolved to")
	}
	if it.Matched.TmdbID != 329865 || it.Matched.Title != "Arrival" {
		t.Errorf("Matched = %+v, want Arrival/329865", *it.Matched)
	}
}

// Several exact title matches must never be guessed between.
func TestBuildPlan_AmbiguousTitleIsNotRequested(t *testing.T) {
	path := syncVault(t, "- [ ] Alien\n")
	c := &stubClient{search: map[string][]models.SearchResult{
		"Alien": {
			movieResult(348, "Alien", "1979-05-25"),
			tvResult(158310, "Alien", "2024-01-01"),
		},
	}}

	plan, err := BuildPlan(context.Background(), c, path)
	if err != nil {
		t.Fatalf("BuildPlan() returned error: %v", err)
	}
	it := itemFor(t, plan, "Alien")
	if it.Action != ActionAmbiguous {
		t.Fatalf("action = %v, want ActionAmbiguous", it.Action)
	}
	if len(it.Candidates) != 2 {
		t.Errorf("candidates = %d, want 2", len(it.Candidates))
	}

	res, err := Apply(context.Background(), c, plan)
	if err != nil {
		t.Fatalf("Apply() returned error: %v", err)
	}
	if len(c.created) != 0 {
		t.Errorf("created %d request(s) for an ambiguous line, want 0", len(c.created))
	}
	if len(res.Written.Applied) != 0 {
		t.Errorf("wrote %d marker(s) for an ambiguous line, want 0", len(res.Written.Applied))
	}
}

// A year hint is what turns an ambiguous title into a confident one.
func TestBuildPlan_YearHintDisambiguates(t *testing.T) {
	path := syncVault(t, "- [ ] Alien (1979)\n")
	c := &stubClient{search: map[string][]models.SearchResult{
		"Alien": {
			movieResult(348, "Alien", "1979-05-25"),
			tvResult(158310, "Alien", "2024-01-01"),
		},
	}}

	plan, err := BuildPlan(context.Background(), c, path)
	if err != nil {
		t.Fatalf("BuildPlan() returned error: %v", err)
	}
	it := itemFor(t, plan, "Alien")
	if it.Action != ActionRequest {
		t.Fatalf("action = %v (%s), want ActionRequest", it.Action, it.Reason)
	}
	if it.Binding.TmdbID != 348 {
		t.Errorf("bound tmdbID = %d, want 348 (the 1979 film)", it.Binding.TmdbID)
	}
}

// The marker is reel's output, not its input: a hand-reset marker on a bound
// line must re-render from Seerr, never trigger a second request.
func TestBuildPlan_HandResetMarkerDoesNotReRequest(t *testing.T) {
	path := syncVault(t, "- [ ] Arrival\n")
	c := &stubClient{
		search: map[string][]models.SearchResult{
			"Arrival": {movieResult(329865, "Arrival", "2016-11-10")},
		},
		requests: []models.MediaRequest{{
			ID:     7,
			Status: 2,
			Media:  models.MediaInfo{TmdbID: 329865, Status: models.MediaStatusAvailable},
		}},
	}

	// First sync binds the line and marks it available.
	plan, err := BuildPlan(context.Background(), c, path)
	if err != nil {
		t.Fatalf("BuildPlan() returned error: %v", err)
	}
	if _, err := Apply(context.Background(), c, plan); err != nil {
		t.Fatalf("Apply() returned error: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != "- [✓] Arrival\n" {
		t.Fatalf("after first sync = %q, want the available marker", after)
	}

	// The user resets the checkbox by hand.
	if err := os.WriteFile(path, []byte("- [ ] Arrival\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	plan2, err := BuildPlan(context.Background(), c, path)
	if err != nil {
		t.Fatalf("BuildPlan() returned error: %v", err)
	}
	it := itemFor(t, plan2, "Arrival")
	if it.Action != ActionMarker {
		t.Fatalf("action = %v, want ActionMarker (re-render, not re-request)", it.Action)
	}
	if _, err := Apply(context.Background(), c, plan2); err != nil {
		t.Fatalf("Apply() returned error: %v", err)
	}
	if len(c.created) != 0 {
		t.Errorf("created %d request(s) after a hand reset, want 0", len(c.created))
	}
	after2, _ := os.ReadFile(path)
	if string(after2) != "- [✓] Arrival\n" {
		t.Errorf("after second sync = %q, want the marker re-rendered", after2)
	}
}

// [x] means "stop writing to this line", not "forget it" — reel must leave
// the line completely alone.
func TestBuildPlan_CheckedAndForeignLinesAreLeftAlone(t *testing.T) {
	const contents = "- [x] Arrival\n- [-] Heat\n- [/] Alien\n"
	path := syncVault(t, contents)
	c := &stubClient{}

	plan, err := BuildPlan(context.Background(), c, path)
	if err != nil {
		t.Fatalf("BuildPlan() returned error: %v", err)
	}
	for _, it := range plan.Items {
		if it.Action != ActionSkip {
			t.Errorf("%q action = %v, want ActionSkip", it.Title, it.Action)
		}
	}
	if _, err := Apply(context.Background(), c, plan); err != nil {
		t.Fatalf("Apply() returned error: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != contents {
		t.Errorf("file = %q, want it untouched", after)
	}
	if len(c.created) != 0 {
		t.Errorf("created %d request(s), want 0", len(c.created))
	}
}

func TestBuildPlan_IgnoreTokenIsSkipped(t *testing.T) {
	path := syncVault(t, "- [ ] Arrival %%reel:ignore%%\n")
	c := &stubClient{search: map[string][]models.SearchResult{
		"Arrival": {movieResult(329865, "Arrival", "2016-11-10")},
	}}

	plan, err := BuildPlan(context.Background(), c, path)
	if err != nil {
		t.Fatalf("BuildPlan() returned error: %v", err)
	}
	if plan.Items[0].Action != ActionSkip {
		t.Errorf("action = %v, want ActionSkip", plan.Items[0].Action)
	}
}

// Media already in the library must be marked available, not requested
// again.
func TestBuildPlan_AlreadyAvailableIsNotRequested(t *testing.T) {
	path := syncVault(t, "- [ ] Arrival\n")
	c := &stubClient{
		search: map[string][]models.SearchResult{
			"Arrival": {movieResult(329865, "Arrival", "2016-11-10")},
		},
		media: map[int]*models.MediaInfo{
			329865: {TmdbID: 329865, Status: models.MediaStatusAvailable},
		},
	}

	plan, err := BuildPlan(context.Background(), c, path)
	if err != nil {
		t.Fatalf("BuildPlan() returned error: %v", err)
	}
	it := itemFor(t, plan, "Arrival")
	if it.Action != ActionMarker || it.NewMarker != Available {
		t.Fatalf("action = %v marker = %v, want ActionMarker/Available", it.Action, it.NewMarker)
	}
	if _, err := Apply(context.Background(), c, plan); err != nil {
		t.Fatalf("Apply() returned error: %v", err)
	}
	if len(c.created) != 0 {
		t.Errorf("created %d request(s) for already-available media, want 0", len(c.created))
	}
}

// A declined request renders [✗] rather than falling back to [ ], which
// would wrongly read as "not yet requested".
func TestBuildPlan_DeclinedRequestRendersFailed(t *testing.T) {
	path := syncVault(t, "- [🎬] Arrival\n")
	c := &stubClient{
		search: map[string][]models.SearchResult{
			"Arrival": {movieResult(329865, "Arrival", "2016-11-10")},
		},
		requests: []models.MediaRequest{{
			ID:     7,
			Status: 3, // declined
			Media:  models.MediaInfo{TmdbID: 329865, Status: models.MediaStatusUnknown},
		}},
	}

	plan, err := BuildPlan(context.Background(), c, path)
	if err != nil {
		t.Fatalf("BuildPlan() returned error: %v", err)
	}
	it := itemFor(t, plan, "Arrival")
	if it.NewMarker != Failed {
		t.Fatalf("marker = %v, want Failed", it.NewMarker)
	}
	if _, err := Apply(context.Background(), c, plan); err != nil {
		t.Fatalf("Apply() returned error: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != "- [✗] Arrival\n" {
		t.Errorf("file = %q, want the failed marker", after)
	}
}

// Everything but the marker byte must survive a sync untouched.
func TestApply_LeavesTheRestOfTheNoteIntact(t *testing.T) {
	const contents = "---\ntags: [media]\n---\n\n# Movies\n\n- [ ] Arrival #film\n  - nested note\n\n```\n- [ ] NotATask\n```\n"
	path := syncVault(t, contents)
	c := &stubClient{search: map[string][]models.SearchResult{
		"Arrival": {movieResult(329865, "Arrival", "2016-11-10")},
	}}

	plan, err := BuildPlan(context.Background(), c, path)
	if err != nil {
		t.Fatalf("BuildPlan() returned error: %v", err)
	}
	if _, err := Apply(context.Background(), c, plan); err != nil {
		t.Fatalf("Apply() returned error: %v", err)
	}

	after, _ := os.ReadFile(path)
	want := strings.Replace(contents, "- [ ] Arrival #film", "- [🎬] Arrival #film", 1)
	if string(after) != want {
		t.Errorf("file = %q,\nwant %q", after, want)
	}
}

// A binding must be recorded even when the note could not be written, so the
// next sync does not submit the same request a second time.
func TestApply_RecordsBindingEvenWhenTheNoteChanged(t *testing.T) {
	path := syncVault(t, "- [ ] Arrival\n")
	c := &stubClient{search: map[string][]models.SearchResult{
		"Arrival": {movieResult(329865, "Arrival", "2016-11-10")},
	}}

	plan, err := BuildPlan(context.Background(), c, path)
	if err != nil {
		t.Fatalf("BuildPlan() returned error: %v", err)
	}

	// The user rewrites the line while reel is talking to Seerr.
	if err := os.WriteFile(path, []byte("- [ ] Arrival (2016)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	res, err := Apply(context.Background(), c, plan)
	if err != nil {
		t.Fatalf("Apply() returned error: %v", err)
	}
	if len(res.Written.Applied) != 0 || len(res.Unwritten) != 1 {
		t.Errorf("written = %+v, unwritten = %d, want 0 applied / 1 unwritten", res.Written, len(res.Unwritten))
	}
	if len(c.created) != 1 {
		t.Fatalf("created %d request(s), want 1", len(c.created))
	}

	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore() returned error: %v", err)
	}
	if _, ok := store.Get(path, "Arrival"); !ok {
		t.Error("binding was not persisted, so the next sync would request Arrival again")
	}
}
