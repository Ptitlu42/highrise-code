package ignore

import "testing"

type probe struct {
	path   string
	is_dir bool
	want   bool
}

func run_probes(t *testing.T, base string, patterns string, probes []probe) {
	t.Helper()
	r := Parse(base, []byte(patterns))
	for _, p := range probes {
		got := r.Is_ignored(p.path, p.is_dir)
		if got != p.want {
			t.Errorf("patterns %q path %q dir=%v: got %v want %v", patterns, p.path, p.is_dir, got, p.want)
		}
	}
}

func Test_basic_patterns(t *testing.T) {
	run_probes(t, "", "*.log\nbuild/\n/root_only.txt\ndocs/*.md\n", []probe{
		{"a.log", false, true},
		{"x/y/a.log", false, true},
		{"a.txt", false, false},
		{"build", true, true},
		{"build", false, false},
		{"src/build", true, true},
		{"root_only.txt", false, true},
		{"sub/root_only.txt", false, false},
		{"docs/readme.md", false, true},
		{"docs/sub/readme.md", false, false},
		{"other/docs/readme.md", false, false},
	})
}

func Test_negation_and_order(t *testing.T) {
	run_probes(t, "", "*.txt\n!keep.txt\n", []probe{
		{"a.txt", false, true},
		{"keep.txt", false, false},
		{"d/keep.txt", false, false},
	})
	run_probes(t, "", "!keep.txt\n*.txt\n", []probe{
		{"keep.txt", false, true},
	})
}

func Test_double_star(t *testing.T) {
	run_probes(t, "", "**/foo\nbar/**\na/**/b\n", []probe{
		{"foo", false, true},
		{"x/y/foo", false, true},
		{"bar/x", false, true},
		{"bar/x/y", false, true},
		{"bar", true, false},
		{"a/b", false, true},
		{"a/x/b", false, true},
		{"a/x/y/b", false, true},
		{"a/x/c", false, false},
	})
}

func Test_escapes_and_comments(t *testing.T) {
	run_probes(t, "", "# comment\n\\#hash\n\\!bang\nspace\\ \ntrail   \n\n", []probe{
		{"#hash", false, true},
		{"!bang", false, true},
		{"space ", false, true},
		{"space", false, false},
		{"trail", false, true},
		{"comment", false, false},
	})
}

func Test_classes_and_wildcards(t *testing.T) {
	run_probes(t, "", "[abc].txt\n[!x]y.go\nfile?.c\n[[:digit:]]*.dat\n*.[oa]\n", []probe{
		{"a.txt", false, true},
		{"d.txt", false, false},
		{"zy.go", false, true},
		{"xy.go", false, false},
		{"file1.c", false, true},
		{"file12.c", false, false},
		{"9data.dat", false, true},
		{"data.dat", false, false},
		{"lib.a", false, true},
		{"lib.o", false, true},
		{"lib.c", false, false},
	})
}

func Test_base_directory(t *testing.T) {
	run_probes(t, "sub/dir", "*.tmp\n/anchored\n", []probe{
		{"sub/dir/a.tmp", false, true},
		{"sub/dir/anchored", false, true},
		{"sub/dir/x/anchored", false, false},
		{"other/a.tmp", false, false},
	})
}

func Test_crlf_and_bom(t *testing.T) {
	r := Parse("", []byte("\xef\xbb\xbf*.bak\r\nnode_modules/\r\n"))
	if !r.Is_ignored("a.bak", false) || !r.Is_ignored("node_modules", true) {
		t.Fatal("crlf or bom handling broken")
	}
}

func Test_matcher_layers(t *testing.T) {
	root := Parse("", []byte("*.log\n"))
	sub := Parse("sub", []byte("!important.log\n/local\n"))
	m := New_matcher().Child(root).Child(sub)
	cases := []probe{
		{"a.log", false, true},
		{"sub/a.log", false, true},
		{"sub/important.log", false, false},
		{"sub/local", false, true},
		{"sub/deep/local", false, false},
	}
	for _, c := range cases {
		if got := m.Is_ignored(c.path, c.is_dir); got != c.want {
			t.Errorf("path %q: got %v want %v", c.path, got, c.want)
		}
	}
}

func Test_degenerate_patterns(t *testing.T) {
	r := Parse("", []byte("/\n//\n!\n[\n[]\n\\\n"))
	_ = r.Is_ignored("anything", false)
	_ = r.Is_ignored("[", false)
}

func Benchmark_matcher(b *testing.B) {
	r := Parse("", []byte("*.o\n*.a\nbuild/\n/out\ndocs/**/*.tmp\nnode_modules/\n.cache\n"))
	m := New_matcher().Child(r)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.Is_ignored("src/pkg/module/file.go", false)
	}
}
