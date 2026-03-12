package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var errTimeout = errors.New("timed out")

func newWaitReadyCmd() *cobra.Command {
	var timeout int
	var tags []string
	var assignee string
	var priority string
	var beadType string

	cmd := &cobra.Command{
		Use:           "wait-ready",
		Short:         "Wait until a ready bead exists",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewClientFromEnv()
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "error:", err)
				return err
			}

			var ctx context.Context
			var cancel context.CancelFunc
			if timeout > 0 {
				ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()

			signals, errChan := c.StreamSSE(ctx)

			// Build the query path once; flags are immutable after parsing.
			params := url.Values{}
			params.Set("ready", "true")
			params.Set("per_page", "1")
			if len(tags) > 0 {
				params.Set("tag", strings.Join(tags, ","))
			}
			if assignee != "" {
				params.Set("assignee", assignee)
			}
			if priority != "" {
				params.Set("priority", priority)
			}
			if beadType != "" {
				params.Set("type", beadType)
			}
			path := "/api/v1/beads?" + params.Encode()

			checkReady := func() (bool, error) {
				data, err := c.Do("GET", path, nil)
				if err != nil {
					return false, err
				}
				var result struct {
					Total int `json:"total"`
				}
				if err := json.Unmarshal(data, &result); err != nil {
					return false, fmt.Errorf("parsing response: %w", err)
				}
				return result.Total >= 1, nil
			}

			ready, err := checkReady()
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "error:", err)
				return err
			}
			if ready {
				return nil
			}

			for {
				select {
				case _, ok := <-signals:
					if !ok {
						// signals closed; errChan will deliver the outcome — stop selecting on signals.
						signals = nil
						continue
					}
					ready, err := checkReady()
					if err != nil {
						fmt.Fprintln(cmd.ErrOrStderr(), "error:", err)
						return err
					}
					if ready {
						return nil
					}
				case err, ok := <-errChan:
					if ok && err != nil {
						fmt.Fprintln(cmd.ErrOrStderr(), "error:", err)
						return err
					}
					// nil error or channel closed: context was cancelled (deadline or explicit).
					return errTimeout
				case <-ctx.Done():
					return errTimeout
				}
			}
		},
	}

	cmd.Flags().IntVar(&timeout, "timeout", 0, "seconds to wait (0 = indefinite)")
	_ = cmd.MarkFlagRequired("timeout")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "filter by tag (repeatable)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "filter by assignee")
	cmd.Flags().StringVar(&priority, "priority", "", "filter by priority")
	cmd.Flags().StringVar(&beadType, "type", "", "filter by bead type")

	return cmd
}
