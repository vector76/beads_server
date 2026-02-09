package cli

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/yourorg/beads_server/internal/server"
	"github.com/yourorg/beads_server/internal/store"
)

func newServeCmd() *cobra.Command {
	var port int
	var dataFile string
	var token string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the beads HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve token: flag > env
			if token == "" {
				token = os.Getenv("BS_TOKEN")
			}
			if token == "" {
				return fmt.Errorf("token is required (use --token or BS_TOKEN env var)")
			}

			// Resolve port: flag > env > default
			if !cmd.Flags().Changed("port") {
				if envPort := os.Getenv("BS_PORT"); envPort != "" {
					p, err := strconv.Atoi(envPort)
					if err != nil {
						return fmt.Errorf("invalid BS_PORT: %w", err)
					}
					port = p
				}
			}

			// Resolve data-file: flag > env > default
			if !cmd.Flags().Changed("data-file") {
				if envFile := os.Getenv("BS_DATA_FILE"); envFile != "" {
					dataFile = envFile
				}
			}

			s, err := store.Load(dataFile)
			if err != nil {
				return fmt.Errorf("loading data file: %w", err)
			}

			cfg := server.Config{
				Port:     port,
				DataFile: dataFile,
			}

			p := server.NewSingleStoreProvider(token, s)
			srv, err := server.New(cfg, p)
			if err != nil {
				return err
			}

			addr := srv.ListenAddr()
			fmt.Fprintf(cmd.OutOrStdout(), "listening on %s\n", addr)
			return http.ListenAndServe(addr, srv.Router)
		},
	}

	cmd.Flags().IntVar(&port, "port", 9999, "port to listen on")
	cmd.Flags().StringVar(&dataFile, "data-file", "beads.json", "path to data file")
	cmd.Flags().StringVar(&token, "token", "", "bearer token for authentication")

	return cmd
}
