package cli

import (
	"strings"
	"testing"
)

// noConfig blanks out every source config.Load reads, so a test exercises
// the paths that must work without a configured Seerr.
func noConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("REEL_SEERR_URL", "")
	t.Setenv("REEL_SEERR_API_KEY", "")
}

// TestRootCmd_NoArgsNonTTYPrintsHelp covers bare "reel" when there's no
// terminal to draw a TUI on. execute() wires rootCmd's out/in to a
// bytes.Buffer and a strings.Reader — neither an *os.File — so interactive()
// is false here, which is also why no test in this package ever starts
// bubbletea. Runs without config on purpose: printing help never needed a
// configured Seerr before the TUI became the default, and shouldn't now.
func TestRootCmd_NoArgsNonTTYPrintsHelp(t *testing.T) {
	noConfig(t)

	out, err := execute(t, "")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	for _, want := range []string{"Usage:", "status"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

// TestRootCmd_HelpOmitsHiddenTUI guards tuiCmd's Hidden flag: "reel tui"
// still works for anyone with it in muscle memory or a script, but bare
// "reel" is the one documented way in, so help (and the completions
// generated from it) shouldn't advertise a second name for it.
func TestRootCmd_HelpOmitsHiddenTUI(t *testing.T) {
	noConfig(t)

	out, err := execute(t, "")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if strings.Contains(out, "Launch the interactive TUI") {
		t.Errorf("help lists the hidden tui command: %s", out)
	}
}

// TestRootCmd_UnknownCommandErrors guards the decision to leave Args unset
// on rootCmd now that it has a RunE: cobra's default legacyArgs still
// rejects an unknown first argument on a root command with subcommands, so
// a typo must not silently open the TUI instead.
func TestRootCmd_UnknownCommandErrors(t *testing.T) {
	noConfig(t)

	if _, err := execute(t, "", "bogus"); err == nil {
		t.Error("execute() returned nil error for an unknown command")
	}
}

// TestCompletionCmd_RunsWithoutConfig guards against rootCmd's
// PersistentPreRunE requiring Seerr config for "completion" subcommands,
// which broke completion generation in the nix build sandbox (no config
// file, no REEL_SEERR_* env vars): Execute() used to return the "missing
// required configuration" error instead of the completion script, which is
// exactly what made installShellCompletion see empty output at build time.
//
// This only checks the returned error, not the captured output: cobra's
// completion subcommands bind their output writer once, the first time the
// "completion" command tree is built on a given *cobra.Command, and rootCmd
// here is the same package-level singleton every other test in this package
// reuses via execute(). So whichever test happens to trigger that first
// build "wins" the writer, and later SetOut calls on a fresh buffer (as
// execute() does per call) don't reach it — a pre-existing cobra quirk
// unrelated to the fix under test.
func TestCompletionCmd_RunsWithoutConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("REEL_SEERR_URL", "")
	t.Setenv("REEL_SEERR_API_KEY", "")

	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			if _, err := execute(t, "", "completion", shell); err != nil {
				t.Fatalf("execute() returned error: %v", err)
			}
		})
	}
}
