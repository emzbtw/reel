package cli

import (
	"github.com/spf13/cobra"

	"github.com/emzbtw/reel/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive TUI",
	// Bare "reel" is the documented way in. This stays registered so
	// existing aliases and scripts calling "reel tui" keep working, but
	// hidden so it doesn't show up in --help or the generated completions
	// as a second name for the default behavior.
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run(cmd.Context(), client)
	},
}
