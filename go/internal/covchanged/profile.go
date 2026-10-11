package covchanged

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

var errProfileLine = errors.New("not a coverage profile line")

func ParseProfileLine(line string) (BlockKey, int, int, error) {
	colon := strings.LastIndexByte(line, ':')
	if colon < 1 {
		return BlockKey{}, 0, 0, errProfileLine
	}
	fields := strings.Fields(line[colon+1:])
	if len(fields) != 3 {
		return BlockKey{}, 0, 0, errProfileLine
	}
	key, err := parseSpan(fields[0])
	if err != nil {
		return BlockKey{}, 0, 0, err
	}
	key.File = line[:colon]
	stmts, err := nonNegative(fields[1])
	if err != nil {
		return BlockKey{}, 0, 0, err
	}
	count, err := nonNegative(fields[2])
	if err != nil {
		return BlockKey{}, 0, 0, err
	}
	return key, stmts, count, nil
}

func parseSpan(span string) (BlockKey, error) {
	start, end, ok := strings.Cut(span, ",")
	if !ok {
		return BlockKey{}, errProfileLine
	}
	sl, sc, err := linePos(start)
	if err != nil {
		return BlockKey{}, err
	}
	el, ec, err := linePos(end)
	if err != nil {
		return BlockKey{}, err
	}
	if sl < 1 || el < sl {
		return BlockKey{}, errProfileLine
	}
	return BlockKey{StartLine: sl, StartCol: sc, EndLine: el, EndCol: ec}, nil
}

func linePos(pos string) (int, int, error) {
	l, c, ok := strings.Cut(pos, ".")
	if !ok {
		return 0, 0, errProfileLine
	}
	line, err := nonNegative(l)
	if err != nil {
		return 0, 0, err
	}
	col, err := nonNegative(c)
	return line, col, err
}

func nonNegative(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, errProfileLine
	}
	return n, nil
}

func ParseProfile(data []byte, modulePath string) (Profile, error) {
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if !strings.HasPrefix(lines[0], "mode: ") {
		return nil, errors.New("covchanged: the profile does not start with a mode line")
	}
	merged := map[BlockKey]Block{}
	prefix := modulePath + "/"
	for n, line := range lines[1:] {
		if err := mergeLine(merged, line, prefix); err != nil {
			return nil, fmt.Errorf("covchanged: profile line %d: %w", n+2, err)
		}
	}
	return byFile(merged), nil
}

func mergeLine(merged map[BlockKey]Block, line, prefix string) error {
	if line == "" {
		return nil
	}
	key, stmts, count, err := ParseProfileLine(line)
	if err != nil {
		return fmt.Errorf("%q: %w", line, err)
	}
	if !strings.HasPrefix(key.File, prefix) {
		return nil
	}
	key.File = strings.TrimPrefix(key.File, prefix)
	b := merged[key]
	merged[key] = Block{BlockKey: key, Statements: stmts, Hit: b.Hit || count > 0}
	return nil
}

func byFile(merged map[BlockKey]Block) Profile {
	out := Profile{}
	for _, b := range merged {
		out[b.File] = append(out[b.File], b)
	}
	for _, blocks := range out {
		sort.Slice(blocks, func(i, j int) bool { return blocks[i].before(blocks[j].BlockKey) })
	}
	return out
}

func (k BlockKey) before(o BlockKey) bool {
	return slices.Compare([]int{k.StartLine, k.StartCol, k.EndLine, k.EndCol}, []int{o.StartLine, o.StartCol, o.EndLine, o.EndCol}) < 0
}
