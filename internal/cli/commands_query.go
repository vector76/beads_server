package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show client and server versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			data, err := c.Do("GET", "/api/v1/version", nil)
			if err != nil {
				return err
			}

			var serverResp struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(data, &serverResp); err != nil {
				return fmt.Errorf("parsing server response: %w", err)
			}

			combined := struct {
				Client string `json:"client"`
				Server string `json:"server"`
			}{Client: version, Server: serverResp.Version}
			b, err := json.Marshal(combined)
			if err != nil {
				return err
			}
			out, err := prettyJSON(b)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

func newListCmd() *cobra.Command {
	var all bool
	var ready bool
	var status string
	var priority string
	var beadType string
	var tag string
	var assignee string
	var page int
	var perPage int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List beads",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			params := url.Values{}
			if all {
				params.Set("all", "true")
			}
			if ready {
				params.Set("ready", "true")
			}
			if status != "" {
				params.Set("status", status)
			}
			if priority != "" {
				params.Set("priority", priority)
			}
			if beadType != "" {
				params.Set("type", beadType)
			}
			if tag != "" {
				params.Set("tag", tag)
			}
			if assignee != "" {
				params.Set("assignee", assignee)
			}
			if cmd.Flags().Changed("page") {
				params.Set("page", strconv.Itoa(page))
			}
			if cmd.Flags().Changed("per-page") {
				params.Set("per_page", strconv.Itoa(perPage))
			}

			path := "/api/v1/beads"
			if len(params) > 0 {
				path += "?" + params.Encode()
			}

			data, err := c.Do("GET", path, nil)
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

	cmd.Flags().BoolVar(&all, "all", false, "include all statuses")
	cmd.Flags().BoolVar(&ready, "ready", false, "only show unblocked beads")
	cmd.Flags().StringVar(&status, "status", "", "filter by status (comma-separated)")
	cmd.Flags().StringVar(&priority, "priority", "", "filter by priority")
	cmd.Flags().StringVar(&beadType, "type", "", "filter by type")
	cmd.Flags().StringVar(&tag, "tag", "", "filter by tag (comma-separated)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "filter by assignee")
	cmd.Flags().IntVar(&page, "page", 1, "page number")
	cmd.Flags().IntVar(&perPage, "per-page", 100, "results per page")

	return cmd
}

func newSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search beads",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			params := url.Values{}
			params.Set("q", args[0])

			data, err := c.Do("GET", "/api/v1/search?"+params.Encode(), nil)
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

func newClaimCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "claim <id>",
		Short: "Claim a bead",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			user := getUser()

			body := map[string]any{
				"user": user,
			}

			data, err := c.Do("POST", "/api/v1/beads/"+args[0]+"/claim", body)
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

func newMineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mine",
		Short: "List beads assigned to me that are in progress",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			user := getUser()

			params := url.Values{}
			params.Set("assignee", user)
			params.Set("status", "in_progress")

			data, err := c.Do("GET", "/api/v1/beads?"+params.Encode(), nil)
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

func newCommentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "comment <id> <text>",
		Short: "Add a comment to a bead",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			user := getUser()

			body := map[string]any{
				"author": user,
				"text":   args[1],
			}

			data, err := c.Do("POST", "/api/v1/beads/"+args[0]+"/comments", body)
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

func newLinkCmd() *cobra.Command {
	var blockedBy string

	cmd := &cobra.Command{
		Use:   "link <id>",
		Short: "Add a dependency to a bead",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if blockedBy == "" {
				return fmt.Errorf("--blocked-by is required")
			}

			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			body := map[string]any{
				"blocked_by": blockedBy,
			}

			data, err := c.Do("POST", "/api/v1/beads/"+args[0]+"/link", body)
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

	cmd.Flags().StringVar(&blockedBy, "blocked-by", "", "ID of the blocking bead")

	return cmd
}

func newUnlinkCmd() *cobra.Command {
	var blockedBy string

	cmd := &cobra.Command{
		Use:   "unlink <id>",
		Short: "Remove a dependency from a bead",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if blockedBy == "" {
				return fmt.Errorf("--blocked-by is required")
			}

			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			data, err := c.Do("DELETE", "/api/v1/beads/"+args[0]+"/link/"+blockedBy, nil)
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

	cmd.Flags().StringVar(&blockedBy, "blocked-by", "", "ID of the blocking bead to remove")

	return cmd
}

func newDepsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deps <id>",
		Short: "Show dependencies of a bead",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				return err
			}

			data, err := c.Do("GET", "/api/v1/beads/"+args[0]+"/deps", nil)
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
