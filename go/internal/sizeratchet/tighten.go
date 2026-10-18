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
	current := sizes(spans)
	tight := make(map[string]int, len(offenders))
	for key, allowance := range offenders {
		if n := current[key]; n > MaxLines {
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
	tight := Tighten(spans, offenders)
	encoded, err = Encode(tight)
	if err != nil {
		return nil, nil, err
	}
	return Changes(offenders, tight), encoded, nil
}
