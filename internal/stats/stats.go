package stats

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/ptitlu42/highrise-code/internal/lang"
	"github.com/ptitlu42/highrise-code/internal/scan"
)

const top_count = 10

type Share struct {
	Count   int64   `json:"count"`
	Percent float64 `json:"percent"`
}

type Line_totals struct {
	Total             int64   `json:"total"`
	Code              Share   `json:"code"`
	Comment           Share   `json:"comment"`
	Blank             Share   `json:"blank"`
	Comment_code_rate float64 `json:"comment_code_ratio"`
}

type Char_totals struct {
	Total              int64   `json:"total"`
	Non_whitespace     int64   `json:"non_whitespace"`
	Code               Share   `json:"code"`
	Comment            Share   `json:"comment"`
	Blank              Share   `json:"blank"`
	Average_per_line   float64 `json:"average_per_line"`
	Non_whitespace_pct float64 `json:"non_whitespace_percent"`
}

type Summary struct {
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	P90    int64   `json:"p90"`
	P99    int64   `json:"p99"`
	Max    int64   `json:"max"`
}

type Size_summary struct {
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
}

type Depth_stats struct {
	Max            int     `json:"max"`
	Mean           float64 `json:"mean"`
	Files_by_level []int64 `json:"files_by_level"`
}

type Bucket struct {
	Label   string  `json:"label"`
	Min     int64   `json:"min"`
	Max     int64   `json:"max"`
	Files   int64   `json:"files"`
	Percent float64 `json:"percent"`
}

type Line_endings struct {
	Lf_only          int64 `json:"lf_only"`
	Crlf_only        int64 `json:"crlf_only"`
	Mixed            int64 `json:"mixed"`
	No_final_newline int64 `json:"no_final_newline"`
}

type Language_stats struct {
	Name               string  `json:"name"`
	Category           string  `json:"category"`
	Files              int64   `json:"files"`
	Lines              int64   `json:"lines"`
	Code               int64   `json:"code"`
	Comment            int64   `json:"comment"`
	Blank              int64   `json:"blank"`
	Chars              int64   `json:"chars"`
	Chars_non_ws       int64   `json:"chars_non_whitespace"`
	Bytes              int64   `json:"bytes"`
	Percent_lines      float64 `json:"percent_lines"`
	Percent_chars      float64 `json:"percent_chars"`
	Percent_code       float64 `json:"percent_code"`
	Avg_lines_per_file float64 `json:"average_lines_per_file"`
	Avg_chars_per_file float64 `json:"average_chars_per_file"`
	Index              int     `json:"index"`
}

type Category_stats struct {
	Name    string  `json:"name"`
	Files   int64   `json:"files"`
	Lines   int64   `json:"lines"`
	Percent float64 `json:"percent_lines"`
}

type Top_file struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Lines    int64  `json:"lines"`
	Bytes    int64  `json:"bytes"`
}

type Skipped struct {
	Symlinks    int64 `json:"symlinks"`
	Special     int64 `json:"special_files"`
	Binary      int64 `json:"binary_files"`
	Too_large   int64 `json:"too_large"`
	Ignored     int64 `json:"ignored"`
	Hidden      int64 `json:"hidden"`
	Unreadable  int64 `json:"unreadable"`
	Depth_limit int64 `json:"depth_limit"`
}

type Report struct {
	Root           string           `json:"root"`
	Files          int64            `json:"files"`
	Dirs           int64            `json:"dirs"`
	Bytes          int64            `json:"bytes"`
	Bytes_iec      string           `json:"bytes_iec"`
	Empty_files    int64            `json:"empty_files"`
	Lines          Line_totals      `json:"lines"`
	Chars          Char_totals      `json:"characters"`
	Lines_per_file Summary          `json:"lines_per_file"`
	Chars_per_file Summary          `json:"chars_per_file"`
	Size_per_file  Size_summary     `json:"size_per_file"`
	Depth          Depth_stats      `json:"depth"`
	Line_buckets   []Bucket         `json:"lines_distribution"`
	Char_buckets   []Bucket         `json:"chars_distribution"`
	Line_endings   Line_endings     `json:"line_endings"`
	Languages      []Language_stats `json:"languages"`
	Categories     []Category_stats `json:"categories"`
	Top_by_lines   []Top_file       `json:"largest_files_by_lines"`
	Top_by_bytes   []Top_file       `json:"largest_files_by_bytes"`
	Skipped        Skipped          `json:"skipped"`
	Warnings_total int64            `json:"warnings_total"`
	Truncated      bool             `json:"truncated"`
	Scan_seconds   float64          `json:"scan_seconds"`
}

type File_stats struct {
	Path             string  `json:"path"`
	Language         string  `json:"language"`
	Bytes            int64   `json:"bytes"`
	Lines_total      int64   `json:"lines_total"`
	Lines_code       int64   `json:"lines_code"`
	Lines_comment    int64   `json:"lines_comment"`
	Lines_blank      int64   `json:"lines_blank"`
	Chars_total      int64   `json:"chars_total"`
	Chars_code       int64   `json:"chars_code"`
	Chars_comment    int64   `json:"chars_comment"`
	Chars_blank      int64   `json:"chars_blank"`
	Chars_non_ws     int64   `json:"chars_non_whitespace"`
	Avg_chars_a_line float64 `json:"average_chars_per_line"`
}

func Round2(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*100) / 100
}

func percent(part, whole int64) float64 {
	if whole == 0 {
		return 0
	}
	return Round2(float64(part) * 100 / float64(whole))
}

func ratio(a, b int64) float64 {
	if b == 0 {
		return 0
	}
	return Round2(float64(a) / float64(b))
}

func share(part, whole int64) Share {
	return Share{Count: part, Percent: percent(part, whole)}
}

func Format_iec(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < 5; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func File_of(path string, f *scan.File) File_stats {
	avg := 0.0
	if f.Lines > 0 {
		avg = Round2(float64(f.Chars) / float64(f.Lines))
	}
	return File_stats{
		Path:             path,
		Language:         f.Language().Name,
		Bytes:            f.Bytes,
		Lines_total:      f.Lines,
		Lines_code:       f.Code,
		Lines_comment:    f.Comment,
		Lines_blank:      f.Blank,
		Chars_total:      f.Chars,
		Chars_code:       f.Chars_code,
		Chars_comment:    f.Chars_comment,
		Chars_blank:      f.Chars_blank,
		Chars_non_ws:     f.Chars_nonws,
		Avg_chars_a_line: avg,
	}
}

func Each_file(res *scan.Result, fn func(File_stats) error) error {
	var walk func(d *scan.Dir) error
	walk = func(d *scan.Dir) error {
		for i := range d.Files {
			if err := fn(File_of(d.File_path(&d.Files[i]), &d.Files[i])); err != nil {
				return err
			}
		}
		for _, c := range d.Dirs {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(res.Root)
}

func Summarize(values []int64) Summary {
	n := len(values)
	if n == 0 {
		return Summary{}
	}
	slices.Sort(values)
	var sum float64
	for _, v := range values {
		sum += float64(v)
	}
	median := float64(values[n/2])
	if n%2 == 0 {
		median = (float64(values[n/2-1]) + float64(values[n/2])) / 2
	}
	return Summary{
		Mean:   Round2(sum / float64(n)),
		Median: Round2(median),
		P90:    nearest_rank(values, 0.90),
		P99:    nearest_rank(values, 0.99),
		Max:    values[n-1],
	}
}

func nearest_rank(sorted []int64, p float64) int64 {
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	return sorted[max(0, min(idx, len(sorted)-1))]
}

type language_acc struct {
	scan.Metrics
	files int64
}

type collector struct {
	lines, chars, sizes []int64
	depth_sum           int64
	depth_max           int
	by_level            []int64
	langs               []language_acc
	endings             Line_endings
	empty               int64
	top_lines           []Top_file
	top_bytes           []Top_file
}

func (c *collector) add_top(list *[]Top_file, t Top_file, key func(Top_file) int64) {
	*list = append(*list, t)
	slices.SortStableFunc(*list, func(a, b Top_file) int { return cmp.Compare(key(b), key(a)) })
	if len(*list) > top_count {
		*list = (*list)[:top_count]
	}
}

func (c *collector) walk(d *scan.Dir) {
	level := d.Depth + 1
	for i := range d.Files {
		f := &d.Files[i]
		c.lines = append(c.lines, f.Lines)
		c.chars = append(c.chars, f.Chars)
		c.sizes = append(c.sizes, f.Bytes)
		c.depth_sum += int64(level)
		if level > c.depth_max {
			c.depth_max = level
		}
		for len(c.by_level) <= level {
			c.by_level = append(c.by_level, 0)
		}
		c.by_level[level]++
		acc := &c.langs[f.Lang]
		acc.files++
		acc.Add(&f.Metrics)
		if f.Bytes == 0 {
			c.empty++
		}
		switch {
		case f.Eol&scan.Eol_lf != 0 && f.Eol&scan.Eol_crlf != 0:
			c.endings.Mixed++
		case f.Eol&scan.Eol_crlf != 0:
			c.endings.Crlf_only++
		case f.Eol&scan.Eol_lf != 0:
			c.endings.Lf_only++
		}
		if f.Eol&scan.Eol_no_final_newline != 0 {
			c.endings.No_final_newline++
		}
		if len(c.top_lines) < top_count || f.Lines > c.top_lines[len(c.top_lines)-1].Lines {
			c.add_top(&c.top_lines, Top_file{Path: d.File_path(f), Language: f.Language().Name, Lines: f.Lines, Bytes: f.Bytes}, func(t Top_file) int64 { return t.Lines })
		}
		if len(c.top_bytes) < top_count || f.Bytes > c.top_bytes[len(c.top_bytes)-1].Bytes {
			c.add_top(&c.top_bytes, Top_file{Path: d.File_path(f), Language: f.Language().Name, Lines: f.Lines, Bytes: f.Bytes}, func(t Top_file) int64 { return t.Bytes })
		}
	}
	for _, sub := range d.Dirs {
		c.walk(sub)
	}
}

func buckets(values []int64, bounds []int64, labels []string) []Bucket {
	out := make([]Bucket, len(labels))
	lo := int64(1)
	for i := range labels {
		hi := int64(-1)
		if i < len(bounds) {
			hi = bounds[i]
		}
		out[i] = Bucket{Label: labels[i], Min: lo, Max: hi}
		if hi > 0 {
			lo = hi + 1
		}
	}
	var total int64
	for _, v := range values {
		if v < 1 {
			continue
		}
		total++
		for i := range out {
			if out[i].Max < 0 || v <= out[i].Max {
				out[i].Files++
				break
			}
		}
	}
	for i := range out {
		out[i].Percent = percent(out[i].Files, total)
	}
	return out
}

func Compute(res *scan.Result) *Report {
	c := &collector{langs: make([]language_acc, lang.Count())}
	c.walk(res.Root)
	root := res.Root
	m := &root.Metrics

	rep := &Report{
		Root:           res.Root_path,
		Files:          int64(len(c.lines)),
		Dirs:           root.Dirs_total,
		Bytes:          m.Bytes,
		Bytes_iec:      Format_iec(m.Bytes),
		Empty_files:    c.empty,
		Line_endings:   c.endings,
		Warnings_total: res.Warnings_total,
		Truncated:      res.Truncated,
		Scan_seconds:   Round2(res.Duration.Seconds()),
		Top_by_lines:   c.top_lines,
		Top_by_bytes:   c.top_bytes,
		Skipped: Skipped{
			Symlinks:    res.Skipped.Symlinks,
			Special:     res.Skipped.Special,
			Binary:      res.Skipped.Binary,
			Too_large:   res.Skipped.Too_large,
			Ignored:     res.Skipped.Ignored,
			Hidden:      res.Skipped.Hidden,
			Unreadable:  res.Skipped.Unreadable,
			Depth_limit: res.Skipped.Depth_limit,
		},
	}
	rep.Lines = Line_totals{
		Total:             m.Lines,
		Code:              share(m.Code, m.Lines),
		Comment:           share(m.Comment, m.Lines),
		Blank:             share(m.Blank, m.Lines),
		Comment_code_rate: ratio(m.Comment, m.Code),
	}
	rep.Chars = Char_totals{
		Total:              m.Chars,
		Non_whitespace:     m.Chars_nonws,
		Code:               share(m.Chars_code, m.Chars),
		Comment:            share(m.Chars_comment, m.Chars),
		Blank:              share(m.Chars_blank, m.Chars),
		Average_per_line:   ratio(m.Chars, m.Lines),
		Non_whitespace_pct: percent(m.Chars_nonws, m.Chars),
	}
	rep.Size_per_file = Size_summary{}
	if len(c.sizes) > 0 {
		s := Summarize(slices.Clone(c.sizes))
		rep.Size_per_file = Size_summary{Mean: s.Mean, Median: s.Median}
	}
	rep.Lines_per_file = Summarize(slices.Clone(c.lines))
	rep.Chars_per_file = Summarize(slices.Clone(c.chars))
	rep.Depth = Depth_stats{Max: c.depth_max, Files_by_level: c.by_level}
	if len(c.lines) > 0 {
		rep.Depth.Mean = Round2(float64(c.depth_sum) / float64(len(c.lines)))
	}
	if rep.Depth.Files_by_level == nil {
		rep.Depth.Files_by_level = []int64{}
	}
	rep.Line_buckets = buckets(c.lines,
		[]int64{10, 50, 100, 500, 1000, 5000},
		[]string{"1-10", "11-50", "51-100", "101-500", "501-1000", "1001-5000", "5000+"})
	rep.Char_buckets = buckets(c.chars,
		[]int64{500, 2000, 10000, 50000, 200000},
		[]string{"1-500", "501-2k", "2k-10k", "10k-50k", "50k-200k", "200k+"})

	cats := map[lang.Category]*Category_stats{}
	for i := range c.langs {
		a := &c.langs[i]
		if a.files == 0 {
			continue
		}
		l := lang.By_index(i)
		rep.Languages = append(rep.Languages, Language_stats{
			Name:               l.Name,
			Category:           l.Category.String(),
			Files:              a.files,
			Lines:              a.Lines,
			Code:               a.Code,
			Comment:            a.Comment,
			Blank:              a.Blank,
			Chars:              a.Chars,
			Chars_non_ws:       a.Chars_nonws,
			Bytes:              a.Bytes,
			Percent_lines:      percent(a.Lines, m.Lines),
			Percent_chars:      percent(a.Chars, m.Chars),
			Percent_code:       percent(a.Code, m.Code),
			Avg_lines_per_file: ratio(a.Lines, a.files),
			Avg_chars_per_file: ratio(a.Chars, a.files),
			Index:              i,
		})
		cs := cats[l.Category]
		if cs == nil {
			cs = &Category_stats{Name: l.Category.String()}
			cats[l.Category] = cs
		}
		cs.Files += a.files
		cs.Lines += a.Lines
	}
	slices.SortFunc(rep.Languages, func(a, b Language_stats) int {
		if d := cmp.Compare(b.Lines, a.Lines); d != 0 {
			return d
		}
		if d := cmp.Compare(b.Files, a.Files); d != 0 {
			return d
		}
		return cmp.Compare(a.Name, b.Name)
	})
	for _, cs := range cats {
		cs.Percent = percent(cs.Lines, m.Lines)
		rep.Categories = append(rep.Categories, *cs)
	}
	slices.SortFunc(rep.Categories, func(a, b Category_stats) int {
		if d := cmp.Compare(b.Lines, a.Lines); d != 0 {
			return d
		}
		return cmp.Compare(a.Name, b.Name)
	})
	if rep.Languages == nil {
		rep.Languages = []Language_stats{}
	}
	if rep.Categories == nil {
		rep.Categories = []Category_stats{}
	}
	if rep.Top_by_lines == nil {
		rep.Top_by_lines = []Top_file{}
	}
	if rep.Top_by_bytes == nil {
		rep.Top_by_bytes = []Top_file{}
	}
	return rep
}
