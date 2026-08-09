package cli

import (
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
		return renderResults(cmd, results.Results, items, "No trending results.")
	},
}
