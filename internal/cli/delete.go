package cli

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/emzbtw/reel/internal/models"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a request",
	Long: `Delete a request by its request ID (the ID column from "reel status",
not the TMDB ID column).

Per Seerr: removing a pending request works for anyone who made it;
removing a request in any other state requires the MANAGE_REQUESTS
permission. Always shows the request and asks for confirmation first —
there is no bypass flag in this pass.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, convErr := strconv.Atoi(args[0])
		if convErr != nil {
			return fmt.Errorf("invalid request id %q", args[0])
		}

		req, err := client.GetRequest(cmd.Context(), id)
		if err != nil {
			return err
		}

		printRequestsTable(cmd.OutOrStdout(), []models.MediaRequest{*req})
		fmt.Fprint(cmd.OutOrStdout(), "Delete this request? [y/N]: ")

		line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		line = strings.ToLower(strings.TrimSpace(line))
		if line != "y" && line != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
			return nil
		}

		if err := client.DeleteRequest(cmd.Context(), id); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Deleted request #%d.\n", id)
		return nil
	},
}
