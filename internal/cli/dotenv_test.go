package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDotenv creates a .env file in dir and changes the working directory
// to dir for the duration of the test. It also resets the dotenv cache.
func writeDotenv(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	resetDotenv()
	t.Cleanup(func() {
		os.Chdir(origDir)
		resetDotenv()
	})
}

func TestLoadDotenv_BasicKeyValue(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "FOO=bar\nBAZ=qux\n")

	vals := loadDotenv()
	if vals["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", vals["FOO"], "bar")
	}
	if vals["BAZ"] != "qux" {
		t.Errorf("BAZ = %q, want %q", vals["BAZ"], "qux")
	}
}

func TestLoadDotenv_CommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "# comment\n\nFOO=bar\n  # indented comment\n")

	vals := loadDotenv()
	if vals["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", vals["FOO"], "bar")
	}
	if _, ok := vals["# comment"]; ok {
		t.Error("comment should not be parsed as a key")
	}
}

func TestLoadDotenv_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, `FOO="hello world"
BAR='single quoted'
BAZ=unquoted value
`)

	vals := loadDotenv()
	if vals["FOO"] != "hello world" {
		t.Errorf("FOO = %q, want %q", vals["FOO"], "hello world")
	}
	if vals["BAR"] != "single quoted" {
		t.Errorf("BAR = %q, want %q", vals["BAR"], "single quoted")
	}
	if vals["BAZ"] != "unquoted value" {
		t.Errorf("BAZ = %q, want %q", vals["BAZ"], "unquoted value")
	}
}

func TestLoadDotenv_WhitespaceHandling(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "  FOO  =  bar  \n")

	vals := loadDotenv()
	if vals["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", vals["FOO"], "bar")
	}
}

func TestLoadDotenv_NoFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	resetDotenv()
	t.Cleanup(func() {
		os.Chdir(origDir)
		resetDotenv()
	})

	vals := loadDotenv()
	if len(vals) != 0 {
		t.Errorf("expected empty map when .env missing, got %v", vals)
	}
}

func TestLoadDotenv_LineWithoutEquals(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "NOEQ\nFOO=bar\n")

	vals := loadDotenv()
	if _, ok := vals["NOEQ"]; ok {
		t.Error("line without = should be skipped")
	}
	if vals["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", vals["FOO"], "bar")
	}
}

func TestLoadDotenv_ValueWithEquals(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "URL=http://localhost:9999?a=b\n")

	vals := loadDotenv()
	if vals["URL"] != "http://localhost:9999?a=b" {
		t.Errorf("URL = %q, want %q", vals["URL"], "http://localhost:9999?a=b")
	}
}

func TestGetenv_EnvVarTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "MY_KEY=from-dotenv\n")

	os.Setenv("MY_KEY", "from-env")
	t.Cleanup(func() { os.Unsetenv("MY_KEY") })

	if got := getenv("MY_KEY"); got != "from-env" {
		t.Errorf("getenv(MY_KEY) = %q, want %q (env takes precedence)", got, "from-env")
	}
}

func TestGetenv_FallsThroughToDotenv(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "MY_KEY=from-dotenv\n")
	os.Unsetenv("MY_KEY")

	if got := getenv("MY_KEY"); got != "from-dotenv" {
		t.Errorf("getenv(MY_KEY) = %q, want %q (dotenv fallback)", got, "from-dotenv")
	}
}

func TestGetenv_MissingEverywhere(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "OTHER=val\n")
	os.Unsetenv("MISSING_KEY")

	if got := getenv("MISSING_KEY"); got != "" {
		t.Errorf("getenv(MISSING_KEY) = %q, want empty", got)
	}
}

func TestGetUser_FromEnv(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "BS_USER=dotenv-user\n")

	os.Setenv("BS_USER", "env-user")
	t.Cleanup(func() { os.Unsetenv("BS_USER") })

	if got := getUser(); got != "env-user" {
		t.Errorf("getUser() = %q, want %q", got, "env-user")
	}
}

func TestGetUser_FromDotenv(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "BS_USER=dotenv-user\n")
	os.Unsetenv("BS_USER")

	if got := getUser(); got != "dotenv-user" {
		t.Errorf("getUser() = %q, want %q", got, "dotenv-user")
	}
}

func TestGetUser_DefaultAnonymous(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "OTHER=val\n")
	os.Unsetenv("BS_USER")

	if got := getUser(); got != "anonymous" {
		t.Errorf("getUser() = %q, want %q", got, "anonymous")
	}
}

func TestNewClientFromEnv_TokenFromDotenv(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "BS_TOKEN=dotenv-secret\nBS_URL=http://dotenv-host:1234\n")
	os.Unsetenv("BS_TOKEN")
	os.Unsetenv("BS_URL")

	c, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Token != "dotenv-secret" {
		t.Errorf("Token = %q, want %q", c.Token, "dotenv-secret")
	}
	if c.BaseURL != "http://dotenv-host:1234" {
		t.Errorf("BaseURL = %q, want %q", c.BaseURL, "http://dotenv-host:1234")
	}
}

func TestNewClientFromEnv_EnvOverridesDotenv(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "BS_TOKEN=dotenv-secret\n")

	os.Setenv("BS_TOKEN", "env-secret")
	t.Cleanup(func() { os.Unsetenv("BS_TOKEN") })

	c, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Token != "env-secret" {
		t.Errorf("Token = %q, want %q", c.Token, "env-secret")
	}
}

func TestNewClientFromEnv_MissingTokenEverywhere(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "OTHER=val\n")
	os.Unsetenv("BS_TOKEN")

	_, err := NewClientFromEnv()
	if err == nil {
		t.Fatal("expected error when BS_TOKEN is missing everywhere")
	}
}

func TestWhoami_UserFromDotenv(t *testing.T) {
	dir := t.TempDir()
	writeDotenv(t, dir, "BS_USER=dotenv-agent\n")
	os.Unsetenv("BS_USER")

	out := runCmd(t, "whoami")
	if !strings.Contains(out, "dotenv-agent") {
		t.Errorf("whoami output = %q, want to contain %q", out, "dotenv-agent")
	}
}
