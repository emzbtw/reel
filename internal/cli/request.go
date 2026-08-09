package cli

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/emzbtw/reel/internal/api"
	"github.com/emzbtw/reel/internal/models"
)

var (
	requestSelectID int
	requestType     string
)

var requestCmd = &cobra.Command{
	Use:   "request [query]",
	Short: "Search and submit a request for a movie or TV show",
	Long: `Search and submit a request for a movie or TV show.

With a query argument, this searches and shows an interactive picker.

With --select and --type, it skips search and the picker entirely and
creates the request directly by TMDB ID — the scriptable path. Combined
with --json, "reel request --select <id> --type movie --json" is fully
non-interactive.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if requestSelectID != 0 || requestType != "" {
			return runSelectedRequest(cmd, args)
		}

		if len(args) != 1 {
			return fmt.Errorf("request requires a <query> argument (or --select/--type for non-interactive use)")
		}
		return runInteractiveRequest(cmd, args[0])
	},
}

func runSelectedRequest(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("a <query> argument cannot be combined with --select")
	}
	if requestSelectID <= 0 {
		return fmt.Errorf("--select must be a positive TMDB ID")
	}
	if requestType == "" {
		return fmt.Errorf("--type is required when --select is set (movie or tv)")
	}

	mediaType, err := parseMediaType(requestType)
	if err != nil {
		return err
	}

	input := api.CreateRequestInput{MediaType: mediaType, MediaID: requestSelectID}
	if mediaType == models.MediaTV {
		input.AllSeasons = true
	}

	req, err := client.CreateRequest(cmd.Context(), input)
	if err != nil {
		return err
	}

	if jsonOutput {
		return writeJSON(cmd.OutOrStdout(), req)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Requested tmdbId %d (request #%d).\n", requestSelectID, req.ID)
	return nil
}

func runInteractiveRequest(cmd *cobra.Command, query string) error {
	results, err := client.Search(cmd.Context(), query, 0)
	if err != nil {
		return err
	}

	items := mediaOnly(results.Results)
	if len(items) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No movies or TV shows found for %q.\n", query)
		return nil
	}

	printPickables(cmd.OutOrStdout(), items)
	fmt.Fprintf(cmd.OutOrStdout(), "Pick a result to request (1-%d, or q to cancel): ", len(items))

	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" || line == "q" || line == "quit" {
		fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
		return nil
	}

	n, convErr := strconv.Atoi(line)
	if convErr != nil || n < 1 || n > len(items) {
		return fmt.Errorf("invalid selection %q", line)
	}
	picked := items[n-1]

	input := api.CreateRequestInput{
		MediaType: picked.Type,
		MediaID:   picked.ID,
	}
	if picked.Type == models.MediaTV {
		input.AllSeasons = true
	}

	req, err := client.CreateRequest(cmd.Context(), input)
	if err != nil {
		return err
	}

	if jsonOutput {
		return writeJSON(cmd.OutOrStdout(), req)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Requested %q (request #%d).\n", picked.Title, req.ID)
	return nil
}

func parseMediaType(s string) (models.MediaType, error) {
	switch strings.ToLower(s) {
	case "movie":
		return models.MediaMovie, nil
	case "tv":
		return models.MediaTV, nil
	default:
		return "", fmt.Errorf("--type must be movie or tv, got %q", s)
	}
}

func init() {
	requestCmd.Flags().IntVar(&requestSelectID, "select", 0, "TMDB ID to request directly, skipping search and the picker (requires --type)")
	requestCmd.Flags().StringVar(&requestType, "type", "", "media type for --select: movie or tv")
}
