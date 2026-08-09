package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var trendingCmd = &cobra.Command{
	Use:   "trending",
	Short: "Show trending movies and TV shows",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := client.DiscoverTrending(cmd.Context(), 0)
		if err != nil {
			return err
		}

		items := mediaOnly(results.Results)
		if len(items) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No trending results.")
			return nil
		}

		printPickables(cmd.OutOrStdout(), items)
		return nil
	},
}
