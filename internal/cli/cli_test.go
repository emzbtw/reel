package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// newTestServer starts an httptest.Server and points REEL_SEERR_URL/
// REEL_SEERR_API_KEY at it, so rootCmd's PersistentPreRunE builds a client
// against it on the next Execute().
func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("REEL_SEERR_URL", srv.URL)
	t.Setenv("REEL_SEERR_API_KEY", "test-key")
	return srv
}

// execute runs rootCmd with the given args and stdin, returning combined
// stdout/stderr output and any error from Execute().
func execute(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	resetFlags(rootCmd)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetIn(strings.NewReader(stdin))
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return out.String(), err
}

// resetFlags restores every flag on cmd and its subcommands to its default
// value. rootCmd's commands are package-level singletons reused across
// tests, and pflag doesn't reset a flag to its default just because it's
// absent from a later Parse call, so without this a flag set in one test
// would leak into the next.
func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, c := range cmd.Commands() {
		resetFlags(c)
	}
}
