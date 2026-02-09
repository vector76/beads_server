package project

import (
	"encoding/json"
	"fmt"
	"os"
)

// ProjectEntry defines a single project's configuration.
type ProjectEntry struct {
	Name     string `json:"name"`
	Token    string `json:"token"`
	DataFile string `json:"data_file"`
}

// projectsFile is the on-disk JSON format for the projects config.
type projectsFile struct {
	Projects []ProjectEntry `json:"projects"`
}

// LoadProjectsFile reads and validates a projects configuration file.
// Returns an error if the file cannot be read, parsed, or contains invalid entries.
func LoadProjectsFile(path string) ([]ProjectEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading projects file: %w", err)
	}

	var pf projectsFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("parsing projects file: %w", err)
	}

	if err := validate(pf.Projects); err != nil {
		return nil, err
	}

	return pf.Projects, nil
}

// validate checks that all project entries are well-formed and unique.
func validate(projects []ProjectEntry) error {
	names := make(map[string]bool)
	tokens := make(map[string]bool)

	for i, p := range projects {
		if p.Name == "" {
			return fmt.Errorf("project %d: name must not be empty", i)
		}
		if p.Token == "" {
			return fmt.Errorf("project %q: token must not be empty", p.Name)
		}
		if p.DataFile == "" {
			return fmt.Errorf("project %q: data_file must not be empty", p.Name)
		}
		if names[p.Name] {
			return fmt.Errorf("duplicate project name: %q", p.Name)
		}
		if tokens[p.Token] {
			return fmt.Errorf("duplicate token in project %q", p.Name)
		}
		names[p.Name] = true
		tokens[p.Token] = true
	}

	return nil
}
