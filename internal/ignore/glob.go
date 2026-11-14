package ignore

import "strings"

type token_kind uint8

const (
	tok_literal token_kind = iota
	tok_any
	tok_star
	tok_set
)

type token struct {
	kind token_kind
	b    byte
	set  *[256]bool
}

type segment struct {
	double_star bool
	is_literal  bool
	literal     string
	suffix      string
	has_suffix  bool
	tokens      []token
}

func compile_segment(p string) segment {
	if p == "**" {
		return segment{double_star: true, tokens: []token{{kind: tok_star}}}
	}
	var tokens []token
	i := 0
	for i < len(p) {
		c := p[i]
		switch c {
		case '\\':
			if i+1 < len(p) {
				i++
				tokens = append(tokens, token{kind: tok_literal, b: p[i]})
			} else {
				tokens = append(tokens, token{kind: tok_literal, b: '\\'})
			}
			i++
		case '*':
			for i < len(p) && p[i] == '*' {
				i++
			}
			tokens = append(tokens, token{kind: tok_star})
		case '?':
			tokens = append(tokens, token{kind: tok_any})
			i++
		case '[':
			set, next, ok := parse_class(p, i)
			if !ok {
				tokens = append(tokens, token{kind: tok_literal, b: '['})
				i++
				break
			}
			tokens = append(tokens, token{kind: tok_set, set: set})
			i = next
		default:
			tokens = append(tokens, token{kind: tok_literal, b: c})
			i++
		}
	}
	return finish_segment(tokens)
}

func finish_segment(tokens []token) segment {
	all_literal := true
	for _, t := range tokens {
		if t.kind != tok_literal {
			all_literal = false
			break
		}
	}
	if all_literal {
		var sb strings.Builder
		for _, t := range tokens {
			sb.WriteByte(t.b)
		}
		return segment{is_literal: true, literal: sb.String(), tokens: tokens}
	}
	if len(tokens) > 1 && tokens[0].kind == tok_star {
		rest := true
		var sb strings.Builder
		for _, t := range tokens[1:] {
			if t.kind != tok_literal {
				rest = false
				break
			}
			sb.WriteByte(t.b)
		}
		if rest {
			return segment{has_suffix: true, suffix: sb.String(), tokens: tokens}
		}
	}
	return segment{tokens: tokens}
}

var posix_classes = map[string]func(byte) bool{
	"alpha": func(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' },
	"digit": func(c byte) bool { return c >= '0' && c <= '9' },
	"alnum": func(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' },
	"upper": func(c byte) bool { return c >= 'A' && c <= 'Z' },
	"lower": func(c byte) bool { return c >= 'a' && c <= 'z' },
	"space": func(c byte) bool { return c == ' ' || c >= '\t' && c <= '\r' },
	"blank": func(c byte) bool { return c == ' ' || c == '\t' },
	"punct": func(c byte) bool {
		return c > ' ' && c < 0x7f && !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9')
	},
	"cntrl":  func(c byte) bool { return c < ' ' || c == 0x7f },
	"graph":  func(c byte) bool { return c > ' ' && c < 0x7f },
	"print":  func(c byte) bool { return c >= ' ' && c < 0x7f },
	"xdigit": func(c byte) bool { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' },
}

func parse_class(p string, start int) (*[256]bool, int, bool) {
	i := start + 1
	negate := false
	if i < len(p) && (p[i] == '!' || p[i] == '^') {
		negate = true
		i++
	}
	var set [256]bool
	first := true
	for i < len(p) {
		c := p[i]
		if c == ']' && !first {
			if negate {
				for k := range set {
					set[k] = !set[k]
				}
				set['/'] = false
			}
			return &set, i + 1, true
		}
		first = false
		if c == '[' && i+1 < len(p) && p[i+1] == ':' {
			end := strings.Index(p[i+2:], ":]")
			if end >= 0 {
				if fn, ok := posix_classes[p[i+2:i+2+end]]; ok {
					for k := 0; k < 256; k++ {
						if fn(byte(k)) {
							set[k] = true
						}
					}
					i += end + 4
					continue
				}
			}
		}
		if c == '\\' && i+1 < len(p) {
			i++
			c = p[i]
		}
		if i+2 < len(p) && p[i+1] == '-' && p[i+2] != ']' {
			hi := p[i+2]
			step := 3
			if hi == '\\' && i+3 < len(p) {
				hi = p[i+3]
				step = 4
			}
			for k := int(c); k <= int(hi); k++ {
				set[k] = true
			}
			i += step
			continue
		}
		set[c] = true
		i++
	}
	return nil, 0, false
}

func (s *segment) match(name string) bool {
	switch {
	case s.is_literal:
		return name == s.literal
	case s.has_suffix:
		return strings.HasSuffix(name, s.suffix)
	}
	return glob_match(s.tokens, name)
}

func glob_match(t []token, s string) bool {
	ti, si := 0, 0
	star_t, star_s := -1, 0
	for si < len(s) {
		if ti < len(t) {
			tk := t[ti]
			switch tk.kind {
			case tok_star:
				star_t = ti
				star_s = si
				ti++
				continue
			case tok_any:
				if s[si] != '/' {
					ti++
					si++
					continue
				}
			case tok_literal:
				if s[si] == tk.b {
					ti++
					si++
					continue
				}
			case tok_set:
				if tk.set[s[si]] {
					ti++
					si++
					continue
				}
			}
		}
		if star_t >= 0 {
			star_s++
			si = star_s
			ti = star_t + 1
			continue
		}
		return false
	}
	for ti < len(t) && t[ti].kind == tok_star {
		ti++
	}
	return ti == len(t)
}

func match_path(segs []segment, path string) bool {
	for len(segs) > 0 {
		sg := &segs[0]
		if sg.double_star {
			if len(segs) == 1 {
				return path != ""
			}
			rest := segs[1:]
			for {
				if match_path(rest, path) {
					return true
				}
				i := strings.IndexByte(path, '/')
				if i < 0 {
					return false
				}
				path = path[i+1:]
			}
		}
		if path == "" {
			return false
		}
		var part string
		if i := strings.IndexByte(path, '/'); i < 0 {
			part, path = path, ""
		} else {
			part, path = path[:i], path[i+1:]
		}
		if !sg.match(part) {
			return false
		}
		segs = segs[1:]
	}
	return path == ""
}
