package cli

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/yourorg/beads_server/internal/project"
	"github.com/yourorg/beads_server/internal/server"
	"github.com/yourorg/beads_server/internal/store"
)

func newServeCmd() *cobra.Command {
	var port int
	var dataFile string
	var token string
	var projectsFile string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the beads HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve projects file: flag > env
			if projectsFile == "" {
				projectsFile = os.Getenv("BS_PROJECTS_FILE")
			}

			// Resolve token: flag > env
			if token == "" {
				token = os.Getenv("BS_TOKEN")
			}

			// Validate mutual exclusivity
			if projectsFile != "" && token != "" {
				return fmt.Errorf("--projects and --token are mutually exclusive")
			}
			if projectsFile == "" && token == "" {
				return fmt.Errorf("either --projects or --token is required")
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

			var provider server.StoreProvider

			if projectsFile != "" {
				// Multi-project mode
				entries, err := project.LoadProjectsFile(projectsFile)
				if err != nil {
					return err
				}

				providerEntries := make([]server.ProviderEntry, len(entries))
				for i, e := range entries {
					s, err := store.Load(e.DataFile)
					if err != nil {
						return fmt.Errorf("loading data file for project %q: %w", e.Name, err)
					}
					providerEntries[i] = server.ProviderEntry{
						Name:  e.Name,
						Token: e.Token,
						Store: s,
					}
				}

				provider = server.NewMultiStoreProvider(providerEntries)
			} else {
				// Single-project mode
				if !cmd.Flags().Changed("data-file") {
					if envFile := os.Getenv("BS_DATA_FILE"); envFile != "" {
						dataFile = envFile
					}
				}

				s, err := store.Load(dataFile)
				if err != nil {
					return fmt.Errorf("loading data file: %w", err)
				}

				provider = server.NewSingleStoreProvider(token, s)
			}

			cfg := server.Config{
				Port: port,
			}

			srv, err := server.New(cfg, provider)
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
	cmd.Flags().StringVar(&projectsFile, "projects", "", "path to projects config file (multi-project mode)")

	return cmd
}
