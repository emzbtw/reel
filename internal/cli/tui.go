package cli

import (
	"github.com/spf13/cobra"

	"github.com/emzbtw/reel/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive TUI",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run(cmd.Context(), client)
	},
}
