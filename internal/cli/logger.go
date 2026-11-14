package cli

import (
	"fmt"
	"io"
	"os"
)

type level int

const (
	level_quiet level = iota
	level_normal
	level_verbose
)

type logger struct {
	w     io.Writer
	level level
	color bool
}

func (l *logger) emit(min level, tag, color_code, format string, args ...any) {
	if l.level < min {
		return
	}
	prefix := "highrise-code: "
	if tag != "" {
		if l.color {
			prefix += "\x1b[" + color_code + "m" + tag + "\x1b[0m: "
		} else {
			prefix += tag + ": "
		}
	}
	fmt.Fprintf(l.w, prefix+format+"\n", args...)
}

func (l *logger) errorf(format string, args ...any) {
	l.emit(level_quiet, "error", "31", format, args...)
}

func (l *logger) warnf(format string, args ...any) {
	l.emit(level_normal, "warning", "33", format, args...)
}

func (l *logger) infof(format string, args ...any) {
	l.emit(level_normal, "", "", format, args...)
}

func (l *logger) debugf(format string, args ...any) {
	l.emit(level_verbose, "debug", "36", format, args...)
}

func is_terminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func color_enabled(w io.Writer, getenv func(string) string) bool {
	if getenv("NO_COLOR") != "" {
		return false
	}
	if getenv("TERM") == "dumb" {
		return false
	}
	return is_terminal(w)
}
