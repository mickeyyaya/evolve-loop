package stelint

import "strings"

type blockSplitter struct {
	paragraphs   []paragraph
	regions      []int
	cur          *paragraph
	fence        string
	isStandard   bool
	inComment    bool
	inHTML       bool
	inRegion     bool
	inWordTable  bool
	inQuote      bool
	quoteDecided bool
	quoteExempt  bool
}

func splitBlocks(lines []string, isStandard bool) blockSplitter {
	b := blockSplitter{isStandard: isStandard}
	for i := frontmatterEnd(lines); i < len(lines); i++ {
		b.line(i+1, strings.TrimRight(lines[i], "\r"))
	}
	b.flush()
	return b
}

func (b *blockSplitter) flush() {
	if b.cur != nil {
		b.paragraphs = append(b.paragraphs, *b.cur)
		b.cur = nil
	}
}

func (b *blockSplitter) line(n int, text string) {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, ">") {
		b.quoteLine(n, trimmed)
		return
	}
	b.inQuote = false
	b.body(n, text, trimmed)
}

func (b *blockSplitter) quoteLine(n int, trimmed string) {
	content := quoteContent(trimmed)
	if !b.inQuote {
		b.flush()
		b.inQuote, b.quoteDecided, b.quoteExempt = true, false, false
	}
	if text := strings.TrimSpace(content); !b.quoteDecided && text != "" {
		b.quoteDecided = true
		b.quoteExempt = startsWithQuotation(text)
	}
	if !b.quoteExempt {
		b.body(n, content, strings.TrimSpace(content))
	}
}

func (b *blockSplitter) body(n int, text, trimmed string) {
	if b.skipping(trimmed) {
		return
	}
	if b.opensSkip(n, trimmed) {
		b.flush()
		return
	}
	switch {
	case trimmed == "":
		b.flush()
	case isHeading(trimmed):
		b.flush()
		b.heading(n, headingText(trimmed))
	case strings.HasPrefix(trimmed, "|"):
		b.flush()
		b.tableRow(n, trimmed)
	case b.cur != nil && isRuleLine(trimmed, "=-"):
		b.cur.kind = headingParagraph
		b.flush()
	case isRuleLine(trimmed, "-*_"):
		b.flush()
	default:
		b.textLine(n, text)
	}
}

func (b *blockSplitter) heading(n int, text string) {
	b.inWordTable = b.isStandard && text == WordTableHeading
	b.paragraphs = append(b.paragraphs, paragraph{kind: headingParagraph, lines: []srcLine{{n: n, text: text}}})
}

func (b *blockSplitter) skipping(trimmed string) bool {
	switch {
	case b.inRegion:
		b.inRegion = !isRegionEnd(trimmed)
	case b.fence != "":
		if closesFence(trimmed, b.fence) {
			b.fence = ""
		}
	case b.inComment:
		b.inComment = !strings.Contains(trimmed, "-->")
	case b.inHTML:
		b.inHTML = trimmed != ""
	default:
		return false
	}
	return true
}

func (b *blockSplitter) opensSkip(n int, trimmed string) bool {
	fence := fenceMarker(trimmed)
	switch {
	case isRegionBegin(trimmed):
		b.regions = append(b.regions, n)
		b.inRegion = true
	case fence != "":
		b.fence = fence
	case strings.HasPrefix(trimmed, "<!--"):
		b.inComment = !strings.Contains(trimmed[len("<!--"):], "-->")
	case isHTMLBlockStart(trimmed):
		b.inHTML = true
	default:
		return false
	}
	return true
}

func (b *blockSplitter) tableRow(n int, trimmed string) {
	cells := splitCells(trimmed)
	if b.inWordTable || isSeparatorRow(cells) {
		return
	}
	for _, c := range cells {
		b.paragraphs = append(b.paragraphs, paragraph{lines: []srcLine{{n: n, text: c}}})
	}
}

func (b *blockSplitter) textLine(n int, text string) {
	if item, ok := listItem(text); ok {
		b.flush()
		item.lines[0].n = n
		b.cur = &item
		return
	}
	if b.cur == nil {
		b.cur = &paragraph{}
	}
	b.cur.lines = append(b.cur.lines, srcLine{n: n, text: text})
}
