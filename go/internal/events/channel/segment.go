package channel

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	segmentPrefix = "seg-"
	segmentSuffix = ".ndjson"
	segmentDigits = 20
)

type Segment struct {
	Base int64
	Path string
}

func segmentName(base int64) string {
	return fmt.Sprintf("%s%0*d%s", segmentPrefix, segmentDigits, base, segmentSuffix)
}

func segmentBase(name string) (int64, bool) {
	digits, ok := strings.CutPrefix(name, segmentPrefix)
	digits, hasSuffix := strings.CutSuffix(digits, segmentSuffix)
	if !ok || !hasSuffix || len(digits) != segmentDigits {
		return 0, false
	}
	base, err := strconv.ParseInt(digits, 10, 64)
	return base, err == nil
}

func (l *Log) Segments() ([]Segment, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, l.errorf("list segments: %w", err)
	}
	var segs []Segment
	for _, e := range entries {
		if base, ok := segmentBase(e.Name()); ok {
			segs = append(segs, Segment{Base: base, Path: filepath.Join(l.dir, e.Name())})
		}
	}
	return segs, nil
}

func (l *Log) tail(segs []Segment) Segment {
	if len(segs) == 0 {
		return Segment{Base: 0, Path: filepath.Join(l.dir, segmentName(0))}
	}
	return segs[len(segs)-1]
}

func segmentAt(segs []Segment, cursor int64) int {
	return sort.Search(len(segs), func(i int) bool { return segs[i].Base > cursor }) - 1
}

func SegmentOf(segs []Segment, cursor int64) (Segment, bool) {
	i := segmentAt(segs, cursor)
	if i < 0 {
		return Segment{}, false
	}
	return segs[i], true
}
