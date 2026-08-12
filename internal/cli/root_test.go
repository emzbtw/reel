package cli

import "testing"

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
