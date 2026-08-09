package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/emzbtw/reel/internal/models"
)

// jsonOutput backs every command's --json flag (registered via
// addJSONFlag). Only one command runs per process, so a single shared
// variable is enough.
var jsonOutput bool

// addJSONFlag registers the shared --json flag on cmd.
func addJSONFlag(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output results as JSON instead of a table")
}

// renderResults prints results as-is (no separate JSON schema) when --json
// is set, or falls back to the numbered table over the already-flattened
// items. emptyMsg is only shown in table mode; JSON mode always emits a
// valid (possibly empty) JSON array.
func renderResults[T any](cmd *cobra.Command, results []T, items []pickable, emptyMsg string) error {
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), emptyMsg)
		return nil
	}

	printPickables(cmd.OutOrStdout(), items)
	return nil
}

// pickable is a movie or TV search/discover result, flattened to what a
// numbered picker needs to disambiguate rows.
type pickable struct {
	ID    int
	Title string
	Year  string
	Type  models.MediaType
}

// mediaOnly filters a mixed search/discover result list down to movies and
// TV shows (person results are dropped: they can't be requested), preserving
// the API's original ordering.
func mediaOnly(results []models.SearchResult) []pickable {
	var out []pickable
	for _, r := range results {
		switch r.MediaType {
		case models.MediaMovie:
			if r.Movie == nil {
				continue
			}
			out = append(out, pickable{
				ID:    r.Movie.ID,
				Title: r.Movie.Title,
				Year:  year(r.Movie.ReleaseDate),
				Type:  models.MediaMovie,
			})
		case models.MediaTV:
			if r.TV == nil {
				continue
			}
			out = append(out, pickable{
				ID:    r.TV.ID,
				Title: r.TV.Name,
				Year:  year(r.TV.FirstAirDate),
				Type:  models.MediaTV,
			})
		}
	}
	return out
}

// moviePickables converts a homogeneous movie list (e.g. from
// DiscoverMovies) into the same row shape mediaOnly produces.
func moviePickables(results []models.MovieResult) []pickable {
	out := make([]pickable, len(results))
	for i, m := range results {
		out[i] = pickable{ID: m.ID, Title: m.Title, Year: year(m.ReleaseDate), Type: models.MediaMovie}
	}
	return out
}

// tvPickables converts a homogeneous TV list (e.g. from DiscoverTV) into
// the same row shape mediaOnly produces.
func tvPickables(results []models.TvResult) []pickable {
	out := make([]pickable, len(results))
	for i, t := range results {
		out[i] = pickable{ID: t.ID, Title: t.Name, Year: year(t.FirstAirDate), Type: models.MediaTV}
	}
	return out
}

func year(date string) string {
	if len(date) < 4 {
		return "—"
	}
	return date[:4]
}

func typeLabel(t models.MediaType) string {
	if t == models.MediaTV {
		return "TV"
	}
	return "Movie"
}

func printPickables(w io.Writer, items []pickable) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tTitle\tYear\tType")
	for i, it := range items {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", i+1, it.Title, it.Year, typeLabel(it.Type))
	}
	tw.Flush()
}
