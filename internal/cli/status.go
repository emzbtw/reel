package cli

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/emzbtw/reel/internal/api"
	"github.com/emzbtw/reel/internal/models"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "List current requests",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		list, err := client.ListRequests(cmd.Context(), api.ListRequestsOptions{})
		if err != nil {
			return err
		}

		if jsonOutput {
			return writeJSON(cmd.OutOrStdout(), list.Results)
		}

		if len(list.Results) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No requests found.")
			return nil
		}

		printRequestsTable(cmd.Context(), cmd.OutOrStdout(), client, list.Results)
		return nil
	},
}

// printRequestsTable renders requests using the same columns wherever a
// request needs to be shown (status's listing, delete's confirmation
// prompt). Seerr's GET /request doesn't join in a title or year (MediaInfo
// only carries tmdbId/tvdbId), so each row's title/year is resolved
// separately via api.ResolveTitles — the same cached, concurrent TMDB-ID
// lookup internal/tui's requests view uses, so the CLI table carries the
// same title/year/type disambiguation the TUI's list shows, just as
// columns instead of a "(<year> · Movie/TV)" tag. The cache is fresh per
// call: a CLI invocation is a single process, so there's nothing to
// persist across runs, but it still dedupes repeated TMDB IDs within one
// table. A failed title lookup falls back to the TMDB ID (with an empty
// Year), matching the TUI's fallback.
func printRequestsTable(ctx context.Context, w io.Writer, client *api.Client, requests []models.MediaRequest) {
	queries := make([]api.TitleQuery, len(requests))
	for i, r := range requests {
		queries[i] = api.TitleQuery{MediaType: r.Type, TmdbID: r.Media.TmdbID}
	}
	summaries := api.ResolveTitles(ctx, client, api.NewTitleCache(), queries)

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTitle\tYear\tType\tTMDB ID\tRequest Status\tAvailability\tRequested")
	for i, r := range requests {
		title := summaries[i].Title
		if title == "" {
			title = fmt.Sprintf("TMDB %d", r.Media.TmdbID)
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
			r.ID, title, summaries[i].Year, mediaTypeLabel(r.Type), r.Media.TmdbID,
			requestStatusLabel(r.Status), mediaStatusLabel(r.Media.Status), r.CreatedAt)
	}
	tw.Flush()
}

// mediaTypeLabel mirrors internal/tui's own typeLabel (Movie/TV). Kept as a
// separate copy rather than shared: it's pure display formatting, not the
// TMDB-lookup logic api.ResolveTitles centralizes, and the two packages
// render different layouts (table vs. list) that happen to want the same
// two words.
func mediaTypeLabel(t models.MediaType) string {
	if t == models.MediaTV {
		return "TV"
	}
	return "Movie"
}

func requestStatusLabel(status int) string {
	switch status {
	case 1:
		return "Pending"
	case 2:
		return "Approved"
	case 3:
		return "Declined"
	case 4:
		return "Failed"
	case 5:
		return "Completed"
	default:
		return "Unknown"
	}
}

func mediaStatusLabel(status models.MediaStatus) string {
	switch status {
	case models.MediaStatusUnknown:
		return "Unknown"
	case models.MediaStatusPending:
		return "Pending"
	case models.MediaStatusProcessing:
		return "Processing"
	case models.MediaStatusPartiallyAvailable:
		return "Partially available"
	case models.MediaStatusAvailable:
		return "Available"
	case models.MediaStatusBlocklisted:
		return "Blocklisted"
	case models.MediaStatusDeleted:
		return "Deleted"
	default:
		return "Unknown"
	}
}
