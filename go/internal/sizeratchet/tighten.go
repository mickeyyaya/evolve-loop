package sizeratchet

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
)

var marshalIndent = json.MarshalIndent

func Tighten(spans []FuncSpan, offenders map[string]int) map[string]int {
	return TightenCounts(sizes(spans), offenders, MaxLines)
}

func TightenCounts(current, allowances map[string]int, floor int) map[string]int {
	tight := make(map[string]int, len(allowances))
	for key, allowance := range allowances {
		if n := current[key]; n > floor {
			tight[key] = min(n, allowance)
		}
	}
	return tight
}

func Encode(offenders map[string]int) ([]byte, error) {
	body, err := marshalIndent(offenders, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("sizeratchet: encode offender list: %w", err)
	}
	return append(body, '\n'), nil
}

func Changes(before, after map[string]int) []string {
	var lines []string
	for _, key := range slices.Sorted(maps.Keys(before)) {
		switch n, kept := after[key]; {
		case !kept:
			lines = append(lines, key+": "+strconv.Itoa(before[key])+" removed")
		case n < before[key]:
			lines = append(lines, fmt.Sprintf("%s: %d -> %d", key, before[key], n))
		}
	}
	return lines
}

func TightenModule(root string) (changes []string, encoded []byte, err error) {
	spans, err := Walk(root)
	if err != nil {
		return nil, nil, err
	}
	offenders, err := LoadOffenders(filepath.Join(root, filepath.FromSlash(OffendersRelPath)))
	if err != nil {
		return nil, nil, err
	}
	return TightenEncoded(sizes(spans), offenders, MaxLines)
}

func TightenEncoded(current, allowances map[string]int, floor int) (changes []string, encoded []byte, err error) {
	tight := TightenCounts(current, allowances, floor)
	encoded, err = Encode(tight)
	if err != nil {
		return nil, nil, err
	}
	return Changes(allowances, tight), encoded, nil
}
