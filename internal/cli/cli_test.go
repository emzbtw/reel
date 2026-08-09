package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetIn(strings.NewReader(stdin))
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return out.String(), err
}
