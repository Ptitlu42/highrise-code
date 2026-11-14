package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ptitlu42/highrise-code/internal/stats"
)

const (
	bar_width       = 30
	max_level_lines = 32
)

type painter struct {
	color bool
}

func (p painter) bold(s string) string {
	if !p.color {
		return s
	}
	return "\x1b[1m" + s + "\x1b[0m"
}

func (p painter) dim(s string) string {
	if !p.color {
		return s
	}
	return "\x1b[2m" + s + "\x1b[0m"
}

func group_int(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func fmt_float(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func fmt_pct(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64) + "%"
}

type text_table struct {
	headers []string
	left    []bool
	rows    [][]string
}

func (t *text_table) add(cells ...string) {
	t.rows = append(t.rows, cells)
}

func (t *text_table) print(w io.Writer, p painter) {
	widths := make([]int, len(t.headers))
	for i, h := range t.headers {
		widths[i] = len([]rune(h))
	}
	for _, r := range t.rows {
		for i, c := range r {
			if n := len([]rune(c)); n > widths[i] {
				widths[i] = n
			}
		}
	}
	line := func(cells []string, header bool) {
		var b strings.Builder
		b.WriteString("  ")
		for i, c := range cells {
			pad := widths[i] - len([]rune(c))
			if t.left[i] {
				b.WriteString(c)
				b.WriteString(strings.Repeat(" ", pad))
			} else {
				b.WriteString(strings.Repeat(" ", pad))
				b.WriteString(c)
			}
			if i < len(cells)-1 {
				b.WriteString("  ")
			}
		}
		text := strings.TrimRight(b.String(), " ")
		if header {
			text = p.dim(text)
		}
		fmt.Fprintln(w, text)
	}
	line(t.headers, true)
	for _, r := range t.rows {
		line(r, false)
	}
}

func bar(percent float64) string {
	filled := int(percent / 100 * bar_width)
	if percent > 0 && filled == 0 {
		filled = 1
	}
	if filled > bar_width {
		filled = bar_width
	}
	return strings.Repeat("#", filled) + strings.Repeat(".", bar_width-filled)
}

func kv(w io.Writer, key, value string) {
	fmt.Fprintf(w, "  %-28s %s\n", key, value)
}

func Write_text_stats(out io.Writer, rep *stats.Report, color bool) error {
	w := bufio.NewWriter(out)
	p := painter{color: color}
	head := func(title string) {
		fmt.Fprintf(w, "\n%s\n", p.bold(title))
	}

	fmt.Fprintf(w, "%s %s\n", p.bold("highrise-code statistics for"), Sanitize(rep.Root))

	head("Overview")
	kv(w, "Files", group_int(rep.Files))
	kv(w, "Directories", group_int(rep.Dirs))
	kv(w, "Size", fmt.Sprintf("%s (%s bytes)", rep.Bytes_iec, group_int(rep.Bytes)))
	kv(w, "Empty files", group_int(rep.Empty_files))
	kv(w, "Scan time", fmt_float(rep.Scan_seconds)+" s")
	if rep.Truncated {
		kv(w, "Truncated", "yes (file limit reached)")
	}

	head("Lines")
	kv(w, "Total", group_int(rep.Lines.Total))
	kv(w, "Code", fmt.Sprintf("%s (%s)", group_int(rep.Lines.Code.Count), fmt_pct(rep.Lines.Code.Percent)))
	kv(w, "Comments", fmt.Sprintf("%s (%s)", group_int(rep.Lines.Comment.Count), fmt_pct(rep.Lines.Comment.Percent)))
	kv(w, "Blank", fmt.Sprintf("%s (%s)", group_int(rep.Lines.Blank.Count), fmt_pct(rep.Lines.Blank.Percent)))
	kv(w, "Comment/code ratio", fmt_float(rep.Lines.Comment_code_rate))

	head("Characters")
	kv(w, "Total", group_int(rep.Chars.Total))
	kv(w, "Non-whitespace", fmt.Sprintf("%s (%s)", group_int(rep.Chars.Non_whitespace), fmt_pct(rep.Chars.Non_whitespace_pct)))
	kv(w, "In code lines", fmt.Sprintf("%s (%s)", group_int(rep.Chars.Code.Count), fmt_pct(rep.Chars.Code.Percent)))
	kv(w, "In comment lines", fmt.Sprintf("%s (%s)", group_int(rep.Chars.Comment.Count), fmt_pct(rep.Chars.Comment.Percent)))
	kv(w, "In blank lines", fmt.Sprintf("%s (%s)", group_int(rep.Chars.Blank.Count), fmt_pct(rep.Chars.Blank.Percent)))
	kv(w, "Average per line", fmt_float(rep.Chars.Average_per_line))

	head("Per-file statistics")
	pf := &text_table{headers: []string{"", "mean", "median", "p90", "p99", "max"}, left: []bool{true, false, false, false, false, false}}
	pf.add("Lines", fmt_float(rep.Lines_per_file.Mean), fmt_float(rep.Lines_per_file.Median), group_int(rep.Lines_per_file.P90), group_int(rep.Lines_per_file.P99), group_int(rep.Lines_per_file.Max))
	pf.add("Characters", fmt_float(rep.Chars_per_file.Mean), fmt_float(rep.Chars_per_file.Median), group_int(rep.Chars_per_file.P90), group_int(rep.Chars_per_file.P99), group_int(rep.Chars_per_file.Max))
	pf.add("Size (bytes)", fmt_float(rep.Size_per_file.Mean), fmt_float(rep.Size_per_file.Median), "", "", "")
	pf.print(w, p)

	head("Depth")
	kv(w, "Maximum", strconv.Itoa(rep.Depth.Max))
	kv(w, "Average", fmt_float(rep.Depth.Mean))
	lv := &text_table{headers: []string{"level", "files"}, left: []bool{false, false}}
	for i := 1; i < len(rep.Depth.Files_by_level) && i <= max_level_lines; i++ {
		lv.add(strconv.Itoa(i), group_int(rep.Depth.Files_by_level[i]))
	}
	lv.print(w, p)
	if len(rep.Depth.Files_by_level)-1 > max_level_lines {
		fmt.Fprintf(w, "  %s\n", p.dim("deeper levels omitted"))
	}

	write_buckets := func(title string, buckets []stats.Bucket) {
		head(title)
		t := &text_table{headers: []string{"range", "files", "share", ""}, left: []bool{true, false, false, true}}
		for _, b := range buckets {
			t.add(b.Label, group_int(b.Files), fmt_pct(b.Percent), bar(b.Percent))
		}
		t.print(w, p)
	}
	write_buckets("File length distribution (lines)", rep.Line_buckets)
	write_buckets("File length distribution (characters)", rep.Char_buckets)

	head("Line endings")
	kv(w, "LF only", group_int(rep.Line_endings.Lf_only))
	kv(w, "CRLF only", group_int(rep.Line_endings.Crlf_only))
	kv(w, "Mixed", group_int(rep.Line_endings.Mixed))
	kv(w, "No final newline", group_int(rep.Line_endings.No_final_newline))

	head("Languages")
	lt := &text_table{
		headers: []string{"language", "category", "files", "lines", "code", "comment", "blank", "chars", "bytes", "%lines", "%chars", "%code", "avg lines", "avg chars"},
		left:    []bool{true, true, false, false, false, false, false, false, false, false, false, false, false, false},
	}
	for _, l := range rep.Languages {
		lt.add(l.Name, l.Category, group_int(l.Files), group_int(l.Lines), group_int(l.Code), group_int(l.Comment), group_int(l.Blank),
			group_int(l.Chars), group_int(l.Bytes), fmt_pct(l.Percent_lines), fmt_pct(l.Percent_chars), fmt_pct(l.Percent_code),
			fmt_float(l.Avg_lines_per_file), fmt_float(l.Avg_chars_per_file))
	}
	lt.print(w, p)

	head("Categories")
	ct := &text_table{headers: []string{"category", "files", "lines", "%lines"}, left: []bool{true, false, false, false}}
	for _, c := range rep.Categories {
		ct.add(c.Name, group_int(c.Files), group_int(c.Lines), fmt_pct(c.Percent))
	}
	ct.print(w, p)

	write_top := func(title string, files []stats.Top_file) {
		head(title)
		t := &text_table{headers: []string{"lines", "bytes", "language", "path"}, left: []bool{false, false, true, true}}
		for _, f := range files {
			t.add(group_int(f.Lines), group_int(f.Bytes), f.Language, Sanitize(f.Path))
		}
		t.print(w, p)
	}
	write_top("Largest files by lines", rep.Top_by_lines)
	write_top("Largest files by size", rep.Top_by_bytes)

	head("Skipped")
	kv(w, "Symlinks", group_int(rep.Skipped.Symlinks))
	kv(w, "Special files", group_int(rep.Skipped.Special))
	kv(w, "Binary files", group_int(rep.Skipped.Binary))
	kv(w, "Too large", group_int(rep.Skipped.Too_large))
	kv(w, "Ignored by rules", group_int(rep.Skipped.Ignored))
	kv(w, "Hidden", group_int(rep.Skipped.Hidden))
	kv(w, "Unreadable", group_int(rep.Skipped.Unreadable))
	kv(w, "Depth limit", group_int(rep.Skipped.Depth_limit))
	kv(w, "Warnings", group_int(rep.Warnings_total))
	return w.Flush()
}
