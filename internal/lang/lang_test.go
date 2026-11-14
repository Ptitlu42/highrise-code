package lang

import "testing"

func Test_language_count(t *testing.T) {
	if Count() < 61 {
		t.Fatalf("expected at least 60 languages, got %d", Count())
	}
	seen := map[string]bool{}
	for _, l := range All() {
		if seen[l.Name] {
			t.Errorf("duplicate language %q", l.Name)
		}
		seen[l.Name] = true
		if l.Index != indexof(l) {
			t.Errorf("bad index for %q", l.Name)
		}
	}
}

func indexof(l *Language) int {
	for i, x := range All() {
		if x == l {
			return i
		}
	}
	return -1
}

func Test_detect(t *testing.T) {
	cases := []struct {
		name string
		head string
		want string
	}{
		{"main.go", "", "Go"},
		{"Makefile", "", "Makefile"},
		{"makefile", "", "Makefile"},
		{"Dockerfile", "", "Dockerfile"},
		{"Dockerfile.dev", "", "Dockerfile"},
		{"PKGBUILD", "", "PKGBUILD"},
		{"CMakeLists.txt", "", "CMake"},
		{"notes.txt", "", "Plain Text"},
		{"script", "#!/usr/bin/env python3\n", "Python"},
		{"script", "#!/bin/bash\n", "Shell"},
		{"script", "#!/usr/bin/env -S node --flag\n", "JavaScript"},
		{"tool", "#!/usr/bin/python3.11\n", "Python"},
		{"UPPER.PY", "", "Python"},
		{"data.bin2", "", "Other"},
		{"noext", "plain", "Other"},
		{".bashrc", "", "Shell"},
		{".env.local", "", "Dotenv"},
		{"a.d.ts", "", "TypeScript"},
	}
	for _, c := range cases {
		if got := Detect(c.name, []byte(c.head)).Name; got != c.want {
			t.Errorf("Detect(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func classify_lines(l *Language, lines ...string) string {
	var c Classifier
	c.Reset(l)
	out := ""
	for _, ln := range lines {
		blank := true
		for i := 0; i < len(ln); i++ {
			if !ascii_space[ln[i]] {
				blank = false
			}
		}
		if blank {
			out += "b"
			continue
		}
		code, comment := c.Classify([]byte(ln))
		if comment && !code {
			out += "m"
		} else {
			out += "c"
		}
	}
	return out
}

func Test_classify_c_like(t *testing.T) {
	l := Detect("a.go", nil)
	got := classify_lines(l,
		"package x",
		"",
		"// comment",
		"x := 1 // trailing",
		"/* block start",
		"still comment",
		"",
		"end */",
		"y := 2 /* inline */ + 3",
		"s := \"/* not a comment\"",
		"z := 3",
		"/* a */ w := 4",
	)
	if got != "cbmcmmbmccc"+"c" {
		t.Errorf("got %s", got)
	}
}

func Test_classify_nested(t *testing.T) {
	l := Detect("a.rs", nil)
	got := classify_lines(l, "/* a /* b */ still */", "code", "/* open", "/* inner */", "close */", "code")
	if got != "mcmmmc" {
		t.Errorf("got %s", got)
	}
}

func Test_classify_hash_and_none(t *testing.T) {
	if got := classify_lines(Detect("a.py", nil), "# c", "x = 1", "  # c2"); got != "mcm" {
		t.Errorf("python got %s", got)
	}
	if got := classify_lines(Detect("a.json", nil), "{", "\"a\": 1", "}"); got != "ccc" {
		t.Errorf("json got %s", got)
	}
}

func Test_classify_html_and_lua(t *testing.T) {
	if got := classify_lines(Detect("a.html", nil), "<!-- c -->", "<p>x</p>", "<!--", "x", "-->", "<b/>"); got != "mcmmmc" {
		t.Errorf("html got %s", got)
	}
	if got := classify_lines(Detect("a.lua", nil), "--[[ a", "b ]]", "x = 1", "-- c"); got != "mmcm" {
		t.Errorf("lua got %s", got)
	}
}
