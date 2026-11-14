package lang

import (
	"path"
	"sort"
	"strings"
)

type Category uint8

const (
	Programming Category = iota
	Markup
	Data
	Prose
)

func (c Category) String() string {
	switch c {
	case Programming:
		return "programming"
	case Markup:
		return "markup"
	case Data:
		return "data"
	case Prose:
		return "prose"
	}
	return "other"
}

type Block struct {
	Start string
	End   string
	start []byte
	end   []byte
}

type Language struct {
	Name           string
	Category       Category
	Extensions     []string
	Filenames      []string
	Interpreters   []string
	Line_comments  []string
	Block_comments []Block
	Nested         bool
	Quotes         string
	Index          int

	line_markers [][]byte
	class        [256]uint8
	is_quote     [256]bool
}

const (
	class_plain uint8 = iota
	class_space
	class_marker
)

type spec struct {
	name    string
	cat     Category
	ext     string
	files   string
	shebang string
	line    string
	block   string
	quotes  string
	nested  bool
}

var (
	languages    []*Language
	by_extension = map[string]int{}
	by_filename  = map[string]int{}
	by_shebang   = map[string]int{}
	other        *Language
)

func init() {
	for _, s := range specs() {
		add(s)
	}
	add(spec{name: "Other", cat: Data})
	other = languages[len(languages)-1]
}

func add(s spec) {
	l := &Language{
		Name:         s.name,
		Category:     s.cat,
		Extensions:   strings.Fields(s.ext),
		Filenames:    strings.Fields(s.files),
		Interpreters: strings.Fields(s.shebang),
		Quotes:       s.quotes,
		Nested:       s.nested,
		Index:        len(languages),
	}
	for b, sp := range ascii_space {
		if sp {
			l.class[b] = class_space
		}
	}
	for _, m := range strings.Fields(s.line) {
		l.Line_comments = append(l.Line_comments, m)
		l.line_markers = append(l.line_markers, []byte(m))
		l.class[m[0]] = class_marker
	}
	pair := strings.Fields(s.block)
	for i := 0; i+1 < len(pair); i += 2 {
		b := Block{Start: pair[i], End: pair[i+1], start: []byte(pair[i]), end: []byte(pair[i+1])}
		l.Block_comments = append(l.Block_comments, b)
		l.class[pair[i][0]] = class_marker
	}
	sort.SliceStable(l.Block_comments, func(a, b int) bool {
		return len(l.Block_comments[a].Start) > len(l.Block_comments[b].Start)
	})
	sort.SliceStable(l.line_markers, func(a, b int) bool {
		return len(l.line_markers[a]) > len(l.line_markers[b])
	})
	for i := 0; i < len(s.quotes); i++ {
		l.is_quote[s.quotes[i]] = true
		l.class[s.quotes[i]] = class_marker
	}
	for _, e := range l.Extensions {
		by_extension[e] = l.Index
	}
	for _, f := range l.Filenames {
		by_filename[strings.ToLower(f)] = l.Index
	}
	for _, i := range l.Interpreters {
		by_shebang[i] = l.Index
	}
	languages = append(languages, l)
}

func All() []*Language {
	return languages
}

func Count() int {
	return len(languages)
}

func By_index(i int) *Language {
	if i < 0 || i >= len(languages) {
		return other
	}
	return languages[i]
}

func Other() *Language {
	return other
}

func Detect(name string, head []byte) *Language {
	lower := strings.ToLower(name)
	if i, ok := by_filename[lower]; ok {
		return languages[i]
	}
	if i, ok := prefixed(lower); ok {
		return languages[i]
	}
	if ext := path.Ext(lower); len(ext) > 1 {
		if i, ok := by_extension[ext[1:]]; ok {
			return languages[i]
		}
	}
	if i, ok := from_shebang(head); ok {
		return languages[i]
	}
	return other
}

func prefixed(lower string) (int, bool) {
	switch {
	case strings.HasPrefix(lower, "dockerfile."), strings.HasPrefix(lower, "containerfile."):
		return by_filename["dockerfile"], true
	case strings.HasPrefix(lower, ".env."):
		return by_filename[".env"], true
	case strings.HasPrefix(lower, "makefile."):
		return by_filename["makefile"], true
	}
	return 0, false
}

func from_shebang(head []byte) (int, bool) {
	if len(head) < 3 || head[0] != '#' || head[1] != '!' {
		return 0, false
	}
	if len(head) > 256 {
		head = head[:256]
	}
	line := head[2:]
	for i, b := range line {
		if b == '\n' || b == '\r' {
			line = line[:i]
			break
		}
	}
	fields := strings.Fields(string(line))
	if len(fields) == 0 {
		return 0, false
	}
	interp := path.Base(fields[0])
	if interp == "env" {
		interp = ""
		for _, f := range fields[1:] {
			if strings.HasPrefix(f, "-") || strings.Contains(f, "=") {
				continue
			}
			interp = path.Base(f)
			break
		}
	}
	if interp == "" {
		return 0, false
	}
	if i, ok := by_shebang[interp]; ok {
		return i, true
	}
	trimmed := strings.TrimRight(interp, "0123456789.")
	if i, ok := by_shebang[trimmed]; ok {
		return i, true
	}
	return 0, false
}
