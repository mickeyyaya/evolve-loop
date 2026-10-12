package covchanged

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

type hunk struct {
	file      string
	next      int
	oldLeft   int
	newLeft   int
	collected AddedLines
	removed   AddedLines
	anchor    int
	dropped   []string
}

func ParseDiff(diff []byte) (Diff, error) {
	h := hunk{collected: AddedLines{}, removed: AddedLines{}}
	for n, line := range strings.Split(strings.TrimSuffix(string(diff), "\n"), "\n") {
		if err := h.consume(line); err != nil {
			return Diff{}, fmt.Errorf("covchanged: diff line %d: %w", n+1, err)
		}
	}
	if h.oldLeft > 0 || h.newLeft > 0 {
		return Diff{}, fmt.Errorf("covchanged: the diff ends inside a hunk of %s", h.file)
	}
	return Diff{Added: h.collected, Removed: h.removed}, nil
}

func (h *hunk) consume(line string) error {
	if h.oldLeft > 0 || h.newLeft > 0 {
		return h.body(line)
	}
	switch {
	case strings.HasPrefix(line, "+++ "):
		return h.target(strings.TrimPrefix(line, "+++ "))
	case strings.HasPrefix(line, "@@"):
		return h.header(line)
	}
	return nil
}

func (h *hunk) target(path string) error {
	path = strings.TrimSuffix(path, "\t")
	if strings.HasPrefix(path, `"`) {
		unquoted, err := strconv.Unquote(path)
		if err != nil {
			return fmt.Errorf("quoted path %s: %w", path, err)
		}
		path = unquoted
	}
	h.file = strings.TrimPrefix(path, "b/")
	if path == "/dev/null" {
		h.file = ""
	}
	return nil
}

func (h *hunk) header(line string) error {
	m := hunkHeader.FindStringSubmatch(line)
	if m == nil {
		return fmt.Errorf("malformed hunk header %q", line)
	}
	oldCount, err := countOrOne(m[1])
	if err != nil {
		return err
	}
	start, err := strconv.Atoi(m[2])
	if err != nil {
		return fmt.Errorf("hunk start %q: %w", m[2], err)
	}
	newCount, err := countOrOne(m[3])
	if err != nil {
		return err
	}
	h.next, h.oldLeft, h.newLeft = start, oldCount, newCount
	return nil
}

func countOrOne(s string) (int, error) {
	if s == "" {
		return 1, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("hunk count %q: %w", s, err)
	}
	return n, nil
}

func (h *hunk) body(line string) error {
	switch {
	case strings.HasPrefix(line, "\\"):
		return nil
	case strings.HasPrefix(line, "-"):
		h.drop(line[1:])
	case strings.HasPrefix(line, "+"):
		h.add()
	case strings.HasPrefix(line, " ") || line == "":
		h.oldLeft--
		h.next++
		h.newLeft--
	default:
		return fmt.Errorf("line %q is not part of the hunk", line)
	}
	if h.oldLeft < 0 || h.newLeft < 0 {
		return fmt.Errorf("line %q is past the hunk's counted lines", line)
	}
	if h.oldLeft == 0 && h.newLeft == 0 {
		h.flush()
	}
	return nil
}

func (h *hunk) add() {
	if h.file != "" {
		h.collected[h.file] = append(h.collected[h.file], h.next)
	}
	h.next++
	h.newLeft--
}

func (h *hunk) drop(text string) {
	if len(h.dropped) == 0 {
		h.anchor = h.next
	}
	h.dropped = append(h.dropped, text)
	h.oldLeft--
}

func (h *hunk) flush() {
	removedCode := len(codeLines([]byte(strings.Join(h.dropped, "\n")))) > 0
	if h.file != "" && h.anchor >= 1 && removedCode {
		h.removed[h.file] = append(h.removed[h.file], h.anchor)
	}
	h.dropped = nil
}
