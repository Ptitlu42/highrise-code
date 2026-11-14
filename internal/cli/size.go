package cli

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var size_units = map[string]float64{
	"":    1,
	"b":   1,
	"k":   1 << 10,
	"m":   1 << 20,
	"g":   1 << 30,
	"t":   1 << 40,
	"kib": 1 << 10,
	"mib": 1 << 20,
	"gib": 1 << 30,
	"tib": 1 << 40,
	"kb":  1e3,
	"mb":  1e6,
	"gb":  1e9,
	"tb":  1e12,
}

func Parse_size(s string) (int64, error) {
	text := strings.TrimSpace(s)
	if text == "" {
		return 0, errors.New("empty size")
	}
	split := 0
	for split < len(text) && (text[split] >= '0' && text[split] <= '9' || text[split] == '.') {
		split++
	}
	number, unit := text[:split], strings.ToLower(strings.TrimSpace(text[split:]))
	if number == "" {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	value, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	mult, ok := size_units[unit]
	if !ok {
		return 0, fmt.Errorf("unknown size unit in %q", s)
	}
	total := value * mult
	if total < 1 || total >= math.MaxInt64 {
		return 0, fmt.Errorf("size out of range: %q", s)
	}
	return int64(total), nil
}
