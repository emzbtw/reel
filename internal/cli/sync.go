package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/emzbtw/reel/internal/obsidian"
)

var (
	syncDryRun bool
	syncYes    bool
	syncRetry  bool
	syncQuiet  bool
)

var syncCmd = &cobra.Command{
	Use:   "sync [note...]",
	Short: "Sync an Obsidian note's checklist with Seerr",
	Long: `Sync an Obsidian note's checklist with Seerr.

Unchecked task lines are searched, requested, and their checkbox updated to
reflect Seerr's status:

  [ ]  not yet synced       [🎬] requested
  [↓]  downloading          [✓] available
  [✗]  declined or removed

Lines checked off by hand ("[x]"), lines carrying a marker reel does not own
("[-]", "[/]"), and lines marked %%reel:ignore%% are left alone.

Titles that don't resolve to exactly one result are reported, never guessed
at — add a year to disambiguate, e.g. "- [ ] Alien (1979)".

With no arguments, syncs the notes listed as obsidian_notes in the config
file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		notes := args
		if len(notes) == 0 {
			notes = cfg.ObsidianNotes
		}
		if len(notes) == 0 {
			return fmt.Errorf("no notes to sync: pass a path, or set obsidian_notes in the config file")
		}

		printedAny := false
		for _, note := range notes {
			header := ""
			if len(notes) > 1 {
				header = note
			}
			printed, err := syncNote(cmd, note, header, printedAny)
			if err != nil {
				return err
			}
			printedAny = printedAny || printed
		}
		return nil
	},
}

// syncNote syncs one note and reports whether it printed anything, so a
// multi-note run can decide whether the next note's header needs a blank
// line above it. header is printed only if the note turns out not to be
// suppressed by --quiet; separator adds a blank line above it when an
// earlier note already printed something.
func syncNote(cmd *cobra.Command, note, header string, separator bool) (bool, error) {
	var retried int
	if syncRetry {
		n, err := forgetFailed(note)
		if err != nil {
			return false, err
		}
		retried = n
	}

	plan, err := obsidian.BuildPlan(cmd.Context(), client, note)
	if err != nil {
		return false, err
	}

	suppress := syncQuiet && !hasChanges(plan)

	if !suppress {
		if header != "" {
			if separator {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			fmt.Fprintf(cmd.OutOrStdout(), "== %s\n", header)
		}
		if retried > 0 && !jsonOutput {
			fmt.Fprintf(cmd.OutOrStdout(), "Retrying %d previously failed line(s).\n", retried)
		}
	}

	if jsonOutput {
		if syncDryRun {
			if suppress {
				return false, nil
			}
			return true, writeJSON(cmd.OutOrStdout(), plan.Items)
		}
	} else if !suppress {
		printPlan(cmd.OutOrStdout(), plan)
	}

	if syncDryRun {
		if !jsonOutput && !suppress {
			fmt.Fprintln(cmd.OutOrStdout(), "\nDry run: nothing was requested or written.")
		}
		return !suppress, nil
	}

	// Requesting reaches outside this machine and is not trivially
	// reversible, so it is confirmed by default. Marker-only updates are
	// local and need no prompt.
	if reqs := plan.Requests(); len(reqs) > 0 && !syncYes {
		ok, err := confirm(cmd, fmt.Sprintf("Submit %d request(s)?", len(reqs)))
		if err != nil {
			return !suppress, err
		}
		if !ok {
			fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
			return !suppress, nil
		}
	}

	res, err := obsidian.Apply(cmd.Context(), client, plan)
	if err != nil {
		return !suppress, err
	}

	if suppress {
		return false, nil
	}

	if jsonOutput {
		return true, writeJSON(cmd.OutOrStdout(), res)
	}
	printResult(cmd.OutOrStdout(), res)
	return true, nil
}

// hasChanges reports whether a plan has anything worth surfacing under
// --quiet: a request, a marker write, or an item that needs the user's
// attention. Skipped lines (opted out, already checked off, etc.) don't
// count — they're routine, not something to catch up on later.
func hasChanges(plan *obsidian.Plan) bool {
	if len(plan.Requests()) > 0 || len(plan.Ambiguous()) > 0 {
		return true
	}
	for _, it := range plan.Items {
		if it.Action == obsidian.ActionMarker {
			return true
		}
	}
	return false
}

// forgetFailed drops the bindings for lines currently showing "[✗]" so the
// next plan treats them as new. This is the escape hatch for a request that
// was declined and has since been sorted out: reel never re-requests a bound
// line, so the binding has to go for a retry to happen.
func forgetFailed(notePath string) (int, error) {
	note, err := obsidian.ParseNote(notePath)
	if err != nil {
		return 0, err
	}
	store, err := obsidian.LoadStore(notePath)
	if err != nil {
		return 0, err
	}

	var n int
	for _, task := range note.Tasks {
		if task.Marker != obsidian.Failed {
			continue
		}
		if _, ok := store.Get(notePath, task.Title); !ok {
			continue
		}
		store.Delete(notePath, task.Title)
		n++
	}
	if n == 0 {
		return 0, nil
	}
	return n, store.Save()
}

func printPlan(w io.Writer, plan *obsidian.Plan) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "Title\tPlan\tDetail")
	for _, it := range plan.Items {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", displayTitle(it), planLabel(it), planDetail(it))
	}
	tw.Flush()

	for _, it := range plan.Ambiguous() {
		if len(it.Candidates) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%q matched several results:\n", displayTitle(it))
		for _, c := range it.Candidates {
			fmt.Fprintf(w, "  %s (%s) — tmdb %d, %s\n", c.Title, orDash(c.Year), c.TmdbID, typeLabel(c.MediaType))
		}
	}
}

func displayTitle(it obsidian.Item) string {
	if it.Year != "" {
		return fmt.Sprintf("%s (%s)", it.Title, it.Year)
	}
	if it.Title == "" {
		return strings.TrimSpace(it.Task.Raw)
	}
	return it.Title
}

func planLabel(it obsidian.Item) string {
	switch it.Action {
	case obsidian.ActionRequest:
		return "request"
	case obsidian.ActionMarker:
		return fmt.Sprintf("mark %s", strings.ToLower(it.NewMarker.String()))
	case obsidian.ActionAmbiguous:
		return "needs attention"
	case obsidian.ActionSkip:
		return "skip"
	default:
		return "up to date"
	}
}

// planDetail names what a line resolved to, so the confirmation prompt is
// approving a specific piece of media rather than a bare count. For lines
// that won't be acted on it carries the reason instead.
func planDetail(it obsidian.Item) string {
	switch it.Action {
	case obsidian.ActionAmbiguous, obsidian.ActionSkip:
		return it.Reason
	}
	if m := it.Matched; m != nil {
		return fmt.Sprintf("%s (%s) — tmdb %d, %s", m.Title, orDash(m.Year), m.TmdbID, typeLabel(m.MediaType))
	}
	// An already-bound line was never searched, so only the stored target is
	// known.
	if it.Binding.TmdbID != 0 {
		return fmt.Sprintf("tmdb %d", it.Binding.TmdbID)
	}
	return ""
}

func printResult(w io.Writer, res obsidian.Result) {
	fmt.Fprintf(w, "\nRequested %d, updated %d marker(s).\n", len(res.Requested), len(res.Written.Applied))
	if len(res.Unwritten) > 0 {
		fmt.Fprintf(w, "%d marker(s) could not be written (the note changed while syncing); "+
			"they will be re-applied on the next sync.\n", len(res.Unwritten))
	}
}

func confirm(cmd *cobra.Command, prompt string) (bool, error) {
	fmt.Fprintf(cmd.OutOrStdout(), "\n%s [y/N] ", prompt)
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && line == "" {
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func init() {
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "show what would happen without requesting or writing anything")
	syncCmd.Flags().BoolVar(&syncYes, "yes", false, "skip the confirmation prompt before submitting requests")
	syncCmd.Flags().BoolVar(&syncRetry, "retry", false, "forget bindings for lines marked [✗] so they are requested again")
	syncCmd.Flags().BoolVar(&syncQuiet, "quiet", false, "suppress output when nothing was requested or written and nothing is ambiguous")
}
