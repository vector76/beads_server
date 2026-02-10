package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestWhoami_DefaultAnonymous(t *testing.T) {
	os.Unsetenv("BS_USER")
	resetDotenv()
	t.Cleanup(resetDotenv)

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

func TestHelp_RootShowsClientCommands(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()

	// Verify grouped sections appear
	if !strings.Contains(out, "Client Commands:") {
		t.Error("help output missing 'Client Commands:' group")
	}
	if !strings.Contains(out, "Server Commands:") {
		t.Error("help output missing 'Server Commands:' group")
	}

	// Verify key client commands are listed
	for _, name := range []string{"add", "show", "edit", "delete", "list", "search", "claim", "mine", "comment"} {
		if !strings.Contains(out, name) {
			t.Errorf("help output missing client command %q", name)
		}
	}

	// Verify serve is listed
	if !strings.Contains(out, "serve") {
		t.Error("help output missing 'serve' command")
	}
}

func TestHelp_ServeShowsFlags(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"serve", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()

	for _, flag := range []string{"--port", "--data-file", "--token"} {
		if !strings.Contains(out, flag) {
			t.Errorf("serve help output missing flag %q", flag)
		}
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
