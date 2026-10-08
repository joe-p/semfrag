package semfrag

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicWritePreservesPermissionsAndSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permissions and symlinks")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target.md")
	link := filepath.Join(root, "CHANGELOG.md")
	writeFile(t, target, "original")
	if err := os.Chmod(target, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.md", link); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(link, "updated"); err != nil {
		t.Fatal(err)
	}
	assertEqual(t, readFile(t, target), "updated")
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, info.Mode().Perm(), fs.FileMode(0o640))
	if _, err := os.Readlink(link); err != nil {
		t.Fatalf("destination symlink was replaced: %v", err)
	}
}

func TestFilesystemErrorsAreNotTreatedAsMissing(t *testing.T) {
	root, dir, output := makeRoot(t)
	blocked := filepath.Join(root, "file")
	writeFile(t, blocked, "not a directory")
	invalid := filepath.Join(blocked, "missing")
	writeFile(t, filepath.Join(dir, "fix.md"), "## Fixes\n- new\n")
	writeFile(t, output, "# 1.0.0\n\n## Fixes\n- old\n")
	original := readFile(t, output)
	for _, test := range []struct {
		name string
		run  func() error
	}{
		{"fragments", func() error { _, err := ReadFragments(invalid); return err }},
		{"input", func() error {
			_, err := Generate(GenerateOptions{Dir: dir, Input: invalid, Output: output, Clear: true})
			return err
		}},
		{"init", func() error {
			_, err := Init(InitOptions{Dir: dir, Output: invalid, DryRun: true})
			return err
		}},
		{"atomic write", func() error { return atomicWrite(invalid, "new") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.run()
			if err == nil || errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("expected filesystem error other than not-exist, got %v", err)
			}
		})
	}
	assertEqual(t, readFile(t, output), original)
	assertEqual(t, readDir(t, dir), []string{"fix.md"})
}
