package scan

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unicode/utf8"
)

func write_tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func scan_root(t *testing.T, root string, mutate func(*Options)) *Result {
	t.Helper()
	opts := Options{Root: root, Workers: 4}
	if mutate != nil {
		mutate(&opts)
	}
	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func find_file(d *Dir, name string) *File {
	for i := range d.Files {
		if d.Files[i].Name == name {
			return &d.Files[i]
		}
	}
	for _, c := range d.Dirs {
		if f := find_file(c, name); f != nil {
			return f
		}
	}
	return nil
}

func count_bytes(t *testing.T, name string, content []byte) *File {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, name), content, 0o644); err != nil {
		t.Fatal(err)
	}
	res := scan_root(t, root, nil)
	f := find_file(res.Root, name)
	if f == nil {
		t.Fatalf("file %q not kept", name)
	}
	return f
}

func Test_line_counting_variants(t *testing.T) {
	cases := []struct {
		name    string
		content string
		lines   int64
		code    int64
		comment int64
		blank   int64
		chars   int64
		eol     uint8
	}{
		{"empty.go", "", 0, 0, 0, 0, 0, 0},
		{"lf.go", "a\n// c\n\nb\n", 4, 2, 1, 1, 10, Eol_lf},
		{"crlf.go", "a\r\n// c\r\n\r\nb\r\n", 4, 2, 1, 1, 14, Eol_crlf},
		{"mixed.go", "a\r\nb\nc", 3, 3, 0, 0, 6, Eol_lf | Eol_crlf | Eol_no_final_newline},
		{"nofinal.go", "a\nb", 2, 2, 0, 0, 3, Eol_lf | Eol_no_final_newline},
		{"blank_only.txt", "\n\n  \n", 3, 0, 0, 3, 5, Eol_lf},
		{"unicode.go", "héllo\n€\n", 2, 2, 0, 0, 8, Eol_lf},
		{"invalid.go", "a\xff\xfeb\n", 1, 1, 0, 0, 5, Eol_lf},
	}
	for _, c := range cases {
		f := count_bytes(t, c.name, []byte(c.content))
		if f.Lines != c.lines || f.Code != c.code || f.Comment != c.comment || f.Blank != c.blank {
			t.Errorf("%s: lines=%d code=%d comment=%d blank=%d", c.name, f.Lines, f.Code, f.Comment, f.Blank)
		}
		if f.Chars != c.chars {
			t.Errorf("%s: chars=%d want %d", c.name, f.Chars, c.chars)
		}
		if f.Eol != c.eol {
			t.Errorf("%s: eol=%d want %d", c.name, f.Eol, c.eol)
		}
		if f.Bytes != int64(len(c.content)) {
			t.Errorf("%s: bytes=%d want %d", c.name, f.Bytes, len(c.content))
		}
		if f.Chars_code+f.Chars_comment+f.Chars_blank != f.Chars {
			t.Errorf("%s: char classes do not sum", c.name)
		}
	}
}

func Test_char_class_attribution(t *testing.T) {
	f := count_bytes(t, "a.go", []byte("x := 1\n// hi\n\n"))
	if f.Chars_code != 7 || f.Chars_comment != 6 || f.Chars_blank != 1 {
		t.Errorf("code=%d comment=%d blank=%d", f.Chars_code, f.Chars_comment, f.Chars_blank)
	}
	if f.Chars_nonws != 4+4 {
		t.Errorf("nonws=%d", f.Chars_nonws)
	}
}

func Test_chunk_boundary_runes(t *testing.T) {
	for _, r := range []string{"é", "€", "😀"} {
		for shift := 0; shift < 5; shift++ {
			var b bytes.Buffer
			b.WriteString(strings.Repeat("a", shift))
			for b.Len() < 3*chunk_size {
				b.WriteString(r)
				if b.Len()%97 == 0 {
					b.WriteByte('\n')
				}
			}
			b.WriteString("\xe2\x82")
			content := b.Bytes()
			f := count_bytes(t, "big.txt", content)
			want := int64(utf8.RuneCount(content))
			if f.Chars != want {
				t.Fatalf("rune %q shift %d: chars=%d want %d", r, shift, f.Chars, want)
			}
			if f.Bytes != int64(len(content)) {
				t.Fatalf("bytes mismatch")
			}
		}
	}
}

func Test_very_long_line(t *testing.T) {
	content := strings.Repeat("x", 3*chunk_size+17)
	f := count_bytes(t, "long.go", []byte(content))
	if f.Lines != 1 || f.Code != 1 || f.Chars != int64(len(content)) {
		t.Errorf("lines=%d code=%d chars=%d", f.Lines, f.Code, f.Chars)
	}
}

func Test_block_comment_across_chunks(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("/*\n")
	for b.Len() < 2*chunk_size {
		b.WriteString("comment line\n")
	}
	b.WriteString("*/\ncode\n")
	f := count_bytes(t, "a.c", b.Bytes())
	if f.Code != 1 || f.Comment != f.Lines-1 {
		t.Errorf("lines=%d code=%d comment=%d", f.Lines, f.Code, f.Comment)
	}
}

func Test_bom(t *testing.T) {
	f := count_bytes(t, "a.go", []byte("\xef\xbb\xbf\nx\n"))
	if f.Lines != 2 || f.Blank != 1 || f.Code != 1 || f.Chars != 4 {
		t.Errorf("lines=%d blank=%d code=%d chars=%d", f.Lines, f.Blank, f.Code, f.Chars)
	}
}

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

func Test_gitignore_nested_and_defaults(t *testing.T) {
	root := write_tree(t, map[string]string{
		".gitignore":                   "*.log\n/top_only\n",
		"a.go":                         "package a\n",
		"a.log":                        "x\n",
		"top_only":                     "x\n",
		"sub/.gitignore":               "!keep.log\nlocal\n",
		"sub/keep.log":                 "x\n",
		"sub/other.log":                "x\n",
		"sub/local":                    "x\n",
		"sub/top_only":                 "x\n",
		"node_modules/pkg/index.js":    "x\n",
		"vendor/v.go":                  "x\n",
		"build/out.c":                  "x\n",
		".git/config":                  "x\n",
		".hidden/file.go":              "x\n",
		"src/__pycache__/m.py":         "x\n",
		"src/target/t.rs":              "x\n",
		"src/dist/d.js":                "x\n",
		"src/.dotfile":                 "x\n",
		"src/real.py":                  "print()\n",
		"nested/a/b/node_modules/x.js": "x\n",
	})
	res := scan_root(t, root, nil)
	got := collect_paths(res.Root)
	want := []string{"a.go", "nested/", "src/real.py", "sub/keep.log", "sub/top_only"}
	for _, w := range want {
		if !contains(got, w) {
			t.Errorf("missing %q in %v", w, got)
		}
	}
	for _, bad := range []string{"a.log", "sub/other.log", "sub/local", "node_modules", "vendor", "build", ".git", ".hidden", "__pycache__", "target", "dist", ".dotfile"} {
		for _, g := range got {
			if strings.Contains(g, bad) {
				t.Errorf("unexpected %q in %v", g, got)
			}
		}
	}
	if contains(got, "top_only") && !contains(got, "sub/top_only") {
		t.Errorf("anchored pattern misapplied")
	}
	for _, g := range got {
		if g == "top_only" {
			t.Errorf("root top_only must be ignored")
		}
	}
	all := scan_root(t, root, func(o *Options) { o.Include_hidden = true })
	if !contains(collect_paths(all.Root), ".hidden/file.go") {
		t.Errorf("include_hidden failed: %v", collect_paths(all.Root))
	}
	if contains(collect_paths(all.Root), ".git/config") {
		t.Errorf(".git must always be ignored")
	}
	nogit := scan_root(t, root, func(o *Options) { o.No_gitignore = true })
	if !contains(collect_paths(nogit.Root), "a.log") {
		t.Errorf("no_gitignore failed")
	}
}

func Test_user_ignore_globs(t *testing.T) {
	root := write_tree(t, map[string]string{"a.go": "x\n", "b.txt": "x\n", "gen/c.go": "x\n"})
	res := scan_root(t, root, func(o *Options) { o.Ignore = []string{"*.txt", "gen/"} })
	got := collect_paths(res.Root)
	if len(got) != 1 || got[0] != "a.go" {
		t.Errorf("got %v", got)
	}
}

func Test_limits(t *testing.T) {
	files := map[string]string{"big.txt": strings.Repeat("x", 1000)}
	for i := 0; i < 50; i++ {
		files[filepath.Join("d", string(rune('a'+i%26)), strings.Repeat("n", i+1)+".go")] = "x\n"
	}
	root := write_tree(t, files)
	res := scan_root(t, root, func(o *Options) { o.Max_file_size = 500 })
	if res.Skipped.Too_large != 1 || res.Warnings_total != 1 {
		t.Errorf("too large: %+v warnings=%d", res.Skipped, res.Warnings_total)
	}
	res = scan_root(t, root, func(o *Options) { o.Max_files = 10 })
	if !res.Truncated || res.Root.Files_total > 10 {
		t.Errorf("truncated=%v files=%d", res.Truncated, res.Root.Files_total)
	}
}

func Test_depth_limit(t *testing.T) {
	root := t.TempDir()
	path := root
	for i := 0; i < Hard_max_depth+5; i++ {
		path = filepath.Join(path, "d")
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Skip("path too long for filesystem")
	}
	res := scan_root(t, root, nil)
	if res.Skipped.Depth_limit == 0 {
		t.Errorf("depth limit not hit")
	}
}

func Test_hostile_names(t *testing.T) {
	root := t.TempDir()
	names := []string{"new\nline.go", "esc\x1b[31m.go", "bad\xff\xfe.go", "tab\there.go"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(root, n), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res := scan_root(t, root, nil)
	if res.Root.Files_total != int64(len(names)) {
		t.Errorf("files_total=%d", res.Root.Files_total)
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

func Test_cancellation(t *testing.T) {
	root := write_tree(t, map[string]string{"a.go": "x\n"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, Options{Root: root}); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func Test_deterministic_and_aggregates(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 700; i++ {
		files[filepath.Join("flat", strings.Repeat("a", 1)+string(rune('a'+i%26))+strings.Repeat("b", i/26)+".go")] = "package x\n// c\n\n"
	}
	root := write_tree(t, files)
	first := scan_root(t, root, func(o *Options) { o.Workers = 1 })
	second := scan_root(t, root, func(o *Options) { o.Workers = 16 })
	if first.Root.Metrics != second.Root.Metrics || first.Root.Files_total != second.Root.Files_total {
		t.Errorf("non deterministic aggregates")
	}
	if first.Root.Files_total != 700 || first.Root.Lines != 2100 || first.Root.Comment != 700 || first.Root.Blank != 700 {
		t.Errorf("aggregates wrong: %+v", first.Root.Metrics)
	}
	a, b := collect_paths(first.Root), collect_paths(second.Root)
	if strings.Join(a, "|") != strings.Join(b, "|") {
		t.Errorf("order differs")
	}
}

func Test_dir_path(t *testing.T) {
	root := write_tree(t, map[string]string{"a/b/c/f.go": "x\n"})
	res := scan_root(t, root, nil)
	d := res.Root.Dirs[0].Dirs[0].Dirs[0]
	if d.Path() != "a/b/c" || d.File_path(&d.Files[0]) != "a/b/c/f.go" || res.Root.Path() != "" {
		t.Errorf("path=%q", d.Path())
	}
}

func collect_paths(d *Dir) []string {
	var out []string
	var walk func(*Dir)
	walk = func(x *Dir) {
		for i := range x.Files {
			out = append(out, x.File_path(&x.Files[i]))
		}
		for _, c := range x.Dirs {
			if len(c.Dirs) == 0 && len(c.Files) == 0 {
				out = append(out, c.Path()+"/")
				continue
			}
			walk(c)
		}
	}
	walk(d)
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s || (strings.HasSuffix(s, "/") && strings.HasPrefix(x, s)) {
			return true
		}
	}
	return false
}
