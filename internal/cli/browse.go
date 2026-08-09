package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var browseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Browse popular movies and TV shows",
}

var browseMoviesPage int

var browseMoviesCmd = &cobra.Command{
	Use:   "movies",
	Short: "Browse popular movies",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if browseMoviesPage < 1 {
			return fmt.Errorf("--page must be >= 1")
		}

		results, err := client.DiscoverMovies(cmd.Context(), browseMoviesPage)
		if err != nil {
			return err
		}

		items := moviePickables(results.Results)
		if len(items) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No movies found.")
			return nil
		}

		printPickables(cmd.OutOrStdout(), items)
		fmt.Fprintf(cmd.OutOrStdout(), "Page %d of %d\n", results.Page, results.TotalPages)
		return nil
	},
}

var browseTVPage int

var browseTVCmd = &cobra.Command{
	Use:   "tv",
	Short: "Browse popular TV shows",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if browseTVPage < 1 {
			return fmt.Errorf("--page must be >= 1")
		}

		results, err := client.DiscoverTV(cmd.Context(), browseTVPage)
		if err != nil {
			return err
		}

		items := tvPickables(results.Results)
		if len(items) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No TV shows found.")
			return nil
		}

		printPickables(cmd.OutOrStdout(), items)
		fmt.Fprintf(cmd.OutOrStdout(), "Page %d of %d\n", results.Page, results.TotalPages)
		return nil
	},
}

func init() {
	browseMoviesCmd.Flags().IntVar(&browseMoviesPage, "page", 1, "page number (1-based)")
	browseTVCmd.Flags().IntVar(&browseTVPage, "page", 1, "page number (1-based)")
	browseCmd.AddCommand(browseMoviesCmd, browseTVCmd)
}
