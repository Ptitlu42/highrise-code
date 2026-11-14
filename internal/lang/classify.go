package lang

import "bytes"

type Classifier struct {
	l     *Language
	depth int
	block *Block
}

var ascii_space = [256]bool{' ': true, '\t': true, '\r': true, '\v': true, '\f': true, '\n': true}

func (c *Classifier) Reset(l *Language) {
	c.l = l
	c.depth = 0
	c.block = nil
}

func (c *Classifier) In_block() bool {
	return c.depth > 0
}

func (c *Classifier) Classify(p []byte) (code, comment bool) {
	l := c.l
	i := 0
	if c.depth > 0 {
		comment = true
		i = c.scan_block(p, 0)
		if c.depth > 0 {
			return false, true
		}
	}
	n := len(p)
	for i < n {
		b := p[i]
		switch l.class[b] {
		case class_space:
			i++
			continue
		case class_plain:
			code = true
			i++
			continue
		}
		if blk := c.match_block(p[i:]); blk != nil {
			comment = true
			c.block = blk
			c.depth = 1
			i = c.scan_block(p, i+len(blk.start))
			if c.depth > 0 {
				return code, comment
			}
			continue
		}
		if c.match_line(p[i:]) {
			return code, true
		}
		if l.is_quote[b] {
			code = true
			i = skip_string(p, i)
			continue
		}
		code = true
		i++
	}
	return code, comment
}

func (c *Classifier) match_block(p []byte) *Block {
	for i := range c.l.Block_comments {
		if bytes.HasPrefix(p, c.l.Block_comments[i].start) {
			return &c.l.Block_comments[i]
		}
	}
	return nil
}

func (c *Classifier) match_line(p []byte) bool {
	for _, m := range c.l.line_markers {
		if bytes.HasPrefix(p, m) {
			return true
		}
	}
	return false
}

func (c *Classifier) scan_block(p []byte, i int) int {
	blk := c.block
	for c.depth > 0 {
		e := bytes.Index(p[i:], blk.end)
		if e < 0 {
			return len(p)
		}
		if c.l.Nested {
			if s := bytes.Index(p[i:i+e], blk.start); s >= 0 {
				c.depth++
				i += s + len(blk.start)
				continue
			}
		}
		i += e + len(blk.end)
		c.depth--
	}
	return i
}

func skip_string(p []byte, i int) int {
	q := p[i]
	i++
	for i < len(p) {
		switch p[i] {
		case '\\':
			i += 2
			continue
		case q:
			return i + 1
		}
		i++
	}
	return len(p)
}
