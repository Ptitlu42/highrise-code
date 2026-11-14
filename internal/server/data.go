package server

import (
	"cmp"
	"encoding/json"
	"slices"
	"sync"

	"github.com/ptitlu42/highrise-code/internal/lang"
	"github.com/ptitlu42/highrise-code/internal/layout"
	"github.com/ptitlu42/highrise-code/internal/scan"
	"github.com/ptitlu42/highrise-code/internal/stats"
)

const (
	max_cached_layouts = 4
	max_search_results = 50
	max_detail_langs   = 16
)

type Config struct {
	Result    *scan.Result
	Report    *stats.Report
	Mode      string
	Color_by  string
	Height_by string
	Max_depth int
	Version   string
}

type language_entry struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

type warning_entry struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type options_entry struct {
	Mode       string `json:"mode"`
	Color_by   string `json:"color_by"`
	Height_by  string `json:"height_by"`
	Max_depth  int    `json:"max_depth"`
	Standalone bool   `json:"standalone"`
}

type summary struct {
	Version        string           `json:"version"`
	Root           string           `json:"root"`
	Options        options_entry    `json:"options"`
	Report         *stats.Report    `json:"report"`
	Languages      []language_entry `json:"languages"`
	Warnings       []warning_entry  `json:"warnings"`
	Warnings_total int64            `json:"warnings_total"`
}

type language_count struct {
	Name  string `json:"name"`
	Files int64  `json:"files"`
	Lines int64  `json:"lines"`
}

type node_detail struct {
	Id                 int              `json:"id"`
	Kind               string           `json:"kind"`
	Name               string           `json:"name"`
	Path               string           `json:"path"`
	Depth              int              `json:"depth"`
	Lang_index         int              `json:"lang_index"`
	Language           string           `json:"language,omitempty"`
	Category           string           `json:"category,omitempty"`
	Bytes              int64            `json:"bytes"`
	Lines              int64            `json:"lines"`
	Code               int64            `json:"code"`
	Comment            int64            `json:"comment"`
	Blank              int64            `json:"blank"`
	Chars              int64            `json:"chars"`
	Chars_code         int64            `json:"chars_code"`
	Chars_comment      int64            `json:"chars_comment"`
	Chars_blank        int64            `json:"chars_blank"`
	Chars_non_ws       int64            `json:"chars_non_whitespace"`
	Avg_chars_per_line float64          `json:"average_chars_per_line"`
	Files              int64            `json:"files"`
	Dirs               int64            `json:"dirs"`
	Languages          []language_count `json:"languages,omitempty"`
}

type search_hit struct {
	Id   int    `json:"id"`
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type dir_names struct {
	Ids   []int32  `json:"ids"`
	Names []string `json:"names"`
}

type cached_layout struct {
	lay      *layout.Layout
	encoded  []byte
	dir_json []byte
}

type store struct {
	cfg     Config
	mu      sync.Mutex
	layouts map[int]*cached_layout
	order   []int
}

func new_store(cfg Config) *store {
	return &store{cfg: cfg, layouts: map[int]*cached_layout{}}
}

func (s *store) clamp_depth(d int) int {
	if d < 1 || d > scan.Hard_max_depth {
		d = s.cfg.Max_depth
	}
	if s.cfg.Max_depth > 0 && d > s.cfg.Max_depth {
		d = s.cfg.Max_depth
	}
	return d
}

func (s *store) layout_for(depth int) (*cached_layout, error) {
	depth = s.clamp_depth(depth)
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.layouts[depth]; ok {
		return c, nil
	}
	lay := layout.Compute(s.cfg.Result, depth)
	var buf byteBuffer
	buf.grow(int(layout.Encoded_size(len(lay.Items))))
	if err := lay.Encode(&buf); err != nil {
		return nil, err
	}
	names := dir_names{Ids: []int32{}, Names: []string{}}
	for _, id := range lay.Dir_ids() {
		names.Ids = append(names.Ids, id)
		names.Names = append(names.Names, lay.Name(int(id)))
	}
	dj, err := json.Marshal(names)
	if err != nil {
		return nil, err
	}
	c := &cached_layout{lay: lay, encoded: buf.b, dir_json: dj}
	if len(s.order) >= max_cached_layouts {
		delete(s.layouts, s.order[0])
		s.order = s.order[1:]
	}
	s.layouts[depth] = c
	s.order = append(s.order, depth)
	return c, nil
}

type byteBuffer struct {
	b []byte
}

func (w *byteBuffer) grow(n int) {
	w.b = make([]byte, 0, n)
}

func (w *byteBuffer) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

func build_summary(cfg Config, standalone bool) summary {
	langs := make([]language_entry, 0, lang.Count())
	for _, l := range lang.All() {
		langs = append(langs, language_entry{Name: l.Name, Category: l.Category.String()})
	}
	warnings := make([]warning_entry, 0, len(cfg.Result.Warnings))
	for _, w := range cfg.Result.Warnings {
		warnings = append(warnings, warning_entry{Path: w.Path, Message: w.Message})
	}
	return summary{
		Version: cfg.Version,
		Root:    cfg.Result.Root_name,
		Options: options_entry{
			Mode:       cfg.Mode,
			Color_by:   cfg.Color_by,
			Height_by:  cfg.Height_by,
			Max_depth:  cfg.Max_depth,
			Standalone: standalone,
		},
		Report:         cfg.Report,
		Languages:      langs,
		Warnings:       warnings,
		Warnings_total: cfg.Result.Warnings_total,
	}
}

func describe(lay *layout.Layout, id int) node_detail {
	it := &lay.Items[id]
	m := lay.Metrics(id)
	d := node_detail{
		Id:            id,
		Name:          lay.Name(id),
		Path:          lay.Path(id),
		Depth:         int(it.Depth),
		Bytes:         m.Bytes,
		Lines:         m.Lines,
		Code:          m.Code,
		Comment:       m.Comment,
		Blank:         m.Blank,
		Chars:         m.Chars,
		Chars_code:    m.Chars_code,
		Chars_comment: m.Chars_comment,
		Chars_blank:   m.Chars_blank,
		Chars_non_ws:  m.Chars_nonws,
	}
	if m.Lines > 0 {
		d.Avg_chars_per_line = stats.Round2(float64(m.Chars) / float64(m.Lines))
	}
	switch it.Kind {
	case layout.File_kind:
		d.Kind = "file"
		l := lang.By_index(int(it.Lang))
		d.Language = l.Name
		d.Lang_index = l.Index
		d.Category = l.Category.String()
		d.Files = 1
	default:
		d.Kind = "dir"
		if it.Kind == layout.Aggregate_kind {
			d.Kind = "aggregate"
		}
		d.Files = it.Dir.Files_total
		d.Dirs = it.Dir.Dirs_total
		l := lang.By_index(int(it.Lang))
		d.Language = l.Name
		d.Lang_index = l.Index
		d.Category = l.Category.String()
		d.Languages = language_breakdown(it.Dir)
	}
	return d
}

func language_breakdown(root *scan.Dir) []language_count {
	counts := make([]language_count, lang.Count())
	var walk func(*scan.Dir)
	walk = func(d *scan.Dir) {
		for i := range d.Files {
			f := &d.Files[i]
			counts[f.Lang].Files++
			counts[f.Lang].Lines += f.Lines
		}
		for _, c := range d.Dirs {
			walk(c)
		}
	}
	walk(root)
	var out []language_count
	for i, c := range counts {
		if c.Files > 0 {
			c.Name = lang.By_index(i).Name
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b language_count) int {
		if c := cmp.Compare(b.Lines, a.Lines); c != 0 {
			return c
		}
		if c := cmp.Compare(b.Files, a.Files); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
	if len(out) > max_detail_langs {
		out = out[:max_detail_langs]
	}
	return out
}
