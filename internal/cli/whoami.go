package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Print current agent identity",
		RunE:  runWhoami,
	}
}

func runWhoami(cmd *cobra.Command, args []string) error {
	user := os.Getenv("BS_USER")
	if user == "" {
		user = "anonymous"
	}
	out, err := json.Marshal(map[string]string{"user": user})
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}
