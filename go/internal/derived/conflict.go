package derived

import "strings"

type lineSpan struct{ first, last int }

func IsDerivedConflict(relPath string, merged []byte) bool {
	_, o, ok := OutputOf(relPath)
	if !ok {
		return false
	}
	lines := strings.Split(string(merged), "\n")
	if o.Region == "" {
		return o.bothSidesCarryTheMarker(lines)
	}
	region, hasRegion := o.regionSpan(lines)
	blocks, closed := conflictBlocks(lines)
	if !hasRegion || !closed || len(blocks) == 0 {
		return false
	}
	for _, b := range blocks {
		if b.first <= region.first || b.last >= region.last {
			return false
		}
	}
	return true
}

func (o Output) regionSpan(lines []string) (lineSpan, bool) {
	begin := indexWithPrefix(lines, o.begin(), 0)
	if begin < 0 {
		return lineSpan{}, false
	}
	end := indexWithPrefix(lines, o.end(), begin+1)
	return lineSpan{begin, end}, end >= 0
}

func indexWithPrefix(lines []string, prefix string, from int) int {
	for i := from; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], prefix) {
			return i
		}
	}
	return -1
}

func conflictBlocks(lines []string) ([]lineSpan, bool) {
	var blocks []lineSpan
	open := -1
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "<<<<<<<") && open < 0:
			open = i
		case strings.HasPrefix(l, ">>>>>>>") && open >= 0:
			blocks = append(blocks, lineSpan{open, i})
			open = -1
		}
	}
	return blocks, open < 0
}

func (o Output) bothSidesCarryTheMarker(lines []string) bool {
	if o.Marker == "" {
		return true
	}
	ours, theirs := conflictSides(lines)
	return strings.Contains(ours, o.Marker) && strings.Contains(theirs, o.Marker)
}

type conflictSide int

const (
	bothSides conflictSide = iota
	oursSide
	baseSide
	theirsSide
)

func conflictSides(lines []string) (ours, theirs string) {
	var o, t []string
	side := bothSides
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "<<<<<<<"):
			side = oursSide
		case strings.HasPrefix(l, "|||||||") && side == oursSide:
			side = baseSide
		case strings.HasPrefix(l, "=======") && side != bothSides:
			side = theirsSide
		case strings.HasPrefix(l, ">>>>>>>") && side != bothSides:
			side = bothSides
		default:
			if side == bothSides || side == oursSide {
				o = append(o, l)
			}
			if side == bothSides || side == theirsSide {
				t = append(t, l)
			}
		}
	}
	return strings.Join(o, "\n"), strings.Join(t, "\n")
}
