package layout

import (
	"cmp"
	"math"
	"slices"

	"github.com/ptitlu42/highrise-code/internal/scan"
)

type Kind uint8

const (
	Dir_kind Kind = iota
	File_kind
	Aggregate_kind
)

const (
	min_world        = 400
	max_world        = 20000
	world_per_root   = 6
	district_pad     = 0.05
	district_pad_cap = 0.006
	building_gap     = 0.08
	building_gap_cap = 0.002
)

type Item struct {
	X, Y, W, H float32
	Parent     int32
	File_index int32
	Dir        *scan.Dir
	Depth      uint16
	Kind       Kind
	Lang       uint16
}

type Layout struct {
	Items         []Item
	World         float32
	Max_depth     int
	Depth_reached int
	Root_name     string
}

type rect struct {
	x, y, w, h float64
}

type entry struct {
	weight float64
	dir    *scan.Dir
	file   int32
}

type builder struct {
	items     []Item
	max_depth int
	world     float64
	reached   int
}

func World_size(files int64) float64 {
	return math.Min(max_world, math.Max(min_world, math.Sqrt(float64(files))*world_per_root))
}

func Compute(res *scan.Result, max_depth int) *Layout {
	if max_depth < 1 || max_depth > scan.Hard_max_depth {
		max_depth = scan.Hard_max_depth
	}
	root := res.Root
	world := World_size(root.Files_total)
	b := &builder{
		items:     make([]Item, 0, int(root.Files_total+root.Dirs_total+1)),
		max_depth: max_depth,
		world:     world,
	}
	b.place_dir(root, rect{0, 0, world, world}, -1)
	return &Layout{
		Items:         b.items,
		World:         float32(world),
		Max_depth:     max_depth,
		Depth_reached: b.reached,
		Root_name:     res.Root_name,
	}
}

func dir_weight(d *scan.Dir) float64 {
	return float64(d.Bytes + d.Files_total)
}

func (b *builder) emit(r rect, parent int32, d *scan.Dir, file int32, kind Kind, depth int, lang uint16) int32 {
	idx := int32(len(b.items))
	b.items = append(b.items, Item{
		X: float32(r.x), Y: float32(r.y), W: float32(r.w), H: float32(r.h),
		Parent: parent, File_index: file, Dir: d, Depth: uint16(depth), Kind: kind, Lang: lang,
	})
	if depth > b.reached {
		b.reached = depth
	}
	return idx
}

func (b *builder) place_dir(d *scan.Dir, r rect, parent int32) {
	if d.Depth >= b.max_depth && d.Parent != nil {
		b.emit(r, parent, d, -1, Aggregate_kind, d.Depth, d.Dominant)
		return
	}
	idx := b.emit(r, parent, d, -1, Dir_kind, d.Depth, d.Dominant)
	entries := make([]entry, 0, len(d.Files)+len(d.Dirs))
	for i := range d.Files {
		entries = append(entries, entry{weight: float64(d.Files[i].Bytes + 1), file: int32(i)})
	}
	for _, c := range d.Dirs {
		if w := dir_weight(c); c.Files_total > 0 && w > 0 {
			entries = append(entries, entry{weight: w, dir: c, file: -1})
		}
	}
	if len(entries) == 0 {
		return
	}
	slices.SortStableFunc(entries, func(a, c entry) int { return cmp.Compare(c.weight, a.weight) })
	pad := math.Min(math.Min(r.w, r.h)*district_pad, b.world*district_pad_cap)
	inner := rect{r.x + pad, r.y + pad, r.w - 2*pad, r.h - 2*pad}
	squarify(entries, inner, func(e entry, cell rect) {
		if e.dir != nil {
			b.place_dir(e.dir, cell, idx)
			return
		}
		gap := math.Min(math.Min(cell.w, cell.h)*building_gap, b.world*building_gap_cap)
		cell = rect{cell.x + gap, cell.y + gap, cell.w - 2*gap, cell.h - 2*gap}
		f := &d.Files[e.file]
		b.emit(cell, idx, d, e.file, File_kind, d.Depth+1, f.Lang)
	})
}

func worst_ratio(sum, max_area, min_area, side float64) float64 {
	s2 := sum * sum
	side2 := side * side
	return math.Max(side2*max_area/s2, s2/(side2*min_area))
}

func squarify(entries []entry, r rect, emit func(entry, rect)) {
	var total float64
	for _, e := range entries {
		total += e.weight
	}
	if total <= 0 || r.w <= 0 || r.h <= 0 {
		return
	}
	scale := r.w * r.h / total
	i := 0
	n := len(entries)
	for i < n {
		side := math.Min(r.w, r.h)
		if side <= 0 {
			side = 1e-9
		}
		first_area := entries[i].weight * scale
		sum := first_area
		j := i + 1
		worst := worst_ratio(sum, first_area, first_area, side)
		for j < n {
			a := entries[j].weight * scale
			next_sum := sum + a
			next_worst := worst_ratio(next_sum, first_area, a, side)
			if next_worst > worst {
				break
			}
			sum = next_sum
			worst = next_worst
			j++
		}
		if j == n {
			sum = math.Max(r.w*r.h, 1e-12)
		}
		thickness := sum / side
		if r.w >= r.h {
			pos := r.y
			for k := i; k < j; k++ {
				h := entries[k].weight * scale / thickness
				if k == j-1 {
					h = r.y + r.h - pos
				}
				emit(entries[k], rect{r.x, pos, thickness, h})
				pos += h
			}
			r.x += thickness
			r.w -= thickness
		} else {
			pos := r.x
			for k := i; k < j; k++ {
				w := entries[k].weight * scale / thickness
				if k == j-1 {
					w = r.x + r.w - pos
				}
				emit(entries[k], rect{pos, r.y, w, thickness})
				pos += w
			}
			r.y += thickness
			r.h -= thickness
		}
		i = j
	}
}

func (l *Layout) Metrics(i int) *scan.Metrics {
	it := &l.Items[i]
	if it.Kind == File_kind {
		return &it.Dir.Files[it.File_index].Metrics
	}
	return &it.Dir.Metrics
}

func (l *Layout) Name(i int) string {
	it := &l.Items[i]
	if it.Kind == File_kind {
		return it.Dir.Files[it.File_index].Name
	}
	if it.Dir.Parent == nil {
		return l.Root_name
	}
	return it.Dir.Name
}

func (l *Layout) Path(i int) string {
	it := &l.Items[i]
	if it.Kind == File_kind {
		return it.Dir.File_path(&it.Dir.Files[it.File_index])
	}
	return it.Dir.Path()
}
