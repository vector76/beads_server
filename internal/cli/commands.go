package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newRedirectCmd returns a hidden command that always exits 1 and prints a
// redirect message plus the full "bs link --help" output to stderr.
func newRedirectCmd(name string) *cobra.Command {
	return &cobra.Command{
		Use:           name,
		Short:         "Unknown command — use 'bs link' to manage dependencies",
		Hidden:        true,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			stderr := cmd.ErrOrStderr()
			fmt.Fprintf(stderr, "Error: unknown command %q for \"bs\" — use \"bs link\" instead\n", name)
			if linkCmd, _, _ := cmd.Root().Find([]string{"link"}); linkCmd != nil {
				linkCmd.SetOut(stderr)
				_ = linkCmd.Help()
				linkCmd.SetOut(nil)
			}
			return fmt.Errorf("unknown command %q for \"bs\"", name)
		},
	}
}

func newAddCmd() *cobra.Command {
	var title string
	var beadType string
	var priority string
	var description string
	var tags []string
	var parentID string
	var status string

	cmd := &cobra.Command{
		Use:   "add [<title>]",
		Short: "Create a new bead",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hasPositional := len(args) == 1
			hasFlag := cmd.Flags().Changed("title")

			if hasPositional && hasFlag {
				return fmt.Errorf("title specified twice: use either the positional argument or --title, not both")
			}
			if !hasPositional && !hasFlag {
				return fmt.Errorf("title is required: provide it as a positional argument or via --title")
			}

			if hasPositional {
				title = args[0]
			}

			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			body := map[string]any{
				"title": title,
			}
			if description != "" {
				body["description"] = description
			}
			if beadType != "" {
				body["type"] = beadType
			}
			if priority != "" {
				body["priority"] = priority
			}
			if len(tags) > 0 {
				body["tags"] = tags
			}
			if parentID != "" {
				body["parent_id"] = parentID
			}
			if status != "" {
				if status != "open" && status != "not_ready" {
					return fmt.Errorf("--status must be 'open' or 'not_ready'")
				}
				body["status"] = status
			}

			data, err := c.Do("POST", "/api/v1/beads", body)
			if err != nil {
				return err
			}

			out, err := prettyJSON(data)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "bead title (alternative to positional argument)")
	cmd.Flags().StringVar(&beadType, "type", "", "bead type (bug, feature, task, chore)")
	cmd.Flags().StringVar(&priority, "priority", "", "priority (critical, high, medium, low, none)")
	cmd.Flags().StringVar(&description, "description", "", "bead description")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "comma-separated tags")
	cmd.Flags().StringVar(&parentID, "parent", "", "parent epic ID (creates a child bead)")
	cmd.Flags().StringVar(&status, "status", "", "initial status (open or not_ready; default: open)")

	return cmd
}

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a bead",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			data, err := c.Do("GET", "/api/v1/beads/"+args[0], nil)
			if err != nil {
				return err
			}

			out, err := prettyJSON(data)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

func newEditCmd() *cobra.Command {
	var title string
	var status string
	var priority string
	var beadType string
	var description string
	var assignee string
	var addTags []string
	var removeTags []string

	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a bead",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			body := map[string]any{}
			if cmd.Flags().Changed("title") {
				body["title"] = title
			}
			if cmd.Flags().Changed("status") {
				body["status"] = status
			}
			if cmd.Flags().Changed("priority") {
				body["priority"] = priority
			}
			if cmd.Flags().Changed("type") {
				body["type"] = beadType
			}
			if cmd.Flags().Changed("description") {
				body["description"] = description
			}
			if cmd.Flags().Changed("assignee") {
				body["assignee"] = assignee
			}
			if len(addTags) > 0 {
				body["add_tags"] = addTags
			}
			if len(removeTags) > 0 {
				body["remove_tags"] = removeTags
			}

			if len(body) == 0 {
				return fmt.Errorf("no fields to update")
			}

			data, err := c.Do("PATCH", "/api/v1/beads/"+args[0], body)
			if err != nil {
				return err
			}

			out, err := prettyJSON(data)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "bead title")
	cmd.Flags().StringVar(&status, "status", "", "status (open, not_ready, in_progress, closed, deleted)")
	cmd.Flags().StringVar(&priority, "priority", "", "priority (critical, high, medium, low, none)")
	cmd.Flags().StringVar(&beadType, "type", "", "bead type (bug, feature, task, chore)")
	cmd.Flags().StringVar(&description, "description", "", "bead description")
	cmd.Flags().StringVar(&assignee, "assignee", "", "assignee")
	cmd.Flags().StringSliceVar(&addTags, "add-tag", nil, "add a tag")
	cmd.Flags().StringSliceVar(&removeTags, "remove-tag", nil, "remove a tag")

	return cmd
}

func newStatusCmd(name string, targetStatus string) *cobra.Command {
	return &cobra.Command{
		Use:   name + " <id>",
		Short: fmt.Sprintf("Set bead status to %s", targetStatus),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			body := map[string]any{
				"status": targetStatus,
			}

			data, err := c.Do("PATCH", "/api/v1/beads/"+args[0], body)
			if err != nil {
				return err
			}

			out, err := prettyJSON(data)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

func newCleanCmd() *cobra.Command {
	var days float64
	var hours float64

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Purge old closed/deleted beads",
		RunE: func(cmd *cobra.Command, args []string) error {
			daysChanged := cmd.Flags().Changed("days")
			hoursChanged := cmd.Flags().Changed("hours")

			if daysChanged && hoursChanged {
				return fmt.Errorf("cannot specify both --days and --hours")
			}

			value := days
			if hoursChanged {
				value = hours / 24.0
			}

			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			body := map[string]any{
				"days": value,
			}

			data, err := c.Do("POST", "/api/v1/clean", body)
			if err != nil {
				return err
			}

			out, err := prettyJSON(data)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}

	cmd.Flags().Float64Var(&days, "days", 5, "remove beads last updated more than N days ago; accepts decimals (0 = all)")
	cmd.Flags().Float64Var(&hours, "hours", 0, "remove beads last updated more than N hours ago; accepts decimals (0 = all)")

	return cmd
}

func newMoveCmd() *cobra.Command {
	var into string
	var out bool

	cmd := &cobra.Command{
		Use:   "move <id>",
		Short: "Move a bead into or out of an epic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			intoChanged := cmd.Flags().Changed("into")
			outChanged := cmd.Flags().Changed("out")

			if !intoChanged && !outChanged {
				return fmt.Errorf("specify --into <epic-id> or --out")
			}
			if intoChanged && outChanged {
				return fmt.Errorf("cannot specify both --into and --out")
			}

			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			body := map[string]any{}
			if intoChanged {
				body["parent_id"] = into
			} else {
				body["parent_id"] = ""
			}

			data, err := c.Do("PATCH", "/api/v1/beads/"+args[0], body)
			if err != nil {
				return err
			}

			pretty, err := prettyJSON(data)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), pretty)
			return nil
		},
	}

	cmd.Flags().StringVar(&into, "into", "", "target epic ID to move the bead into")
	cmd.Flags().BoolVar(&out, "out", false, "detach the bead from its parent epic")

	return cmd
}

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a bead",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			data, err := c.Do("DELETE", "/api/v1/beads/"+args[0], nil)
			if err != nil {
				return err
			}

			out, err := prettyJSON(data)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

