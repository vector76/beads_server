package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var beadType string
	var priority string
	var description string
	var tags []string

	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Create a new bead",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			body := map[string]any{
				"title": args[0],
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

	cmd.Flags().StringVar(&beadType, "type", "", "bead type (bug, feature, task, epic, chore)")
	cmd.Flags().StringVar(&priority, "priority", "", "priority (critical, high, medium, low, none)")
	cmd.Flags().StringVar(&description, "description", "", "bead description")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "comma-separated tags")

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
	cmd.Flags().StringVar(&status, "status", "", "status (open, in_progress, resolved, closed, wontfix, deleted)")
	cmd.Flags().StringVar(&priority, "priority", "", "priority (critical, high, medium, low, none)")
	cmd.Flags().StringVar(&beadType, "type", "", "bead type (bug, feature, task, epic, chore)")
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

