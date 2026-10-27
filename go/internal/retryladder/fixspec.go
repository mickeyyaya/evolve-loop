package retryladder

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrMalformedFixSpec = errors.New("malformed fix spec")
	ErrMalformedFixDiff = errors.New("malformed fix diff")
)

const (
	fixSpecHeader   = "fix-spec: 1"
	expectedPrefix  = "expected:"
	editPrefix      = "edit:"
	replacementMark = ">"
)

type FixEdit struct {
	File        string
	FromLine    int
	ToLine      int
	Replacement []string
}

type FixSpec struct {
	Expected string
	Edits    []FixEdit
}

type specParser struct {
	spec       FixSpec
	collecting bool
}

func ParseFixSpec(text string) (FixSpec, error) {
	var p specParser
	sawHeader := false
	for i, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var err error
		if sawHeader {
			err = p.accept(raw)
		} else {
			err = checkSpecHeader(raw)
			sawHeader = true
		}
		if err != nil {
			return FixSpec{}, fmt.Errorf("%w: line %d: %v", ErrMalformedFixSpec, i+1, err)
		}
	}
	if !sawHeader {
		return FixSpec{}, fmt.Errorf("%w: empty text", ErrMalformedFixSpec)
	}
	if err := p.finish(); err != nil {
		return FixSpec{}, fmt.Errorf("%w: %v", ErrMalformedFixSpec, err)
	}
	return p.spec, nil
}

func checkSpecHeader(raw string) error {
	if strings.TrimSpace(raw) != fixSpecHeader {
		return fmt.Errorf("the first line must be %q", fixSpecHeader)
	}
	return nil
}

func (p *specParser) accept(raw string) error {
	switch {
	case strings.HasPrefix(raw, replacementMark):
		return p.addReplacement(raw)
	case strings.HasPrefix(raw, expectedPrefix):
		return p.setExpected(raw)
	case strings.HasPrefix(raw, editPrefix):
		return p.openEdit(raw)
	}
	return errors.New("unrecognized line")
}

func (p *specParser) closeEdit() error {
	if p.collecting && len(p.spec.Edits[len(p.spec.Edits)-1].Replacement) == 0 {
		return errors.New("an edit has no replacement line")
	}
	p.collecting = false
	return nil
}

func (p *specParser) finish() error {
	if err := p.closeEdit(); err != nil {
		return err
	}
	if p.spec.Expected == "" {
		return errors.New("no expected behavior")
	}
	if len(p.spec.Edits) == 0 {
		return errors.New("no edit")
	}
	return nil
}

func (p *specParser) addReplacement(raw string) error {
	if !p.collecting {
		return errors.New("replacement line outside an edit")
	}
	line := strings.TrimPrefix(strings.TrimPrefix(raw, replacementMark), " ")
	last := &p.spec.Edits[len(p.spec.Edits)-1]
	last.Replacement = append(last.Replacement, line)
	return nil
}

func (p *specParser) setExpected(raw string) error {
	if err := p.closeEdit(); err != nil {
		return err
	}
	value := strings.TrimSpace(strings.TrimPrefix(raw, expectedPrefix))
	if value == "" {
		return errors.New("empty expected behavior")
	}
	if p.spec.Expected != "" {
		return errors.New("duplicate expected behavior")
	}
	p.spec.Expected = value
	return nil
}

func (p *specParser) openEdit(raw string) error {
	if err := p.closeEdit(); err != nil {
		return err
	}
	edit, err := parseEditLocation(strings.TrimSpace(strings.TrimPrefix(raw, editPrefix)))
	if err != nil {
		return err
	}
	for _, earlier := range p.spec.Edits {
		if earlier.File == edit.File && earlier.FromLine <= edit.ToLine && edit.FromLine <= earlier.ToLine {
			return fmt.Errorf("edit overlaps an earlier edit of %s", edit.File)
		}
	}
	p.spec.Edits = append(p.spec.Edits, edit)
	p.collecting = true
	return nil
}

func parseEditLocation(location string) (FixEdit, error) {
	cut := strings.LastIndex(location, ":")
	if cut < 0 {
		return FixEdit{}, errors.New("edit has no file:from-to location")
	}
	file := location[:cut]
	if !repoRelativeFile(file) {
		return FixEdit{}, fmt.Errorf("edit file %q is not a clean repo-relative path", file)
	}
	fromText, toText, ok := strings.Cut(location[cut+1:], "-")
	if !ok {
		return FixEdit{}, errors.New("edit range must be from-to")
	}
	from, fromErr := strconv.Atoi(fromText)
	to, toErr := strconv.Atoi(toText)
	if fromErr != nil || toErr != nil {
		return FixEdit{}, errors.New("edit range is not a number")
	}
	if from < 1 || to < from {
		return FixEdit{}, fmt.Errorf("edit range %d-%d is empty or starts before line 1", from, to)
	}
	return FixEdit{File: file, FromLine: from, ToLine: to}, nil
}

func repoRelativeFile(name string) bool {
	return name != "" && name != "." && !path.IsAbs(name) && path.Clean(name) == name &&
		name != ".." && !strings.HasPrefix(name, "../")
}

type ScopeCode string

const (
	ScopeFileOutside  ScopeCode = "file_out_of_scope"
	ScopeLinesOutside ScopeCode = "lines_out_of_scope"
)

type ScopeViolation struct {
	Code     ScopeCode
	File     string
	FromLine int
	ToLine   int
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+\d+(?:,\d+)? @@`)

func FixScopeViolations(spec FixSpec, diff string) ([]ScopeViolation, error) {
	allowed := map[string][]FixEdit{}
	for _, edit := range spec.Edits {
		allowed[edit.File] = append(allowed[edit.File], edit)
	}
	var out []ScopeViolation
	reported := map[string]bool{}
	file := ""
	outsideFile := func(name string) {
		if _, named := allowed[name]; !named && !reported[name] {
			reported[name] = true
			out = append(out, ScopeViolation{Code: ScopeFileOutside, File: name})
		}
	}
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			target, ok := diffTargetFile(line)
			if !ok {
				return nil, fmt.Errorf("%w: no b/ path in %q", ErrMalformedFixDiff, line)
			}
			file = target
			outsideFile(file)
		case strings.HasPrefix(line, "rename from "):
			outsideFile(strings.TrimPrefix(line, "rename from "))
		case strings.HasPrefix(line, "@@"):
			if file == "" {
				return nil, fmt.Errorf("%w: hunk before any file header", ErrMalformedFixDiff)
			}
			violation, found, err := hunkViolation(allowed[file], file, line)
			if err != nil {
				return nil, err
			}
			if found {
				out = append(out, violation)
			}
		}
	}
	return out, nil
}

func diffTargetFile(header string) (string, bool) {
	rest := strings.TrimPrefix(header, "diff --git ")
	cut := strings.LastIndex(rest, " b/")
	if cut < 0 {
		return "", false
	}
	return rest[cut+len(" b/"):], true
}

func hunkViolation(edits []FixEdit, file, header string) (ScopeViolation, bool, error) {
	match := hunkHeader.FindStringSubmatch(header)
	if match == nil {
		return ScopeViolation{}, false, fmt.Errorf("%w: unreadable hunk header %q", ErrMalformedFixDiff, header)
	}
	start, err := strconv.Atoi(match[1])
	if err != nil {
		return ScopeViolation{}, false, fmt.Errorf("%w: hunk start in %q: %v", ErrMalformedFixDiff, header, err)
	}
	count := 1
	if match[2] != "" {
		if count, err = strconv.Atoi(match[2]); err != nil {
			return ScopeViolation{}, false, fmt.Errorf("%w: hunk length in %q: %v", ErrMalformedFixDiff, header, err)
		}
	}
	if len(edits) == 0 {
		return ScopeViolation{}, false, nil
	}
	from, to := start, start+count-1
	if count == 0 {
		from, to = start, start
	}
	for _, edit := range edits {
		if hunkInsideEdit(edit, from, to, count == 0) {
			return ScopeViolation{}, false, nil
		}
	}
	return ScopeViolation{Code: ScopeLinesOutside, File: file, FromLine: from, ToLine: to}, true, nil
}

func hunkInsideEdit(edit FixEdit, from, to int, insertion bool) bool {
	if insertion {
		return from >= edit.FromLine-1 && from <= edit.ToLine
	}
	return from >= edit.FromLine && to <= edit.ToLine
}
