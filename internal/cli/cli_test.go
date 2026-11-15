package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParse_size(t *testing.T) {
	cases := map[string]int64{
		"1":      1,
		"64MiB":  64 << 20,
		"1k":     1 << 10,
		"1KB":    1000,
		"2 GiB":  2 << 30,
		"1.5M":   3 << 19,
		"512B":   512,
		"10 mib": 10 << 20,
	}
	for input, want := range cases {
		got, err := Parse_size(input)
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}
		if got != want {
			t.Fatalf("%q: got %d want %d", input, got, want)
		}
	}
	for _, bad := range []string{"", "abc", "-1", "0", "1XB", "1e5", "99999999999999999999TiB"} {
		if _, err := Parse_size(bad); err == nil {
			t.Fatalf("%q: expected error", bad)
		}
	}
}

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"plain.go":        "plain.go",
		"a\nb":            `a\nb`,
		"a\tb":            `a\tb`,
		"\x1b[31mred":     `\x1b[31mred`,
		"bad\xffbyte":     `bad\xffbyte`,
		"zero\u200bwidth": `zero\u200bwidth`,
		"caf\u00e9":       "caf\u00e9",
		"\u2028x":         `\u2028x`,
	}
	for input, want := range cases {
		if got := Sanitize(input); got != want {
			t.Fatalf("%q: got %q want %q", input, got, want)
		}
	}
}

func TestParse_args_defaults(t *testing.T) {
	opts, err := Parse_args([]string{"."})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Root != "." || opts.Mode != "3d" || opts.Color_by != "language" || opts.Height_by != "lines" {
		t.Fatalf("unexpected defaults: %+v", opts)
	}
	if opts.Max_depth != 256 || opts.Format != "text" || opts.Max_files != 2000000 {
		t.Fatalf("unexpected limits: %+v", opts)
	}
}

func TestParse_args_flags(t *testing.T) {
	opts, err := Parse_args([]string{
		"-m", "iso", "--color-by=depth", "-Hcode", "-d", "5", "-i", "*.log", "-i", "tmp/",
		"--include-hidden", "--no-gitignore", "--max-files", "10", "--max-file-size", "1MiB",
		"--workers", "3", "--port", "9000", "--no-open", "--idle-timeout", "30s", "--strict", "-v", "root",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Mode != "iso" || opts.Color_by != "depth" || opts.Height_by != "code" || opts.Max_depth != 5 {
		t.Fatalf("unexpected view options: %+v", opts)
	}
	if len(opts.Ignore) != 2 || !opts.Include_hidden || !opts.No_gitignore || opts.Max_files != 10 {
		t.Fatalf("unexpected scan options: %+v", opts)
	}
	if opts.Max_file_size != 1<<20 || opts.Workers != 3 || opts.Port != 9000 || !opts.No_open {
		t.Fatalf("unexpected runtime options: %+v", opts)
	}
	if opts.Idle_timeout != 30*time.Second || !opts.Strict || !opts.Verbose || opts.Root != "root" {
		t.Fatalf("unexpected misc options: %+v", opts)
	}
}

func TestParse_args_clustered_short(t *testing.T) {
	opts, err := Parse_args([]string{"-vi", "x", "dir"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.Verbose || len(opts.Ignore) != 1 || opts.Ignore[0] != "x" {
		t.Fatalf("unexpected: %+v", opts)
	}
}

func TestParse_args_caps(t *testing.T) {
	opts, err := Parse_args([]string{"-d", "100000", "--workers", "1000", "."})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Max_depth != 256 || opts.Workers != 64 || len(opts.Notes) != 2 {
		t.Fatalf("caps not applied: %+v", opts)
	}
}

func TestParse_args_double_dash(t *testing.T) {
	opts, err := Parse_args([]string{"--", "-weird"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Root != "-weird" {
		t.Fatalf("root %q", opts.Root)
	}
}

func TestParse_args_errors(t *testing.T) {
	cases := [][]string{
		{},
		{"a", "b"},
		{"--nope", "."},
		{"-Z", "."},
		{"--mode", "4d", "."},
		{"--mode"},
		{"-d", "0", "."},
		{"-d", "x", "."},
		{"--workers", "0", "."},
		{"--port", "70000", "."},
		{"--format", "json", "."},
		{"-v", "-q", "."},
		{"--stats=1", "."},
		{"--idle-timeout", "-5s", "."},
		{"--idle-timeout", "abc", "."},
		{"--max-file-size", "zzz", "."},
		{"--max-files", "0", "."},
		{"--out", "", "."},
		{"-i", "", "."},
	}
	for _, args := range cases {
		_, err := Parse_args(args)
		if err == nil {
			t.Fatalf("%v: expected error", args)
		}
		if !Is_usage_error(err) {
			t.Fatalf("%v: expected usage error, got %T", args, err)
		}
	}
}

func TestParse_args_help_version_skip_validation(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"-V"}, {"--version"}} {
		if _, err := Parse_args(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func test_env(t *testing.T) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	return Env{Stdout: out, Stderr: errb, Getenv: func(string) string { return "" }}, out, errb
}

func make_project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"main.go":      "package main\n\n// hi\nfunc main() {}\n",
		"lib/util.py":  "# c\nx = 1\n",
		"docs/read.md": "# title\n",
	}
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

func TestRun_help_and_version(t *testing.T) {
	env, out, _ := test_env(t)
	if code := Run(context.Background(), []string{"--help"}, env); code != 0 {
		t.Fatalf("help exit %d", code)
	}
	if !strings.Contains(out.String(), "Usage: highrise-code") {
		t.Fatalf("help output: %q", out.String())
	}
	env, out, _ = test_env(t)
	if code := Run(context.Background(), []string{"-V"}, env); code != 0 {
		t.Fatalf("version exit %d", code)
	}
	if !strings.HasPrefix(out.String(), "highrise-code ") {
		t.Fatalf("version output: %q", out.String())
	}
}

func TestRun_usage_error(t *testing.T) {
	env, out, errb := test_env(t)
	if code := Run(context.Background(), []string{"--bogus"}, env); code != 2 {
		t.Fatalf("exit %d", code)
	}
	if out.Len() != 0 || !strings.Contains(errb.String(), "unknown option") {
		t.Fatalf("stdout %q stderr %q", out.String(), errb.String())
	}
}

func TestRun_missing_root(t *testing.T) {
	env, _, _ := test_env(t)
	missing := filepath.Join(t.TempDir(), "nope")
	if code := Run(context.Background(), []string{"--stats", missing}, env); code != 1 {
		t.Fatalf("exit %d", code)
	}
}

func TestRun_stats_text(t *testing.T) {
	env, out, _ := test_env(t)
	if code := Run(context.Background(), []string{"-q", "--stats", make_project(t)}, env); code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"Go", "Python", "Markdown"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in output", want)
		}
	}
}

func TestRun_stats_json(t *testing.T) {
	env, out, _ := test_env(t)
	if code := Run(context.Background(), []string{"-q", "--stats", "--format", "json", make_project(t)}, env); code != 0 {
		t.Fatalf("exit %d", code)
	}
	text := out.String()
	if !strings.HasPrefix(text, "{") || !strings.Contains(text, `"languages"`) {
		t.Fatalf("unexpected json: %.200s", text)
	}
}

func TestRun_out_file(t *testing.T) {
	env, _, _ := test_env(t)
	target := filepath.Join(t.TempDir(), "city.html")
	if code := Run(context.Background(), []string{"-q", "--out", target, make_project(t)}, env); code != 0 {
		t.Fatalf("exit %d", code)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("<!doctype html>")) && !bytes.Contains(data, []byte("<!DOCTYPE html>")) {
		t.Fatalf("not html: %.120s", data)
	}
	if bytes.Contains(data, []byte("{{head}}")) || bytes.Contains(data, []byte("{{boot}}")) {
		t.Fatal("unreplaced markers")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
		t.Fatalf("mode %v", info.Mode())
	}
}

func TestRun_out_stdout(t *testing.T) {
	env, out, _ := test_env(t)
	if code := Run(context.Background(), []string{"-q", "--out", "-", make_project(t)}, env); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out.Len() == 0 {
		t.Fatal("no output")
	}
}

func TestRun_strict_with_warnings(t *testing.T) {
	root := make_project(t)
	if err := os.WriteFile(filepath.Join(root, "big.txt"), bytes.Repeat([]byte("x\n"), 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	env, _, _ := test_env(t)
	code := Run(context.Background(), []string{"-q", "--stats", "--strict", "--max-file-size", "1KiB", root}, env)
	if code != 3 {
		t.Fatalf("exit %d", code)
	}
	env, _, _ = test_env(t)
	code = Run(context.Background(), []string{"-q", "--stats", "--max-file-size", "1KiB", root}, env)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
}

func TestRun_cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	env, _, _ := test_env(t)
	if code := Run(ctx, []string{"-q", "--stats", make_project(t)}, env); code != 1 {
		t.Fatalf("exit %d", code)
	}
}

func TestColor_enabled(t *testing.T) {
	var buf bytes.Buffer
	if color_enabled(&buf, func(string) string { return "" }) {
		t.Fatal("non-file writer must not enable color")
	}
}
