package scan

import (
	"bytes"
	"unicode"
	"unicode/utf8"

	"github.com/ptitlu42/highrise-code/internal/lang"
)

const (
	prefix_limit = 4096
	bom_length   = 3
)

var ascii_blank = [utf8.RuneSelf]bool{' ': true, '\t': true, '\n': true, '\v': true, '\f': true, '\r': true}

type counter struct {
	classifier lang.Classifier
	prefix     [prefix_limit]byte
	prefix_len int
	line_len   int64
	line_chars int64
	line_nonws int64
	last_byte  byte
	has_lf     bool
	has_crlf   bool
	m          Metrics
}

func (c *counter) reset(l *lang.Language) {
	c.classifier.Reset(l)
	c.prefix_len = 0
	c.line_len = 0
	c.line_chars = 0
	c.line_nonws = 0
	c.last_byte = 0
	c.has_lf = false
	c.has_crlf = false
	c.m = Metrics{}
}

func count_runes(seg []byte) (chars, nonws int64) {
	i := 0
	n := len(seg)
	for i < n {
		b := seg[i]
		if b < utf8.RuneSelf {
			chars++
			if !ascii_blank[b] {
				nonws++
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(seg[i:])
		chars++
		if !unicode.IsSpace(r) {
			nonws++
		}
		i += size
	}
	return chars, nonws
}

func (c *counter) skip_bom() {
	c.line_len += bom_length
	c.line_chars++
	c.last_byte = 0xBF
}

func (c *counter) segment(seg []byte) {
	if len(seg) == 0 {
		return
	}
	c.line_len += int64(len(seg))
	c.last_byte = seg[len(seg)-1]
	if room := prefix_limit - c.prefix_len; room > 0 {
		n := copy(c.prefix[c.prefix_len:], seg)
		c.prefix_len += n
	}
	chars, nonws := count_runes(seg)
	c.line_chars += chars
	c.line_nonws += nonws
}

func (c *counter) feed(data []byte) {
	for len(data) > 0 {
		nl := bytes.IndexByte(data, '\n')
		if nl < 0 {
			c.segment(data)
			return
		}
		c.segment(data[:nl])
		if c.last_byte == '\r' {
			c.has_crlf = true
		} else {
			c.has_lf = true
		}
		c.line_len++
		c.line_chars++
		c.end_line()
		data = data[nl+1:]
	}
}

func (c *counter) finish() uint8 {
	var eol uint8
	if c.line_len > 0 {
		c.end_line()
		eol |= Eol_no_final_newline
	}
	if c.has_lf {
		eol |= Eol_lf
	}
	if c.has_crlf {
		eol |= Eol_crlf
	}
	return eol
}

func (c *counter) end_line() {
	m := &c.m
	m.Lines++
	m.Chars += c.line_chars
	m.Chars_nonws += c.line_nonws
	if c.line_nonws == 0 {
		m.Blank++
		m.Chars_blank += c.line_chars
	} else {
		code, comment := c.classifier.Classify(c.prefix[:c.prefix_len])
		if comment && !code {
			m.Comment++
			m.Chars_comment += c.line_chars
		} else {
			m.Code++
			m.Chars_code += c.line_chars
		}
	}
	c.prefix_len = 0
	c.line_len = 0
	c.line_chars = 0
	c.line_nonws = 0
	c.last_byte = 0
}

func incomplete_tail(data []byte) int {
	n := len(data)
	lo := n - (utf8.UTFMax - 1)
	if lo < 0 {
		lo = 0
	}
	for i := n - 1; i >= lo; i-- {
		if utf8.RuneStart(data[i]) {
			if !utf8.FullRune(data[i:]) {
				return i
			}
			return n
		}
	}
	return n
}
