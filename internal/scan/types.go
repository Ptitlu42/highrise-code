package scan

import (
	"time"

	"github.com/ptitlu42/highrise-code/internal/lang"
)

const (
	Hard_max_depth       = 256
	Hard_max_workers     = 64
	Max_warnings         = 1000
	Default_max_files    = 2000000
	Default_max_filesize = 64 << 20
)

const (
	Eol_lf uint8 = 1 << iota
	Eol_crlf
	Eol_no_final_newline
)

type Metrics struct {
	Bytes         int64
	Lines         int64
	Code          int64
	Comment       int64
	Blank         int64
	Chars         int64
	Chars_code    int64
	Chars_comment int64
	Chars_blank   int64
	Chars_nonws   int64
}

func (m *Metrics) Add(o *Metrics) {
	m.Bytes += o.Bytes
	m.Lines += o.Lines
	m.Code += o.Code
	m.Comment += o.Comment
	m.Blank += o.Blank
	m.Chars += o.Chars
	m.Chars_code += o.Chars_code
	m.Chars_comment += o.Chars_comment
	m.Chars_blank += o.Chars_blank
	m.Chars_nonws += o.Chars_nonws
}

type File struct {
	Name string
	Lang uint16
	Eol  uint8
	Metrics
}

func (f *File) Language() *lang.Language {
	return lang.By_index(int(f.Lang))
}

type Dir struct {
	Name        string
	Parent      *Dir
	Depth       int
	Dirs        []*Dir
	Files       []File
	Files_total int64
	Dirs_total  int64
	Dominant    uint16
	Metrics

	kept []bool
}

func (d *Dir) Path() string {
	if d.Parent == nil {
		return ""
	}
	n := -1
	for x := d; x.Parent != nil; x = x.Parent {
		n += len(x.Name) + 1
	}
	buf := make([]byte, n)
	pos := n
	for x := d; x.Parent != nil; x = x.Parent {
		pos -= len(x.Name)
		copy(buf[pos:], x.Name)
		if pos > 0 {
			pos--
			buf[pos] = '/'
		}
	}
	return string(buf)
}

func (d *Dir) File_path(f *File) string {
	p := d.Path()
	if p == "" {
		return f.Name
	}
	return p + "/" + f.Name
}

type Skipped struct {
	Symlinks    int64
	Special     int64
	Binary      int64
	Too_large   int64
	Ignored     int64
	Hidden      int64
	Unreadable  int64
	Depth_limit int64
}

type Warning struct {
	Path    string
	Message string
}

type Result struct {
	Root           *Dir
	Root_path      string
	Root_name      string
	Skipped        Skipped
	Warnings       []Warning
	Warnings_total int64
	Truncated      bool
	Duration       time.Duration
}
