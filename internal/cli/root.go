package cli

import (
	"github.com/spf13/cobra"
)

var version = "0.1.0"

// NewRootCmd creates the root cobra command for the bs CLI.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "bs",
		Short:   "Beads server CLI",
		Long:    "Beads server CLI — a tool for managing beads (issues/tasks).\n\nClient commands require BS_TOKEN and optionally BS_URL (default http://localhost:9999).",
		Version: version,
	}

	root.CompletionOptions.DisableDefaultCmd = true
	root.AddGroup(
		&cobra.Group{ID: "server", Title: "Server Commands:"},
		&cobra.Group{ID: "client", Title: "Client Commands:"},
	)

	serveCmd := newServeCmd()
	serveCmd.GroupID = "server"
	root.AddCommand(serveCmd)

	for _, cmd := range []*cobra.Command{
		newWhoamiCmd(),
		newAddCmd(),
		newShowCmd(),
		newEditCmd(),
		newStatusCmd("close", "closed"),
		newStatusCmd("reopen", "open"),
		newMoveCmd(),
		newDeleteCmd(),
		newCleanCmd(),
		newListCmd(),
		newSearchCmd(),
		newClaimCmd(),
		newMineCmd(),
		newCommentCmd(),
		newLinkCmd(),
		newUnlinkCmd(),
		newDepsCmd(),
	} {
		cmd.GroupID = "client"
		root.AddCommand(cmd)
	}

	// Hidden aliases: "create" -> "add", "resolve" -> "close"
	createAlias := newAddCmd()
	createAlias.Use = "create <title>"
	createAlias.Hidden = true
	root.AddCommand(createAlias)

	resolveAlias := newStatusCmd("resolve", "closed")
	resolveAlias.Hidden = true
	root.AddCommand(resolveAlias)

	return root
}
