package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

var testBinaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all tests
	build := exec.Command("go", "build", "-o", "bs_test_binary", ".")
	if out, err := build.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		os.Exit(1)
	}
	testBinaryPath = "./bs_test_binary"

	code := m.Run()

	os.Remove("bs_test_binary")
	os.Exit(code)
}

func TestBinaryRunsAndShowsHelp(t *testing.T) {
	out, err := exec.Command(testBinaryPath).CombinedOutput()
	if err != nil {
		t.Fatalf("binary exited with error: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Beads server CLI") {
		t.Errorf("expected help output to contain 'Beads server CLI', got: %q", output)
	}
}

func TestBinaryVersionFlag(t *testing.T) {
	out, err := exec.Command(testBinaryPath, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("binary exited with error: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "client: ") {
		t.Errorf("expected output to contain 'client: ', got: %q", output)
	}
	if !strings.Contains(output, "server: ") {
		t.Errorf("expected output to contain 'server: ', got: %q", output)
	}
}

func TestBinaryWhoami(t *testing.T) {
	cmd := exec.Command(testBinaryPath, "whoami")
	cmd.Env = append(os.Environ(), "BS_USER=test-agent")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary exited with error: %v\n%s", err, out)
	}

	output := strings.TrimSpace(string(out))
	want := `{"user":"test-agent"}`
	if output != want {
		t.Errorf("whoami output = %q, want %q", output, want)
	}
}

func TestBinaryServeRefusesWithoutToken(t *testing.T) {
	cmd := exec.Command(testBinaryPath, "serve")
	cmd.Env = []string{"HOME=" + os.Getenv("HOME"), "PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected serve to fail without token, but it succeeded")
	}

	output := string(out)
	if !strings.Contains(output, "token is required") {
		t.Errorf("expected error about missing token, got: %q", output)
	}
}
