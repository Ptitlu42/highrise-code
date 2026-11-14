package layout

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ptitlu42/highrise-code/internal/scan"
)

func build(t *testing.T, files map[string]string) *scan.Result {
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

func sample_files() map[string]string {
	files := map[string]string{}
	for i := 0; i < 40; i++ {
		files[fmt.Sprintf("d%d/f%d.go", i%5, i)] = strings.Repeat("x := 1\n", i+1)
		files[fmt.Sprintf("d%d/sub%d/g%d.py", i%5, i%3, i)] = strings.Repeat("y = 2\n", 2*i+1)
	}
	files["top.c"] = "int x;\n"
	return files
}

func inside(child, parent *Item, eps float32) bool {
	return child.X >= parent.X-eps && child.Y >= parent.Y-eps &&
		child.X+child.W <= parent.X+parent.W+eps && child.Y+child.H <= parent.Y+parent.H+eps
}

func overlap(a, b *Item) bool {
	const eps = 1e-3
	return a.X < b.X+b.W-eps && b.X < a.X+a.W-eps && a.Y < b.Y+b.H-eps && b.Y < a.Y+a.H-eps
}

func Test_layout_invariants(t *testing.T) {
	res := build(t, sample_files())
	l := Compute(res, 256)
	files := 0
	for i := range l.Items {
		it := &l.Items[i]
		if it.W <= 0 || it.H <= 0 {
			t.Fatalf("item %d has empty rect", i)
		}
		if it.Kind == File_kind {
			files++
		}
		if i == 0 {
			if it.Parent != -1 || it.Kind != Dir_kind {
				t.Fatalf("bad root")
			}
			continue
		}
		p := &l.Items[it.Parent]
		if !inside(it, p, 1e-2) {
			t.Fatalf("item %d (%s) outside parent", i, l.Path(i))
		}
		if int(it.Parent) >= i {
			t.Fatalf("parent must precede child")
		}
	}
	if int64(files) != res.Root.Files_total {
		t.Fatalf("files=%d total=%d", files, res.Root.Files_total)
	}
	children := map[int32][]int{}
	for i := 1; i < len(l.Items); i++ {
		children[l.Items[i].Parent] = append(children[l.Items[i].Parent], i)
	}
	for _, c := range children {
		for a := 0; a < len(c); a++ {
			for b := a + 1; b < len(c); b++ {
				if overlap(&l.Items[c[a]], &l.Items[c[b]]) {
					t.Fatalf("siblings %s and %s overlap", l.Path(c[a]), l.Path(c[b]))
				}
			}
		}
	}
}

func Test_area_proportional(t *testing.T) {
	res := build(t, map[string]string{
		"a.txt": strings.Repeat("x", 3000),
		"b.txt": strings.Repeat("x", 1000),
	})
	l := Compute(res, 256)
	var area_a, area_b float64
	for i := range l.Items {
		it := &l.Items[i]
		if it.Kind != File_kind {
			continue
		}
		a := float64(it.W) * float64(it.H)
		if l.Name(i) == "a.txt" {
			area_a = a
		} else {
			area_b = a
		}
	}
	if r := area_a / area_b; r < 2.3 || r > 3.7 {
		t.Errorf("area ratio %v", r)
	}
}

func Test_max_depth_collapses(t *testing.T) {
	res := build(t, map[string]string{"a/b/c/d.go": "x\n", "a/e.go": "x\n", "top.go": "x\n"})
	l := Compute(res, 2)
	aggregates := 0
	for i := range l.Items {
		if l.Items[i].Kind == Aggregate_kind {
			aggregates++
			if l.Path(i) != "a/b" {
				t.Errorf("unexpected aggregate %q", l.Path(i))
			}
		}
		if l.Items[i].Kind == File_kind && l.Name(i) == "d.go" {
			t.Errorf("collapsed file leaked")
		}
	}
	if aggregates != 1 {
		t.Errorf("aggregates=%d", aggregates)
	}
	full := Compute(res, 256)
	if len(full.Items) <= len(l.Items) {
		t.Errorf("collapse must reduce items")
	}
}

func Test_deterministic(t *testing.T) {
	res := build(t, sample_files())
	var a, b bytes.Buffer
	if err := Compute(res, 256).Encode(&a); err != nil {
		t.Fatal(err)
	}
	if err := Compute(res, 256).Encode(&b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Error("layout not deterministic")
	}
}

func Test_encode_format(t *testing.T) {
	res := build(t, sample_files())
	l := Compute(res, 256)
	var buf bytes.Buffer
	if err := l.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	n := len(l.Items)
	if int64(buf.Len()) != Encoded_size(n) {
		t.Fatalf("size=%d want %d", buf.Len(), Encoded_size(n))
	}
	data := buf.Bytes()
	if string(data[:4]) != Magic {
		t.Fatal("magic")
	}
	if int(binary.LittleEndian.Uint32(data[4:])) != n {
		t.Fatal("count")
	}
	if math.Float32frombits(binary.LittleEndian.Uint32(data[8:])) != l.World {
		t.Fatal("world")
	}
	x_off := Header_bytes
	if got := math.Float32frombits(binary.LittleEndian.Uint32(data[x_off+4*3:])); got != l.Items[3].X {
		t.Errorf("x[3]=%v want %v", got, l.Items[3].X)
	}
	lines_off := Header_bytes + 16*n
	if got := binary.LittleEndian.Uint32(data[lines_off:]); int64(got) != res.Root.Lines {
		t.Errorf("lines[0]=%d want %d", got, res.Root.Lines)
	}
	parent_off := Header_bytes + 40*n
	if got := int32(binary.LittleEndian.Uint32(data[parent_off:])); got != -1 {
		t.Errorf("parent[0]=%d", got)
	}
	kind_off := Header_bytes + 48*n
	if data[kind_off] != byte(Dir_kind) {
		t.Errorf("kind[0]=%d", data[kind_off])
	}
}

func Test_search(t *testing.T) {
	res := build(t, map[string]string{"src/Main.go": "x\n", "src/util/helper.go": "x\n", "README.md": "x\n"})
	l := Compute(res, 256)
	if got := l.Search("main", 10); len(got) != 1 || l.Name(int(got[0])) != "Main.go" {
		t.Errorf("name search %v", got)
	}
	if got := l.Search("util/hel", 10); len(got) != 1 || l.Path(int(got[0])) != "src/util/helper.go" {
		t.Errorf("path search %v", got)
	}
	if got := l.Search(".go", 1); len(got) != 1 {
		t.Errorf("limit not honored")
	}
	if l.Search("", 5) != nil {
		t.Errorf("empty query")
	}
}

func Test_empty_tree(t *testing.T) {
	res := build(t, map[string]string{})
	l := Compute(res, 256)
	if len(l.Items) != 1 {
		t.Errorf("items=%d", len(l.Items))
	}
	var buf bytes.Buffer
	if err := l.Encode(&buf); err != nil {
		t.Fatal(err)
	}
}
