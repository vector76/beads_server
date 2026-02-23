package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

var version = "dev"

func init() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok &&
			info.Main.Version != "" &&
			info.Main.Version != "(devel)" {
			version = strings.TrimPrefix(info.Main.Version, "v")
		}
	}
}

// NewRootCmd creates the root cobra command for the bs CLI.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "bs",
		Short: "Beads server CLI",
		Long:  "Beads server CLI — a tool for managing beads (issues/tasks).\n\nClient commands require BS_TOKEN and optionally BS_URL (default http://localhost:9999).",
		RunE: func(cmd *cobra.Command, args []string) error {
			showVersion, _ := cmd.Flags().GetBool("version")
			if !showVersion {
				return cmd.Help()
			}
			fmt.Fprintf(cmd.OutOrStdout(), "client: %s\n", version)
			serverVersion := "unavailable"
			serverURL := getenv("BS_URL")
			if serverURL == "" {
				serverURL = defaultURL
			}
			if resp, err := http.Get(serverURL + "/api/v1/version"); err == nil {
				defer resp.Body.Close()
				var v struct {
					Version string `json:"version"`
				}
				if json.NewDecoder(resp.Body).Decode(&v) == nil && v.Version != "" {
					serverVersion = v.Version
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "server: %s\n", serverVersion)
			return nil
		},
	}

	root.Flags().BoolP("version", "v", false, "show version information")
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

	root.AddCommand(newRedirectCmd("depend"))
	root.AddCommand(newRedirectCmd("block"))

	return root
}
