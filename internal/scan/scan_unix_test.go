//go:build unix

package scan

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func Test_binary_and_special_files(t *testing.T) {
	root := write_tree(t, map[string]string{
		"text.go":      "package x\n",
		"blob.dat":     "abc\x00def",
		"sub/nested.c": "int x;\n",
	})
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("text.go", filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sub", filepath.Join(root, "dirlink")); err != nil {
		t.Fatal(err)
	}
	res := scan_root(t, root, nil)
	if res.Root.Files_total != 2 {
		t.Errorf("files_total=%d", res.Root.Files_total)
	}
	if res.Skipped.Binary != 1 || res.Skipped.Special != 1 || res.Skipped.Symlinks != 3 {
		t.Errorf("skipped=%+v", res.Skipped)
	}
	if res.Root.Dirs_total != 1 {
		t.Errorf("dirs_total=%d", res.Root.Dirs_total)
	}
}

func Test_permission_errors_are_warnings(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permissions")
	}
	root := write_tree(t, map[string]string{"ok.go": "x\n", "locked/f.go": "x\n", "secret.go": "x\n"})
	if err := os.Chmod(filepath.Join(root, "locked"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "secret.go"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Join(root, "locked"), 0o755) })
	res := scan_root(t, root, nil)
	if res.Warnings_total != 2 || res.Skipped.Unreadable != 2 {
		t.Errorf("warnings=%d skipped=%+v", res.Warnings_total, res.Skipped)
	}
	if res.Root.Files_total != 1 {
		t.Errorf("files_total=%d", res.Root.Files_total)
	}
}
