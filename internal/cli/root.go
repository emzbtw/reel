// Package cli wires reel's cobra commands to internal/api and internal/config.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/emzbtw/reel/internal/api"
	"github.com/emzbtw/reel/internal/config"
)

// client is built once in rootCmd's PersistentPreRunE and used by every
// subcommand's RunE. cfg is kept alongside it for the settings that aren't
// only about reaching Seerr, such as the notes "reel sync" defaults to.
var (
	client *api.Client
	cfg    *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "reel",
	Short: "A terminal companion for Seerr",
	// Runtime errors (bad API key, 404, rate limit, ...) aren't usage
	// mistakes; main.go formats and prints them itself.
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "completion" || (cmd.Parent() != nil && cmd.Parent().Name() == "completion") {
			return nil
		}
		loaded, err := config.Load()
		if err != nil {
			return err
		}
		cfg = loaded
		client = api.NewClient(cfg)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCmd, requestCmd, statusCmd, trendingCmd, browseCmd, deleteCmd, syncCmd, tuiCmd)

	// --json is shared across the read-only, result-listing commands, plus
	// request (which only applies it to the final created-request output;
	// the picker itself always stays interactive text).
	for _, cmd := range []*cobra.Command{searchCmd, trendingCmd, browseMoviesCmd, browseTVCmd, statusCmd, requestCmd, syncCmd} {
		addJSONFlag(cmd)
	}
}

// Execute runs the root command. Any returned error should be formatted
// with FormatError before being shown to the user.
func Execute() error {
	return rootCmd.Execute()
}
