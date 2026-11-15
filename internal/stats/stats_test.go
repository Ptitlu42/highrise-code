package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ptitlu42/highrise-code/internal/scan"
)

func make_tree(t *testing.T, files map[string]string) *scan.Result {
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
	res, err := scan.Run(context.Background(), scan.Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func Test_summarize(t *testing.T) {
	s := Summarize([]int64{5, 1, 3, 2, 4, 100, 7, 8, 9, 10})
	if s.Max != 100 || s.Median != 6 || s.P90 != 10 || s.P99 != 100 || s.Mean != 14.9 {
		t.Errorf("%+v", s)
	}
	if (Summarize(nil) != Summary{}) {
		t.Error("empty summary must be zero")
	}
	one := Summarize([]int64{7})
	if one.Median != 7 || one.P90 != 7 || one.P99 != 7 {
		t.Errorf("%+v", one)
	}
}

func Test_format_iec(t *testing.T) {
	cases := map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.00 KiB", 1536: "1.50 KiB", 1 << 20: "1.00 MiB", 5 << 30: "5.00 GiB"}
	for in, want := range cases {
		if got := Format_iec(in); got != want {
			t.Errorf("Format_iec(%d)=%q want %q", in, got, want)
		}
	}
}

func Test_compute_report(t *testing.T) {
	res := make_tree(t, map[string]string{
		"a.go":         "package a\n// c\n\nfunc f() {}\n",
		"b.py":         "# c\nx = 1\n",
		"sub/c.go":     "package c\r\n",
		"sub/deep/d.c": "int x;",
		"empty.txt":    "",
	})
	r := Compute(res)
	if r.Files != 5 || r.Dirs != 2 {
		t.Fatalf("files=%d dirs=%d", r.Files, r.Dirs)
	}
	if r.Lines.Total != 8 || r.Lines.Code.Count != 5 || r.Lines.Comment.Count != 2 || r.Lines.Blank.Count != 1 {
		t.Errorf("lines %+v", r.Lines)
	}
	if r.Lines.Code.Percent != 62.5 || r.Lines.Comment_code_rate != 0.4 {
		t.Errorf("percent %+v", r.Lines)
	}
	if r.Empty_files != 1 {
		t.Errorf("empty=%d", r.Empty_files)
	}
	if r.Depth.Max != 3 || len(r.Depth.Files_by_level) != 4 || r.Depth.Files_by_level[1] != 3 || r.Depth.Files_by_level[2] != 1 || r.Depth.Files_by_level[3] != 1 {
		t.Errorf("depth %+v", r.Depth)
	}
	if r.Line_endings.Crlf_only != 1 || r.Line_endings.No_final_newline != 1 || r.Line_endings.Lf_only != 2 {
		t.Errorf("endings %+v", r.Line_endings)
	}
	if r.Chars.Code.Count+r.Chars.Comment.Count+r.Chars.Blank.Count != r.Chars.Total {
		t.Errorf("chars do not sum")
	}
	if r.Languages[0].Name != "Go" || r.Languages[0].Files != 2 {
		t.Errorf("languages %+v", r.Languages)
	}
	var pct float64
	for _, l := range r.Languages {
		pct += l.Percent_lines
	}
	if pct < 99.9 || pct > 100.1 {
		t.Errorf("language percents sum to %v", pct)
	}
	if len(r.Line_buckets) != 7 || len(r.Char_buckets) != 6 {
		t.Errorf("buckets")
	}
	var in_buckets int64
	for _, b := range r.Line_buckets {
		in_buckets += b.Files
	}
	if in_buckets != 4 {
		t.Errorf("line buckets hold %d files", in_buckets)
	}
	if len(r.Top_by_lines) != 5 || r.Top_by_lines[0].Path != "a.go" {
		t.Errorf("top %+v", r.Top_by_lines)
	}
}

func Test_buckets_edges(t *testing.T) {
	b := buckets([]int64{0, 1, 10, 11, 5000, 5001, 99999}, []int64{10, 50, 100, 500, 1000, 5000},
		[]string{"1-10", "11-50", "51-100", "101-500", "501-1000", "1001-5000", "5000+"})
	want := []int64{2, 1, 0, 0, 0, 1, 2}
	for i, w := range want {
		if b[i].Files != w {
			t.Errorf("bucket %s: %d want %d", b[i].Label, b[i].Files, w)
		}
	}
}

func Test_empty_report_json(t *testing.T) {
	res := make_tree(t, map[string]string{})
	r := Compute(res)
	var buf bytes.Buffer
	if err := Write_json(&buf, r, res); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, buf.String())
	}
}

func Test_json_with_hostile_names(t *testing.T) {
	root := t.TempDir()
	created := 0
	for _, n := range []string{"a\nb.go", "esc\x1b[0m.go", "bad\xff.go"} {
		if err := os.WriteFile(filepath.Join(root, n), []byte("x\n"), 0o644); err != nil {
			t.Logf("filesystem rejects %q: %v", n, err)
			continue
		}
		created++
	}
	if created == 0 {
		t.Skip("filesystem rejects every hostile name")
	}
	res, err := scan.Run(context.Background(), scan.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write_json(&buf, Compute(res), res); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Files []File_stats `json:"files"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(out.Files) != created || strings.Contains(buf.String(), "\x1b") {
		t.Errorf("unexpected output")
	}
}
