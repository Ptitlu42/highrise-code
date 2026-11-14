package layout

func fold(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 'a' - 'A'
	}
	return b
}

func contains_fold(s, lower_sub string) bool {
	n, m := len(s), len(lower_sub)
	if m == 0 {
		return true
	}
	for i := 0; i+m <= n; i++ {
		j := 0
		for j < m && fold(s[i+j]) == lower_sub[j] {
			j++
		}
		if j == m {
			return true
		}
	}
	return false
}

func lower_ascii(s string) string {
	b := []byte(s)
	for i := range b {
		b[i] = fold(b[i])
	}
	return string(b)
}

func (l *Layout) Search(query string, limit int) []int32 {
	if query == "" || limit <= 0 {
		return nil
	}
	q := lower_ascii(query)
	by_path := false
	for i := 0; i < len(q); i++ {
		if q[i] == '/' {
			by_path = true
			break
		}
	}
	var out []int32
	for i := range l.Items {
		if i == 0 {
			continue
		}
		var hay string
		if by_path {
			hay = l.Path(i)
		} else {
			hay = l.Name(i)
		}
		if contains_fold(hay, q) {
			out = append(out, int32(i))
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}
