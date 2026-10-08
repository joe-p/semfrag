package semfrag_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	semfrag "github.com/joe-p/semfrag"
)

func TestCLIInitHonorsUmask(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("umask is a Unix permission mechanism")
	}
	root := t.TempDir()
	command := exec.Command("sh", "-c", `umask 077; exec "$1" init`, "sh", cliPath)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, output)
	}
	for _, name := range []string{"CHANGELOG.md", "semfrag.json"} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("%s permissions = %04o, want 0600", name, got)
		}
	}
}

var cliPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "semfrag-cli-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	cliPath = filepath.Join(dir, "semfrag")
	build := exec.Command("go", "build", "-o", cliPath, "./cmd/semfrag")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	m.Run()
}

func runCLI(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command(cliPath, args...)
	command.Dir = dir
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err != nil {
		return stdout.String() + stderr.String(), err
	}
	return stdout.String(), nil
}

func cliRoot(t *testing.T) (root, dir, output string) {
	t.Helper()
	root = t.TempDir()
	dir = filepath.Join(root, "changelog.d")
	output = filepath.Join(root, "CHANGELOG.md")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}
	return root, dir, output
}

func cliWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func cliRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	return string(data)
}

func TestCLILatestPrintsReleasedVersion(t *testing.T) {
	root, _, output := cliRoot(t)
	cliWrite(t, output, "# 1.0.0\n\n## Features\n\n- Released\n")

	stdout, err := runCLI(t, root, "latest")
	if err != nil {
		t.Fatalf("runCLI returned error: %v", err)
	}
	if stdout != "1.0.0\n" {
		t.Fatalf("got %q, want %q", stdout, "1.0.0\n")
	}
}

func TestCLINotesPrintsLatestReleaseBody(t *testing.T) {
	root, _, output := cliRoot(t)
	cliWrite(t, output, "# 1.0.0\n\n## Features\n\n- Released\n")

	stdout, err := runCLI(t, root, "notes")
	if err != nil {
		t.Fatalf("runCLI returned error: %v", err)
	}
	if stdout != "## Features\n\n- Released\n" {
		t.Fatalf("got %q, want %q", stdout, "## Features\n\n- Released\n")
	}
}

func TestCLIPromoteRequiresKnownChannel(t *testing.T) {
	root, _, output := cliRoot(t)
	original := "# 1.0.1 - UNRELEASED\n\n## Fixes\n- fix\n"
	cliWrite(t, output, original)

	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"promote"}, "promote requires a channel"},
		{[]string{"promote", "gamma"}, "unknown channel: gamma"},
		{[]string{"promote", "stable", "extra"}, "unexpected argument: extra"},
	} {
		message, err := runCLI(t, root, test.args...)
		if err == nil {
			t.Fatalf("expected error for %v", test.args)
		}
		if !strings.Contains(message, test.want) {
			t.Fatalf("expected %q in %q", test.want, message)
		}
	}
	if got := cliRead(t, output); got != original {
		t.Fatalf("changelog was modified: %q", got)
	}
}

func TestCLIPromoteTagsUnreleasedSection(t *testing.T) {
	root, _, output := cliRoot(t)
	cliWrite(t, output, "# 1.0.1 - UNRELEASED\n\n## Fixes\n- fix\n")

	stdout, err := runCLI(t, root, "promote", "alpha")
	if err != nil {
		t.Fatalf("runCLI returned error: %v", err)
	}
	if !strings.Contains(stdout, "Promoted to 1.0.1-alpha.1") {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
	if !strings.HasPrefix(cliRead(t, output), "# 1.0.1-alpha.1 - ") {
		t.Fatalf("changelog not promoted: %q", cliRead(t, output))
	}
}

func TestCLIStdoutUsesDefaultChangelog(t *testing.T) {
	root, dir, output := cliRoot(t)
	cliWrite(t, output, "# 2.3.4\n\n## Fixes\n- old\n")
	cliWrite(t, filepath.Join(root, "semfrag.json"), `{"sections":[{"title":"Fixes","bump":"PATCH"}]}`)
	cliWrite(t, filepath.Join(dir, "fix.md"), "## Fixes\n- new\n")

	preview, err := runCLI(t, root, "-o", "-")
	if err != nil {
		t.Fatalf("runCLI returned error: %v", err)
	}
	if !strings.HasPrefix(preview, "# 2.3.5 - UNRELEASED") {
		t.Fatalf("unexpected preview: %q", preview)
	}

	if _, err := runCLI(t, root); err != nil {
		t.Fatalf("runCLI generate returned error: %v", err)
	}
	pending, err := runCLI(t, root, "--dry-run")
	if err != nil {
		t.Fatalf("runCLI dry-run returned error: %v", err)
	}
	if !strings.HasPrefix(pending, "# 2.3.5 - UNRELEASED") {
		t.Fatalf("unexpected pending output: %q", pending)
	}
}

func TestCLIRejectsFlagLikeOptionValues(t *testing.T) {
	root, dir, output := cliRoot(t)
	original := "# 1.0.0\n\n## Features\n\n- Released\n"
	cliWrite(t, output, original)
	fragment := filepath.Join(dir, "fix.md")
	cliWrite(t, fragment, "## Fixes\n- fix\n")

	for _, args := range [][]string{
		{"generate", "--output", "--dry-run"},
		{"generate", "-o", "--dry-run"},
		{"generate", "--config", "--no-clear"},
	} {
		message, err := runCLI(t, root, args...)
		if err == nil {
			t.Fatalf("expected error for %v, got %q", args, message)
		}
		if !strings.Contains(message, "ambiguous") {
			t.Fatalf("expected an ambiguous-argument error for %v, got %q", args, message)
		}
	}

	if _, err := os.Stat(fragment); err != nil {
		t.Fatalf("fragment was cleared: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "--dry-run")); err == nil {
		t.Fatal("created a file named --dry-run")
	}
	if got := cliRead(t, output); got != original {
		t.Fatalf("changelog changed: %q", got)
	}
}

func TestCLIAllowsExplicitAndStdoutValues(t *testing.T) {
	root, dir, _ := cliRoot(t)
	cliWrite(t, filepath.Join(dir, "fix.md"), "## Fixes\n- fix\n")

	if _, err := runCLI(t, root, "generate", "--output=-weird", "--no-clear"); err != nil {
		t.Fatalf("explicit --output=-weird should be accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "-weird")); err != nil {
		t.Fatalf("expected file -weird to be written: %v", err)
	}

	stdout, err := runCLI(t, root, "generate", "-o", "-", "--no-clear")
	if err != nil {
		t.Fatalf("stdout value - should be accepted: %v", err)
	}
	if !strings.HasPrefix(stdout, "# ") {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestCLIVersionUsesLinkerFlag(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "semfrag-version")
	build := exec.Command("go", "build", "-ldflags", "-X main.version=9.9.9", "-o", binary, "./cmd/semfrag")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("go build returned error: %v", err)
	}

	out, err := exec.Command(binary, "--version").Output()
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "9.9.9" {
		t.Fatalf("got %q, want %q", got, "9.9.9")
	}
}

func TestCLIInitWritesInitialVersionAndConfig(t *testing.T) {
	root, dir, output := cliRoot(t)
	_ = dir

	stdout, err := runCLI(t, root, "init", "--initial", "0.1.0")
	if err != nil {
		t.Fatalf("runCLI returned error: %v", err)
	}
	if !strings.Contains(stdout, "Initialized") || !strings.Contains(stdout, "0.1.0 - UNRELEASED") || !strings.Contains(stdout, "semfrag.json") {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
	if got := cliRead(t, output); got != "# 0.1.0 - UNRELEASED\n" {
		t.Fatalf("unexpected changelog: %q", got)
	}

	config, err := semfrag.ReadConfig(filepath.Join(root, "semfrag.json"))
	if err != nil {
		t.Fatalf("ReadConfig returned error: %v", err)
	}
	if config.Sections[0].Bump != semfrag.BumpMinor {
		t.Fatalf("expected first section bump MINOR, got %q", config.Sections[0].Bump)
	}
}
