// Package cli wires reel's cobra commands to internal/api and internal/config.
package cli

import (
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/emzbtw/reel/internal/api"
	"github.com/emzbtw/reel/internal/config"
	"github.com/emzbtw/reel/internal/tui"
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
	Long: `reel is a terminal companion for Seerr.

Run "reel" with no arguments to launch the interactive TUI. The
subcommands below cover the same ground non-interactively, for scripts
and one-off lookups.`,
	// Runtime errors (bad API key, 404, rate limit, ...) aren't usage
	// mistakes; main.go formats and prints them itself.
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "completion" || (cmd.Parent() != nil && cmd.Parent().Name() == "completion") {
			return nil
		}
		// Bare "reel" on a non-TTY only prints help — same as before the
		// TUI became the default, and that never needed a configured Seerr.
		// !HasParent() identifies the root command; comparing against
		// rootCmd itself here would be an initialization cycle.
		if !cmd.HasParent() && !interactive(cmd) {
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
	// Bare "reel" launches the TUI: it is the primary interface, and the
	// subcommands are the scripted path. When there's no terminal to draw
	// on (piped, redirected, CI) fall back to the help text reel printed
	// here before the TUI became the default.
	//
	// Args is deliberately left unset: cobra's default legacyArgs already
	// rejects an unknown first argument on a root command that has
	// subcommands, so "reel bogus" still errors (with suggestions) rather
	// than opening the TUI.
	RunE: func(cmd *cobra.Command, args []string) error {
		if !interactive(cmd) {
			return cmd.Help()
		}
		return tui.Run(cmd.Context(), client)
	},
}

// interactive reports whether cmd is attached to a real terminal on both
// stdin and stdout — bubbletea needs both. Anything that isn't an *os.File
// (a test's bytes.Buffer, a pipe) is not a terminal.
func interactive(cmd *cobra.Command) bool {
	return isTerminalFile(cmd.OutOrStdout()) && isTerminalFile(cmd.InOrStdin())
}

func isTerminalFile(v any) bool {
	f, ok := v.(*os.File)
	return ok && isatty.IsTerminal(f.Fd())
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
