package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search movies and TV shows",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		results, err := client.Search(cmd.Context(), query, 0)
		if err != nil {
			return err
		}

		items := mediaOnly(results.Results)
		return renderResults(cmd, results.Results, items, fmt.Sprintf("No movies or TV shows found for %q.", query))
	},
}
