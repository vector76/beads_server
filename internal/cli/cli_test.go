package cli

import (
	"bytes"
	"os"
	"testing"
)

func TestWhoami_DefaultAnonymous(t *testing.T) {
	os.Unsetenv("BS_USER")

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"whoami"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	want := `{"user":"anonymous"}` + "\n"
	if got != want {
		t.Errorf("whoami output = %q, want %q", got, want)
	}
}

func TestWhoami_WithBSUser(t *testing.T) {
	os.Setenv("BS_USER", "agent-007")
	defer os.Unsetenv("BS_USER")

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"whoami"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	want := `{"user":"agent-007"}` + "\n"
	if got != want {
		t.Errorf("whoami output = %q, want %q", got, want)
	}
}

func TestServe_RefusesWithoutToken(t *testing.T) {
	os.Unsetenv("BS_TOKEN")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"serve"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when token is missing, got nil")
	}
}

func TestServe_RefusesWithoutToken_EnvAlsoClear(t *testing.T) {
	os.Unsetenv("BS_TOKEN")

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"serve", "--token", ""})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when token is empty, got nil")
	}
}
