package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/emzbtw/reel/internal/models"
)

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
