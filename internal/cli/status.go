package cli

import (
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

		printRequestsTable(cmd.OutOrStdout(), list.Results)
		return nil
	},
}

// printRequestsTable renders requests using the same columns wherever a
// request needs to be shown (status's listing, delete's confirmation
// prompt). Seerr's GET /request doesn't join in a title (MediaInfo only
// carries tmdbId/tvdbId), so rows are identified by TMDB ID until title
// enrichment is added.
func printRequestsTable(w io.Writer, requests []models.MediaRequest) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTMDB ID\tRequest Status\tAvailability\tRequested")
	for _, r := range requests {
		fmt.Fprintf(tw, "%d\t%d\t%s\t%s\t%s\n",
			r.ID, r.Media.TmdbID, requestStatusLabel(r.Status), mediaStatusLabel(r.Media.Status), r.CreatedAt)
	}
	tw.Flush()
}

func requestStatusLabel(status int) string {
	switch status {
	case 1:
		return "Pending"
	case 2:
		return "Approved"
	case 3:
		return "Declined"
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
	case models.MediaStatusDeleted:
		return "Deleted"
	default:
		return "Unknown"
	}
}
