package scan

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ptitlu42/highrise-code/internal/ignore"
	"github.com/ptitlu42/highrise-code/internal/lang"
)

const (
	batch_size       = 256
	max_ignore_bytes = 1 << 20
	chunk_size       = 64 << 10
	binary_probe     = 8 << 10
	shebang_probe    = 256
)

var default_ignored = []string{".git", "node_modules/", "vendor/", "dist/", "build/", "target/", "__pycache__/"}

type Options struct {
	Root           string
	Ignore         []string
	Include_hidden bool
	No_gitignore   bool
	Max_files      int64
	Max_file_size  int64
	Max_depth      int
	Workers        int
	Progress       *atomic.Int64
}

type task struct {
	dir     *Dir
	rel     string
	matcher *ignore.Matcher
	first   int
	last    int
	is_dir  bool
}

type state struct {
	ctx       context.Context
	opts      Options
	root      *os.Root
	forced    *ignore.Rules
	pending   int
	tasks     []task
	mu        sync.Mutex
	cond      *sync.Cond
	files     atomic.Int64
	truncated atomic.Bool
	warn_mu   sync.Mutex
	warnings  []Warning
	warn_n    int64
	skip_mu   sync.Mutex
	skipped   Skipped
}

type worker struct {
	s       *state
	buf     []byte
	counter counter
	skipped Skipped
}

func Run(ctx context.Context, opts Options) (*Result, error) {
	opts = normalize(opts)
	start := time.Now()
	abs, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s: not a directory", opts.Root)
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	s := &state{ctx: ctx, opts: opts, root: root}
	s.cond = sync.NewCond(&s.mu)
	patterns := append(slices.Clone(default_ignored), opts.Ignore...)
	s.forced = ignore.Parse("", []byte(strings.Join(patterns, "\n")))

	top := &Dir{Name: "", Depth: 0}
	s.tasks = append(s.tasks, task{dir: top, rel: "", matcher: ignore.New_matcher(), is_dir: true})
	s.pending = 1

	stop := context.AfterFunc(ctx, func() {
		s.mu.Lock()
		s.cond.Broadcast()
		s.mu.Unlock()
	})
	defer stop()

	var wg sync.WaitGroup
	workers := make([]*worker, opts.Workers)
	for i := range workers {
		workers[i] = &worker{s: s, buf: make([]byte, chunk_size+4)}
		wg.Add(1)
		go func(w *worker) {
			defer wg.Done()
			w.loop()
		}(workers[i])
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, w := range workers {
		s.merge_skipped(&w.skipped)
	}
	finalize(top)
	name := filepath.Base(abs)
	if name == "." || name == string(filepath.Separator) {
		name = abs
	}
	return &Result{
		Root:           top,
		Root_path:      abs,
		Root_name:      name,
		Skipped:        s.skipped,
		Warnings:       s.warnings,
		Warnings_total: s.warn_n,
		Truncated:      s.truncated.Load(),
		Duration:       time.Since(start),
	}, nil
}

func normalize(o Options) Options {
	if o.Max_files <= 0 {
		o.Max_files = Default_max_files
	}
	if o.Max_file_size <= 0 {
		o.Max_file_size = Default_max_filesize
	}
	if o.Max_depth <= 0 || o.Max_depth > Hard_max_depth {
		o.Max_depth = Hard_max_depth
	}
	if o.Workers <= 0 {
		o.Workers = runtime.NumCPU()
	}
	if o.Workers > Hard_max_workers {
		o.Workers = Hard_max_workers
	}
	return o
}

func (s *state) push(ts []task) {
	if len(ts) == 0 {
		return
	}
	s.mu.Lock()
	s.tasks = append(s.tasks, ts...)
	s.pending += len(ts)
	s.mu.Unlock()
	if len(ts) == 1 {
		s.cond.Signal()
	} else {
		s.cond.Broadcast()
	}
}

func (s *state) pop() (task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.tasks) == 0 && s.pending > 0 {
		s.cond.Wait()
	}
	if len(s.tasks) == 0 {
		return task{}, false
	}
	t := s.tasks[len(s.tasks)-1]
	s.tasks = s.tasks[:len(s.tasks)-1]
	return t, true
}

func (s *state) done() {
	s.mu.Lock()
	s.pending--
	if s.pending == 0 {
		s.cond.Broadcast()
	}
	s.mu.Unlock()
}

func (s *state) warn(path, message string) {
	s.warn_mu.Lock()
	s.warn_n++
	if len(s.warnings) < Max_warnings {
		s.warnings = append(s.warnings, Warning{Path: path, Message: message})
	}
	s.warn_mu.Unlock()
}

func (s *state) merge_skipped(o *Skipped) {
	s.skip_mu.Lock()
	s.skipped.Symlinks += o.Symlinks
	s.skipped.Special += o.Special
	s.skipped.Binary += o.Binary
	s.skipped.Too_large += o.Too_large
	s.skipped.Ignored += o.Ignored
	s.skipped.Hidden += o.Hidden
	s.skipped.Unreadable += o.Unreadable
	s.skipped.Depth_limit += o.Depth_limit
	s.skip_mu.Unlock()
}

func (w *worker) loop() {
	for {
		t, ok := w.s.pop()
		if !ok {
			return
		}
		if w.s.ctx.Err() == nil {
			if t.is_dir {
				w.run_dir(t)
			} else {
				w.run_batch(t)
			}
		}
		w.s.done()
	}
}

func reason(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

func join_rel(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

func (s *state) open_dir(rel string) (*os.Root, error) {
	if rel == "" {
		return s.root.OpenRoot(".")
	}
	before, err := s.root.Lstat(rel)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() {
		return nil, errors.New("not a directory")
	}
	dr, err := s.root.OpenRoot(rel)
	if err != nil {
		return nil, err
	}
	after, err := dr.Stat(".")
	if err != nil {
		dr.Close()
		return nil, err
	}
	if !os.SameFile(before, after) {
		dr.Close()
		return nil, errors.New("directory changed during scan")
	}
	return dr, nil
}

func read_entries(dr *os.Root) ([]fs.DirEntry, error) {
	f, err := dr.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(-1)
}

func (w *worker) read_ignore_rules(dr *os.Root, rel string) *ignore.Rules {
	f, info, err := w.open_regular(dr, ".gitignore")
	if err != nil {
		return nil
	}
	defer f.Close()
	if info.Size() > max_ignore_bytes {
		w.s.warn(join_rel(rel, ".gitignore"), "ignore file too large, skipped")
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(f, max_ignore_bytes))
	if err != nil {
		w.s.warn(join_rel(rel, ".gitignore"), reason(err))
		return nil
	}
	return ignore.Parse(rel, data)
}

var err_not_regular = errors.New("not a regular file")

func (w *worker) open_regular(dr *os.Root, name string) (*os.File, os.FileInfo, error) {
	before, err := dr.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, before, err_not_regular
	}
	f, err := dr.OpenFile(name, os.O_RDONLY|open_flags, 0)
	if err != nil {
		return nil, nil, err
	}
	after, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, after) {
		f.Close()
		return nil, after, err_not_regular
	}
	return f, after, nil
}

func (w *worker) run_dir(t task) {
	s := w.s
	dr, err := s.open_dir(t.rel)
	if err != nil {
		w.skipped.Unreadable++
		s.warn(t.rel, reason(err))
		return
	}
	defer dr.Close()
	entries, err := read_entries(dr)
	if err != nil {
		w.skipped.Unreadable++
		s.warn(t.rel, reason(err))
	}
	matcher := t.matcher
	if !s.opts.No_gitignore {
		for _, e := range entries {
			if e.Name() == ".gitignore" && e.Type().IsRegular() {
				matcher = matcher.Child(w.read_ignore_rules(dr, t.rel))
				break
			}
		}
	}
	var subdirs []task
	var names []string
	for _, e := range entries {
		name := e.Name()
		typ := e.Type()
		switch {
		case typ&fs.ModeSymlink != 0:
			w.skipped.Symlinks++
			continue
		case typ.IsDir():
		case typ.IsRegular():
		default:
			w.skipped.Special++
			continue
		}
		is_dir := typ.IsDir()
		rel := join_rel(t.rel, name)
		if s.forced.Is_ignored(rel, is_dir) {
			w.skipped.Ignored++
			continue
		}
		if name[0] == '.' && !s.opts.Include_hidden {
			w.skipped.Hidden++
			continue
		}
		if matcher.Is_ignored(rel, is_dir) {
			w.skipped.Ignored++
			continue
		}
		if !is_dir {
			names = append(names, name)
			continue
		}
		if t.dir.Depth+1 >= s.opts.Max_depth {
			w.skipped.Depth_limit++
			s.warn(rel, "directory depth limit reached")
			continue
		}
		child := &Dir{Name: name, Parent: t.dir, Depth: t.dir.Depth + 1}
		t.dir.Dirs = append(t.dir.Dirs, child)
		subdirs = append(subdirs, task{dir: child, rel: rel, matcher: matcher, is_dir: true})
	}
	t.dir.Files = make([]File, len(names))
	t.dir.kept = make([]bool, len(names))
	for i, n := range names {
		t.dir.Files[i].Name = n
	}
	var batches []task
	for first := batch_size; first < len(names); first += batch_size {
		last := min(first+batch_size, len(names))
		batches = append(batches, task{dir: t.dir, rel: t.rel, first: first, last: last})
	}
	s.push(subdirs)
	s.push(batches)
	w.process_files(dr, t.dir, t.rel, 0, min(batch_size, len(names)))
}

func (w *worker) run_batch(t task) {
	dr, err := w.s.open_dir(t.rel)
	if err != nil {
		w.skipped.Unreadable += int64(t.last - t.first)
		w.s.warn(t.rel, reason(err))
		return
	}
	defer dr.Close()
	w.process_files(dr, t.dir, t.rel, t.first, t.last)
}

func (w *worker) process_files(dr *os.Root, d *Dir, rel string, first, last int) {
	s := w.s
	for i := first; i < last; i++ {
		if s.ctx.Err() != nil || s.truncated.Load() {
			return
		}
		if s.files.Add(1) > s.opts.Max_files {
			if !s.truncated.Swap(true) {
				s.warn(rel, fmt.Sprintf("file limit of %d reached, scan truncated", s.opts.Max_files))
			}
			return
		}
		f := &d.Files[i]
		path := join_rel(rel, f.Name)
		if w.process_file(dr, path, f) {
			d.kept[i] = true
		}
		if s.opts.Progress != nil {
			s.opts.Progress.Add(1)
		}
	}
}

func (w *worker) process_file(dr *os.Root, path string, f *File) bool {
	s := w.s
	file, info, err := w.open_regular(dr, f.Name)
	if err != nil {
		switch {
		case errors.Is(err, err_not_regular):
			if info != nil && info.Mode()&fs.ModeSymlink != 0 {
				w.skipped.Symlinks++
			} else {
				w.skipped.Special++
			}
		case errors.Is(err, fs.ErrNotExist):
			s.warn(path, "file vanished during scan")
			w.skipped.Unreadable++
		default:
			s.warn(path, reason(err))
			w.skipped.Unreadable++
		}
		return false
	}
	defer file.Close()
	if info.Size() > s.opts.Max_file_size {
		w.skipped.Too_large++
		s.warn(path, fmt.Sprintf("file larger than %d bytes, skipped", s.opts.Max_file_size))
		return false
	}
	status, err := w.count_file(file, f)
	switch status {
	case count_ok:
		return true
	case count_binary:
		w.skipped.Binary++
	case count_too_large:
		w.skipped.Too_large++
		s.warn(path, fmt.Sprintf("file larger than %d bytes, skipped", s.opts.Max_file_size))
	case count_error:
		w.skipped.Unreadable++
		s.warn(path, reason(err))
	}
	return false
}

type count_status int

const (
	count_ok count_status = iota
	count_binary
	count_too_large
	count_error
)

func (w *worker) count_file(file *os.File, f *File) (count_status, error) {
	buf := w.buf
	n, err := io.ReadFull(file, buf[:binary_probe])
	eof := false
	switch {
	case err == nil:
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		eof = true
	default:
		return count_error, err
	}
	data := buf[:n]
	if bytes.IndexByte(data[:min(n, binary_probe)], 0) >= 0 {
		return count_binary, nil
	}
	l := lang.Detect(f.Name, data[:min(n, shebang_probe)])
	c := &w.counter
	c.reset(l)
	total := int64(n)
	if len(data) >= bom_length && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		c.skip_bom()
		data = data[bom_length:]
	}
	carry := 0
	for {
		proc := data
		carry = 0
		if !eof {
			cut := incomplete_tail(data)
			carry = len(data) - cut
			proc = data[:cut]
		}
		c.feed(proc)
		if eof {
			break
		}
		copy(buf, data[len(data)-carry:])
		m, err := file.Read(buf[carry : carry+chunk_size])
		total += int64(m)
		if total > w.s.opts.Max_file_size {
			return count_too_large, nil
		}
		switch {
		case err == nil:
		case errors.Is(err, io.EOF):
			eof = true
		default:
			return count_error, err
		}
		if m == 0 && err == nil {
			eof = true
		}
		data = buf[:carry+m]
	}
	eol := c.finish()
	f.Lang = uint16(l.Index)
	f.Eol = eol
	f.Metrics = c.m
	f.Bytes = total
	return count_ok, nil
}
