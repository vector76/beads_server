package project

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projects.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestLoadProjectsFile_Valid(t *testing.T) {
	path := writeFile(t, `{
		"projects": [
			{"name": "webapp", "token": "tok-abc", "data_file": "webapp.json"},
			{"name": "backend", "token": "tok-def", "data_file": "backend.json"}
		]
	}`)

	entries, err := LoadProjectsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "webapp" {
		t.Errorf("expected name=webapp, got %q", entries[0].Name)
	}
	if entries[1].Token != "tok-def" {
		t.Errorf("expected token=tok-def, got %q", entries[1].Token)
	}
}

func TestLoadProjectsFile_EmptyProjects(t *testing.T) {
	path := writeFile(t, `{"projects": []}`)

	entries, err := LoadProjectsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestLoadProjectsFile_FileNotFound(t *testing.T) {
	_, err := LoadProjectsFile("/nonexistent/projects.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadProjectsFile_InvalidJSON(t *testing.T) {
	path := writeFile(t, `{not valid json}`)

	_, err := LoadProjectsFile(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadProjectsFile_EmptyName(t *testing.T) {
	path := writeFile(t, `{
		"projects": [
			{"name": "", "token": "tok-abc", "data_file": "data.json"}
		]
	}`)

	_, err := LoadProjectsFile(path)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestLoadProjectsFile_EmptyToken(t *testing.T) {
	path := writeFile(t, `{
		"projects": [
			{"name": "webapp", "token": "", "data_file": "data.json"}
		]
	}`)

	_, err := LoadProjectsFile(path)
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestLoadProjectsFile_EmptyDataFile(t *testing.T) {
	path := writeFile(t, `{
		"projects": [
			{"name": "webapp", "token": "tok-abc", "data_file": ""}
		]
	}`)

	_, err := LoadProjectsFile(path)
	if err == nil {
		t.Fatal("expected error for empty data_file")
	}
}

func TestLoadProjectsFile_DuplicateNames(t *testing.T) {
	path := writeFile(t, `{
		"projects": [
			{"name": "webapp", "token": "tok-abc", "data_file": "a.json"},
			{"name": "webapp", "token": "tok-def", "data_file": "b.json"}
		]
	}`)

	_, err := LoadProjectsFile(path)
	if err == nil {
		t.Fatal("expected error for duplicate names")
	}
}

func TestLoadProjectsFile_DuplicateTokens(t *testing.T) {
	path := writeFile(t, `{
		"projects": [
			{"name": "webapp", "token": "tok-abc", "data_file": "a.json"},
			{"name": "backend", "token": "tok-abc", "data_file": "b.json"}
		]
	}`)

	_, err := LoadProjectsFile(path)
	if err == nil {
		t.Fatal("expected error for duplicate tokens")
	}
}

func TestLoadProjectsFile_SingleProject(t *testing.T) {
	path := writeFile(t, `{
		"projects": [
			{"name": "solo", "token": "tok-only", "data_file": "solo.json"}
		]
	}`)

	entries, err := LoadProjectsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].DataFile != "solo.json" {
		t.Errorf("expected data_file=solo.json, got %q", entries[0].DataFile)
	}
}
