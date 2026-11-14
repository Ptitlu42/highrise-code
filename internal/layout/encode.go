package layout

import (
	"bufio"
	"encoding/binary"
	"io"
	"math"
)

const (
	Magic        = "HRL1"
	Header_bytes = 16
	chunk_items  = 4096
)

func sat32(v int64) uint32 {
	if v < 0 {
		return 0
	}
	if v > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(v)
}

func Encoded_size(n int) int64 {
	return Header_bytes + int64(n)*49
}

func (l *Layout) Encode(w io.Writer) error {
	bw := bufio.NewWriterSize(w, 1<<16)
	n := len(l.Items)
	header := make([]byte, Header_bytes)
	copy(header, Magic)
	binary.LittleEndian.PutUint32(header[4:], uint32(n))
	binary.LittleEndian.PutUint32(header[8:], math.Float32bits(l.World))
	binary.LittleEndian.PutUint32(header[12:], uint32(l.Depth_reached))
	if _, err := bw.Write(header); err != nil {
		return err
	}
	scratch := make([]byte, chunk_items*4)

	write_f32 := func(get func(*Item) float32) error {
		for start := 0; start < n; start += chunk_items {
			end := min(start+chunk_items, n)
			buf := scratch[:(end-start)*4]
			for i := start; i < end; i++ {
				binary.LittleEndian.PutUint32(buf[(i-start)*4:], math.Float32bits(get(&l.Items[i])))
			}
			if _, err := bw.Write(buf); err != nil {
				return err
			}
		}
		return nil
	}
	write_u32 := func(get func(int) uint32) error {
		for start := 0; start < n; start += chunk_items {
			end := min(start+chunk_items, n)
			buf := scratch[:(end-start)*4]
			for i := start; i < end; i++ {
				binary.LittleEndian.PutUint32(buf[(i-start)*4:], get(i))
			}
			if _, err := bw.Write(buf); err != nil {
				return err
			}
		}
		return nil
	}
	write_u16 := func(get func(*Item) uint16) error {
		for start := 0; start < n; start += chunk_items {
			end := min(start+chunk_items, n)
			buf := scratch[:(end-start)*2]
			for i := start; i < end; i++ {
				binary.LittleEndian.PutUint16(buf[(i-start)*2:], get(&l.Items[i]))
			}
			if _, err := bw.Write(buf); err != nil {
				return err
			}
		}
		return nil
	}

	steps := []func() error{
		func() error { return write_f32(func(it *Item) float32 { return it.X }) },
		func() error { return write_f32(func(it *Item) float32 { return it.Y }) },
		func() error { return write_f32(func(it *Item) float32 { return it.W }) },
		func() error { return write_f32(func(it *Item) float32 { return it.H }) },
		func() error { return write_u32(func(i int) uint32 { return sat32(l.Metrics(i).Lines) }) },
		func() error { return write_u32(func(i int) uint32 { return sat32(l.Metrics(i).Code) }) },
		func() error { return write_u32(func(i int) uint32 { return sat32(l.Metrics(i).Comment) }) },
		func() error { return write_u32(func(i int) uint32 { return sat32(l.Metrics(i).Bytes) }) },
		func() error { return write_u32(func(i int) uint32 { return sat32(l.Metrics(i).Chars) }) },
		func() error { return write_u32(func(i int) uint32 { return sat32(l.Metrics(i).Chars_nonws) }) },
		func() error { return write_u32(func(i int) uint32 { return uint32(l.Items[i].Parent) }) },
		func() error { return write_u16(func(it *Item) uint16 { return it.Lang }) },
		func() error { return write_u16(func(it *Item) uint16 { return it.Depth }) },
		func() error {
			for start := 0; start < n; start += chunk_items {
				end := min(start+chunk_items, n)
				buf := scratch[:end-start]
				for i := start; i < end; i++ {
					buf[i-start] = byte(l.Items[i].Kind)
				}
				if _, err := bw.Write(buf); err != nil {
					return err
				}
			}
			return nil
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return bw.Flush()
}
