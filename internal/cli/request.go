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

var requestCmd = &cobra.Command{
	Use:   "request <query>",
	Short: "Search and submit a request for a movie or TV show",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

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
	},
}
