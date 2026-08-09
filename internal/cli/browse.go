package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var browseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Browse popular movies and TV shows",
}

var browseMoviesCmd = &cobra.Command{
	Use:   "movies",
	Short: "Browse popular movies",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := client.DiscoverMovies(cmd.Context(), 1)
		if err != nil {
			return err
		}

		items := moviePickables(results.Results)
		if len(items) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No movies found.")
			return nil
		}

		printPickables(cmd.OutOrStdout(), items)
		return nil
	},
}

var browseTVCmd = &cobra.Command{
	Use:   "tv",
	Short: "Browse popular TV shows",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := client.DiscoverTV(cmd.Context(), 1)
		if err != nil {
			return err
		}

		items := tvPickables(results.Results)
		if len(items) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No TV shows found.")
			return nil
		}

		printPickables(cmd.OutOrStdout(), items)
		return nil
	},
}

func init() {
	browseCmd.AddCommand(browseMoviesCmd, browseTVCmd)
}
