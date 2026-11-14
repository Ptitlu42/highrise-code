package ignore

import (
	"strings"
)

const (
	max_line_bytes = 4096
	max_rules      = 20000
)

type rule struct {
	negate   bool
	dir_only bool
	anchored bool
	segments []segment
}

type Rules struct {
	base  string
	rules []rule
}

func Parse(base string, data []byte) *Rules {
	r := &Rules{base: base}
	text := string(data)
	text = strings.TrimPrefix(text, "\xef\xbb\xbf")
	for len(text) > 0 && len(r.rules) < max_rules {
		var line string
		if i := strings.IndexByte(text, '\n'); i < 0 {
			line, text = text, ""
		} else {
			line, text = text[:i], text[i+1:]
		}
		if len(line) > max_line_bytes {
			continue
		}
		if rl, ok := parse_line(line); ok {
			r.rules = append(r.rules, rl)
		}
	}
	return r
}

func parse_line(line string) (rule, bool) {
	line = strings.TrimSuffix(line, "\r")
	end := len(line)
	for end > 0 && line[end-1] == ' ' {
		if end >= 2 && line[end-2] == '\\' {
			break
		}
		end--
	}
	line = line[:end]
	if line == "" || line[0] == '#' {
		return rule{}, false
	}
	var rl rule
	if line[0] == '!' {
		rl.negate = true
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") {
		rl.dir_only = true
		line = strings.TrimRight(line, "/")
	}
	if line == "" {
		return rule{}, false
	}
	if strings.Contains(line, "/") {
		rl.anchored = true
		for _, part := range strings.Split(line, "/") {
			if part == "" {
				continue
			}
			rl.segments = append(rl.segments, compile_segment(part))
		}
		if len(rl.segments) == 0 {
			return rule{}, false
		}
		return rl, true
	}
	sg := compile_segment(line)
	sg.double_star = false
	rl.segments = []segment{sg}
	return rl, true
}

func (r *Rules) Len() int {
	if r == nil {
		return 0
	}
	return len(r.rules)
}

func (r *Rules) relative(rel string) (string, bool) {
	if r.base == "" {
		return rel, true
	}
	if len(rel) > len(r.base) && rel[len(r.base)] == '/' && rel[:len(r.base)] == r.base {
		return rel[len(r.base)+1:], true
	}
	return "", false
}

func (rl *rule) matches(rel string, is_dir bool) bool {
	if rl.dir_only && !is_dir {
		return false
	}
	if !rl.anchored {
		name := rel
		if i := strings.LastIndexByte(rel, '/'); i >= 0 {
			name = rel[i+1:]
		}
		return rl.segments[0].match(name)
	}
	return match_path(rl.segments, rel)
}

func (r *Rules) Decide(rel string, is_dir bool) (ignored, decided bool) {
	if r == nil {
		return false, false
	}
	sub, ok := r.relative(rel)
	if !ok {
		return false, false
	}
	for i := len(r.rules) - 1; i >= 0; i-- {
		if r.rules[i].matches(sub, is_dir) {
			return !r.rules[i].negate, true
		}
	}
	return false, false
}

func (r *Rules) Is_ignored(rel string, is_dir bool) bool {
	ignored, _ := r.Decide(rel, is_dir)
	return ignored
}

type Matcher struct {
	parent *Matcher
	rules  *Rules
}

func New_matcher() *Matcher {
	return &Matcher{}
}

func (m *Matcher) Child(r *Rules) *Matcher {
	if r.Len() == 0 {
		return m
	}
	return &Matcher{parent: m, rules: r}
}

func (m *Matcher) Is_ignored(rel string, is_dir bool) bool {
	for x := m; x != nil; x = x.parent {
		if x.rules == nil {
			continue
		}
		if ignored, decided := x.rules.Decide(rel, is_dir); decided {
			return ignored
		}
	}
	return false
}
