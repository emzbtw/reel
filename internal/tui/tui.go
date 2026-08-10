// Package tui implements reel's interactive terminal UI, built with
// bubbletea/bubbles/lipgloss: browsing movies and TV, viewing an item's
// detail, and submitting a request.
package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/emzbtw/reel/internal/api"
)

// Run starts the interactive TUI and blocks until the user quits.
func Run(ctx context.Context, client *api.Client) error {
	p := tea.NewProgram(newModel(ctx, client), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
