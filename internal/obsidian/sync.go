package obsidian

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/emzbtw/reel/internal/api"
	"github.com/emzbtw/reel/internal/models"
)

// Client is the slice of *api.Client that syncing needs. It is an interface
// so a plan can be built and applied against a stub in tests without
// standing up an HTTP server.
type Client interface {
	Search(ctx context.Context, query string, page int) (*api.SearchResults, error)
	CreateRequest(ctx context.Context, in api.CreateRequestInput) (*models.MediaRequest, error)
	ListRequests(ctx context.Context, opts api.ListRequestsOptions) (*api.RequestList, error)
	MediaStatus(ctx context.Context, t models.MediaType, tmdbID int) (*models.MediaInfo, error)
}

// Action is what a sync will do to one task line.
type Action int

const (
	// ActionNone means the line is already correct; nothing to do.
	ActionNone Action = iota
	// ActionRequest means reel will submit a Seerr request for this line
	// and then write the resulting marker.
	ActionRequest
	// ActionMarker means the line is already bound and only its marker
	// needs updating to match Seerr.
	ActionMarker
	// ActionAmbiguous means the title could not be resolved to exactly one
	// result. Nothing is requested and the line is left untouched.
	ActionAmbiguous
	// ActionSkip means the line is deliberately out of scope: opted out,
	// user-completed, or carrying someone else's marker vocabulary.
	ActionSkip
)

// Candidate is one search result offered for an ambiguous line.
type Candidate struct {
	TmdbID        int
	Title         string
	OriginalTitle string
	Year          string
	MediaType     models.MediaType
	VoteCount     int
	Popularity    float64
}

// Item is the decision reached for a single task line.
type Item struct {
	Task       TaskLine
	Title      string
	Year       string
	Action     Action
	Reason     string      // why, for Skip and Ambiguous
	Candidates []Candidate // populated for Ambiguous
	// Matched is the search result this line resolved to, when the line was
	// resolved during this plan. It is nil for an already-bound line, whose
	// target is in Binding — nothing was searched for it.
	Matched   *Candidate
	Binding   Binding
	NewMarker Marker
}

// Plan is the full set of decisions for one note, computed without changing
// anything. Apply carries it out.
type Plan struct {
	NotePath string
	Items    []Item

	store *Store
}

// Requests returns the items that would submit a Seerr request. Callers
// confirm against this before applying, since requesting is the one part of
// a sync that reaches outside the machine.
func (p *Plan) Requests() []Item {
	var out []Item
	for _, it := range p.Items {
		if it.Action == ActionRequest {
			out = append(out, it)
		}
	}
	return out
}

// Ambiguous returns the items that could not be resolved and need the user
// to disambiguate.
func (p *Plan) Ambiguous() []Item {
	var out []Item
	for _, it := range p.Items {
		if it.Action == ActionAmbiguous {
			out = append(out, it)
		}
	}
	return out
}

// statusMarker maps Seerr's view of an item to the marker reel writes.
// Request status is consulted first: a declined request is a distinct,
// user-visible outcome that the media's own status does not capture. The
// bool reports whether Seerr said anything useful at all — an unknown status
// leaves the existing marker alone rather than guessing.
func statusMarker(media *models.MediaInfo, requestStatus int) (Marker, bool) {
	const requestDeclined = 3
	if requestStatus == requestDeclined {
		return Failed, true
	}
	if media == nil {
		return Unsynced, false
	}
	switch media.Status {
	case models.MediaStatusAvailable:
		return Available, true
	case models.MediaStatusProcessing, models.MediaStatusPartiallyAvailable:
		return Downloading, true
	case models.MediaStatusPending:
		return Requested, true
	case models.MediaStatusDeleted:
		return Failed, true
	default:
		return Unsynced, false
	}
}

// requestSweep pages through every Seerr request once, indexed by TMDB ID,
// so status for already-bound lines costs one sweep rather than a call per
// line.
func requestSweep(ctx context.Context, c Client) (map[int]models.MediaRequest, error) {
	const pageSize = 100
	out := map[int]models.MediaRequest{}
	for skip := 0; ; skip += pageSize {
		list, err := c.ListRequests(ctx, api.ListRequestsOptions{Take: pageSize, Skip: skip})
		if err != nil {
			return nil, err
		}
		for _, r := range list.Results {
			// Requests are returned newest first; keep the newest per item.
			if _, seen := out[r.Media.TmdbID]; !seen {
				out[r.Media.TmdbID] = r
			}
		}
		if len(list.Results) < pageSize {
			return out, nil
		}
	}
}

// BuildPlan decides what a sync of notePath would do, without changing
// anything on disk or in Seerr. Searching happens here (so an ambiguous
// title can be reported before anything is submitted); requesting and
// writing happen in Apply.
func BuildPlan(ctx context.Context, c Client, notePath string) (*Plan, error) {
	note, err := ParseNote(notePath)
	if err != nil {
		return nil, err
	}
	store, err := LoadStore(notePath)
	if err != nil {
		return nil, err
	}
	sweep, err := requestSweep(ctx, c)
	if err != nil {
		return nil, err
	}

	plan := &Plan{NotePath: notePath, store: store}
	for _, task := range note.Tasks {
		plan.Items = append(plan.Items, planTask(ctx, c, store, sweep, notePath, task))
	}
	return plan, nil
}

// planTask applies the sync decision table to one line. The governing rule
// is that the marker is reel's output, never its input: whether a request
// gets submitted is decided solely by whether a binding exists, so a user
// hand-resetting a marker can never cause a duplicate request.
func planTask(ctx context.Context, c Client, store *Store, sweep map[int]models.MediaRequest, notePath string, task TaskLine) Item {
	it := Item{Task: task, Title: task.Title, Year: task.Year}

	if task.Ignored {
		it.Action, it.Reason = ActionSkip, "opted out with %%reel:ignore%%"
		return it
	}
	if !task.Marker.Writable() {
		it.Action = ActionSkip
		if task.Marker == Done {
			it.Reason = "checked off by hand"
		} else {
			it.Reason = "carries a marker reel does not own"
		}
		return it
	}
	if strings.TrimSpace(task.Title) == "" {
		it.Action, it.Reason = ActionSkip, "no title on the line"
		return it
	}

	binding, bound := store.Get(notePath, task.Title)
	if bound {
		it.Binding = binding
		it.NewMarker, it.Action = markerFor(ctx, c, binding, sweep, task.Marker)
		return it
	}

	// Unbound. Resolve the title, but never guess between close candidates:
	// an unresolved line is reported and left alone rather than requested.
	res, err := resolve(ctx, c, task.Title, task.Year)
	if err != nil {
		it.Action, it.Reason = ActionSkip, fmt.Sprintf("search failed: %v", err)
		return it
	}
	if res.Picked == nil {
		it.Action = ActionAmbiguous
		it.Reason = res.Reason
		it.Candidates = topN(res.Ranked, maxReported)
		return it
	}

	picked := *res.Picked
	it.Matched = &picked
	it.Binding = Binding{
		Note:      notePath,
		Title:     task.Title,
		TmdbID:    picked.TmdbID,
		MediaType: string(picked.MediaType),
	}

	// A line reel has never seen may already be in the library, or already
	// requested by someone else. Requesting again would be wrong, so check
	// before deciding to.
	if req, ok := sweep[picked.TmdbID]; ok {
		it.Binding.RequestID = req.ID
		if m, ok := statusMarker(&req.Media, req.Status); ok {
			it.NewMarker = m
			it.Action = actionForMarker(m, task.Marker)
			return it
		}
	}
	if info, err := c.MediaStatus(ctx, picked.MediaType, picked.TmdbID); err == nil && info != nil {
		if m, ok := statusMarker(info, 0); ok && m == Available {
			it.NewMarker = m
			it.Action = actionForMarker(m, task.Marker)
			return it
		}
	}

	it.Action, it.NewMarker = ActionRequest, Requested
	return it
}

// markerFor resolves the current marker for an already-bound line. The
// request sweep covers everything reel asked Seerr for; MediaStatus is the
// fallback for items with no request record at all (already owned before
// reel existed, or a request an admin has since pruned).
func markerFor(ctx context.Context, c Client, b Binding, sweep map[int]models.MediaRequest, current Marker) (Marker, Action) {
	if req, ok := sweep[b.TmdbID]; ok {
		if m, ok := statusMarker(&req.Media, req.Status); ok {
			return m, actionForMarker(m, current)
		}
	}
	info, err := c.MediaStatus(ctx, models.MediaType(b.MediaType), b.TmdbID)
	if err != nil || info == nil {
		return current, ActionNone
	}
	if m, ok := statusMarker(info, 0); ok {
		return m, actionForMarker(m, current)
	}
	return current, ActionNone
}

func actionForMarker(m, current Marker) Action {
	if m == current {
		return ActionNone
	}
	return ActionMarker
}

// Matching thresholds. TMDB carries many same-titled, low-relevance entries
// for almost any common word — a plain title search for "Arrival" returns
// nine exact matches — so exact string equality alone reports nearly every
// line as ambiguous and forces the user to hand-annotate a year everywhere.
// Ranking by vote count and requiring the leader to dominate separates
// "the film people mean" from catalogue noise, while still reporting real
// collisions like Dune 1984 vs 2021 (a ratio of roughly 4).
const (
	// dominanceRatio is how far ahead the leading candidate must be to be
	// taken without asking.
	dominanceRatio = 10
	// minVotesToJudge is the vote count below which a candidate is treated
	// as too new to rank on votes at all.
	//
	// This is a rough constant, picked to sit above catalogue noise and
	// below anything with a real audience. It is worth revisiting once this
	// has run against a real vault for a while — treat it as a starting
	// point, not a settled value.
	minVotesToJudge = 20
	// maxReported caps how many candidates an ambiguous line lists.
	maxReported = 5
)

// resolution is the outcome of matching one note title against search
// results: either a confident pick, or the ranked candidates and the reason
// no pick was made.
type resolution struct {
	Picked *Candidate
	Ranked []Candidate
	Reason string
}

// resolve searches Seerr for title and decides whether any one result can be
// acted on without asking. It never guesses between close candidates.
func resolve(ctx context.Context, c Client, title, year string) (resolution, error) {
	results, err := c.Search(ctx, title, 0)
	if err != nil {
		return resolution{}, err
	}

	want := normalizeForMatch(title)
	var matches []Candidate
	for _, r := range results.Results {
		cand, ok := candidateOf(r)
		if !ok {
			continue
		}
		// The original-language title is checked too: a note may well use
		// the title a user knows the film by rather than the localized one.
		if normalizeForMatch(cand.Title) != want && normalizeForMatch(cand.OriginalTitle) != want {
			continue
		}
		matches = append(matches, cand)
	}
	if len(matches) == 0 {
		return resolution{Reason: "no movie or TV match"}, nil
	}
	rank(matches)

	if year != "" {
		var byYear []Candidate
		for _, m := range matches {
			if m.Year == year {
				byYear = append(byYear, m)
			}
		}
		// An explicit year is the user telling us which one they mean.
		// Falling back to the unfiltered set when it matches nothing would
		// discard that and pick something they didn't ask for, so report
		// instead.
		if len(byYear) == 0 {
			return resolution{
				Ranked: matches,
				Reason: fmt.Sprintf("nothing matching released in %s — check the year", year),
			}, nil
		}
		if len(byYear) == 1 {
			return resolution{Picked: &byYear[0]}, nil
		}
		matches = byYear
	}

	if len(matches) == 1 || dominates(matches[0], matches[1]) {
		return resolution{Picked: &matches[0]}, nil
	}
	return resolution{
		Ranked: matches,
		Reason: "several close matches — add a year to pick one",
	}, nil
}

// rank orders candidates by how likely they are to be the one a person
// writing this title meant. Vote count leads because it accumulates with a
// genuine audience, where popularity is recency-weighted and a freshly
// released obscurity can briefly outrank a classic.
//
// But when nothing in the set clears minVotesToJudge, vote counts are noise
// — three votes on a forgotten short would otherwise outrank an unreleased
// film the whole internet is waiting for. In that case the set is ranked on
// popularity instead, which is the only signal that has had time to move.
// This keeps ranking and dominates consistent: whichever signal ordered the
// list is the one the leader is then judged on.
func rank(cands []Candidate) {
	byVotes := false
	for _, c := range cands {
		if c.VoteCount >= minVotesToJudge {
			byVotes = true
			break
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if byVotes {
			if a.VoteCount != b.VoteCount {
				return a.VoteCount > b.VoteCount
			}
			return a.Popularity > b.Popularity
		}
		if a.Popularity != b.Popularity {
			return a.Popularity > b.Popularity
		}
		return a.VoteCount > b.VoteCount
	})
}

// dominates reports whether top is so far ahead of the runner-up that it can
// be taken without asking. Only the runner-up matters: beating it means
// beating everything below it.
func dominates(top, second Candidate) bool {
	if top.VoteCount >= minVotesToJudge {
		return top.VoteCount >= dominanceRatio*second.VoteCount
	}
	// Not enough votes to judge — an unreleased or just-released title.
	// Popularity reacts within days where vote counts take months, so it is
	// the only usable signal here. Two candidates with no signal at all are
	// never dominant over each other.
	return top.Popularity > 0 && top.Popularity >= dominanceRatio*second.Popularity
}

// normalizeForMatch reduces a title to lowercase letters, digits, and single
// spaces, so punctuation differences don't defeat a match: a note reading
// "Alien Romulus" should still match TMDB's "Alien: Romulus".
func normalizeForMatch(s string) string {
	var b strings.Builder
	pendingSpace := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if pendingSpace && b.Len() > 0 {
				b.WriteRune(' ')
			}
			b.WriteRune(r)
			pendingSpace = false
			continue
		}
		pendingSpace = true
	}
	return b.String()
}

func topN(cands []Candidate, n int) []Candidate {
	if len(cands) <= n {
		return cands
	}
	return cands[:n]
}

// candidateOf flattens one search result into the fields matching needs.
// It is deliberately separate from internal/cli's picker rows: those exist
// to disambiguate for a human and drop the original-language titles that
// matching wants to consider.
func candidateOf(r models.SearchResult) (Candidate, bool) {
	switch r.MediaType {
	case models.MediaMovie:
		if r.Movie == nil {
			return Candidate{}, false
		}
		return Candidate{
			TmdbID:        r.Movie.ID,
			Title:         r.Movie.Title,
			OriginalTitle: r.Movie.OriginalTitle,
			Year:          yearOf(r.Movie.ReleaseDate),
			MediaType:     models.MediaMovie,
			VoteCount:     r.Movie.VoteCount,
			Popularity:    r.Movie.Popularity,
		}, true
	case models.MediaTV:
		if r.TV == nil {
			return Candidate{}, false
		}
		return Candidate{
			TmdbID:        r.TV.ID,
			Title:         r.TV.Name,
			OriginalTitle: r.TV.OriginalName,
			Year:          yearOf(r.TV.FirstAirDate),
			MediaType:     models.MediaTV,
			VoteCount:     r.TV.VoteCount,
			Popularity:    r.TV.Popularity,
		}, true
	default:
		return Candidate{}, false
	}
}

func yearOf(date string) string {
	if len(date) < 4 {
		return ""
	}
	return date[:4]
}

// Result reports what Apply actually did.
type Result struct {
	Requested []Item
	Written   WriteResult
	// Unwritten are marker changes that could not be applied because the
	// line changed on disk while the sync was talking to Seerr. Their Seerr
	// side already happened and the binding records it, so the next sync
	// re-applies them.
	Unwritten []Edit
}

// Apply carries out a plan: submits the requests, then writes the resulting
// markers back into the note and saves the bindings.
//
// Requests are submitted before the note is written, and every binding is
// recorded regardless of whether its line could be written. A marker is a
// rendering of state held in Seerr and the store; losing one to a concurrent
// edit costs nothing but a redraw on the next sync.
func Apply(ctx context.Context, c Client, p *Plan) (Result, error) {
	var res Result

	for i := range p.Items {
		it := &p.Items[i]
		if it.Action != ActionRequest {
			continue
		}
		in := api.CreateRequestInput{
			MediaType: models.MediaType(it.Binding.MediaType),
			MediaID:   it.Binding.TmdbID,
		}
		if in.MediaType == models.MediaTV {
			in.AllSeasons = true
		}
		req, err := c.CreateRequest(ctx, in)
		if err != nil {
			it.Action, it.Reason = ActionSkip, fmt.Sprintf("request failed: %v", err)
			continue
		}
		it.Binding.RequestID = req.ID
		res.Requested = append(res.Requested, *it)
	}

	var edits []Edit
	for _, it := range p.Items {
		switch it.Action {
		case ActionRequest, ActionMarker:
			if it.NewMarker != it.Task.Marker {
				edits = append(edits, Edit{OriginalLine: it.Task.Raw, NewMarker: it.NewMarker})
			}
		}
	}

	written, err := WriteEdits(p.NotePath, edits)
	if err != nil && err != ErrRecentlyModified {
		return res, err
	}
	res.Written = written
	res.Unwritten = written.Skipped
	if err == ErrRecentlyModified {
		res.Unwritten = edits
	}

	// Record bindings even when the note could not be written, so the next
	// sync knows these lines are already requested and does not submit them
	// a second time.
	for _, it := range p.Items {
		if it.Binding.TmdbID == 0 {
			continue
		}
		b := it.Binding
		if it.NewMarker != Unsynced {
			b.LastMarker = string(it.NewMarker.Rune())
			b.LastStatus = it.NewMarker.String()
		}
		p.store.Put(b)
	}
	if err := p.store.Save(); err != nil {
		return res, err
	}
	return res, nil
}
