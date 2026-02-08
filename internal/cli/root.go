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
		Version: version,
	}

	root.AddCommand(newServeCmd())
	root.AddCommand(newWhoamiCmd())
	root.AddCommand(newAddCmd())
	root.AddCommand(newShowCmd())
	root.AddCommand(newEditCmd())
	root.AddCommand(newStatusCmd("close", "closed"))
	root.AddCommand(newStatusCmd("resolve", "resolved"))
	root.AddCommand(newStatusCmd("reopen", "open"))
	root.AddCommand(newDeleteCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newSearchCmd())
	root.AddCommand(newClaimCmd())
	root.AddCommand(newMineCmd())
	root.AddCommand(newCommentCmd())
	root.AddCommand(newLinkCmd())
	root.AddCommand(newUnlinkCmd())
	root.AddCommand(newDepsCmd())

	return root
}
