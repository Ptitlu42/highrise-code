package scan

import (
	"slices"
	"strings"

	"github.com/ptitlu42/highrise-code/internal/lang"
)

func finalize(d *Dir) []int64 {
	kept := d.Files[:0]
	for i := range d.Files {
		if d.kept[i] {
			kept = append(kept, d.Files[i])
		}
	}
	clear(d.Files[len(kept):])
	d.Files = slices.Clip(kept)
	d.kept = nil
	slices.SortFunc(d.Files, func(a, b File) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(d.Dirs, func(a, b *Dir) int { return strings.Compare(a.Name, b.Name) })

	weights := make([]int64, lang.Count())
	d.Files_total = int64(len(d.Files))
	for i := range d.Files {
		f := &d.Files[i]
		d.Metrics.Add(&f.Metrics)
		weights[f.Lang] += f.Bytes + 1
	}
	for _, c := range d.Dirs {
		sub := finalize(c)
		d.Metrics.Add(&c.Metrics)
		d.Files_total += c.Files_total
		d.Dirs_total += c.Dirs_total + 1
		for i, w := range sub {
			weights[i] += w
		}
	}
	best := int64(0)
	d.Dominant = uint16(lang.Other().Index)
	for i, w := range weights {
		if w > best {
			best = w
			d.Dominant = uint16(i)
		}
	}
	return weights
}
