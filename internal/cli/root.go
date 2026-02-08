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

	return root
}
