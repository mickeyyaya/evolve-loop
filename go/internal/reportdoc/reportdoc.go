// Package reportdoc parses small, machine-checked sections in agent-authored
// Markdown reports. It ignores fenced, indented, and HTML-commented content so
// hidden examples cannot satisfy a deliverable contract.
package reportdoc

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxCitationBytes = 16 << 20

// Section returns one exact level-two section. Duplicate headings are invalid.
func Section(markdown, heading string) (string, bool, error) {
	lines := visibleLines(markdown)
	start := -1
	for i, line := range lines {
		if title, ok := levelTwoHeading(line); ok && title == heading {
			if start >= 0 {
				return "", true, fmt.Errorf("report has duplicate ## %s sections", heading)
			}
			start = i + 1
		}
	}
	if start < 0 {
		return "", false, nil
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if isSectionBoundary(lines[i]) {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n")), true, nil
}

func isSectionBoundary(line string) bool {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return false
	}
	return strings.HasPrefix(trimmed, "# ") ||
		strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ")
}

// Fields parses unique, allowed "- Key: value" metadata lines
// case-insensitively. Unknown prose fields are ignored.
func Fields(body string, allowed ...string) (map[string]string, error) {
	allow := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		allow[strings.ToLower(strings.TrimSpace(key))] = true
	}
	fields := map[string]string{}
	for _, raw := range visibleLines(body) {
		line := strings.TrimLeft(raw, " ")
		if len(raw)-len(line) > 3 || !strings.HasPrefix(line, "- ") {
			continue
		}
		line = strings.ReplaceAll(strings.TrimSpace(strings.TrimPrefix(line, "- ")), "**", "")
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" || len(allow) > 0 && !allow[key] {
			continue
		}
		if _, exists := fields[key]; exists {
			return nil, fmt.Errorf("report has duplicate %s fields", key)
		}
		fields[key] = strings.TrimSpace(value)
	}
	return fields, nil
}

// RequirePathLineEvidence rejects token/canned evidence and requires every
// authoritative path to be cited at a concrete line. This keeps qualitative
// review grounded in inspected source rather than path-name repetition.
func RequirePathLineEvidence(evidence string, references ...string) error {
	evidence = strings.TrimSpace(evidence)
	if len(evidence) < 20 {
		return fmt.Errorf("explanation review requires concrete Evidence")
	}
	for _, reference := range references {
		if reference == "" {
			continue
		}
		if _, ok := citationLine(evidence, reference); !ok {
			return fmt.Errorf("explanation review Evidence must cite %s with path:line evidence", reference)
		}
	}
	return nil
}

// RequirePathLineEvidenceAt additionally proves that every cited line exists
// in the current file, or in the base blob when the Build deleted that path.
func RequirePathLineEvidenceAt(ctx context.Context, root, baseSHA, evidence string, references ...string) error {
	if err := RequirePathLineEvidence(evidence, references...); err != nil {
		return err
	}
	if root == "" {
		return nil
	}
	for _, reference := range references {
		if reference == "" {
			continue
		}
		line, _ := citationLine(evidence, reference)
		lineCount, err := referencedLineCount(ctx, root, baseSHA, reference)
		if err != nil {
			return fmt.Errorf("verify explanation review citation %s: %w", reference, err)
		}
		if line > lineCount {
			return fmt.Errorf("explanation review Evidence cites %s:%d, but the file has %d lines", reference, line, lineCount)
		}
	}
	return nil
}

func citationLine(evidence, reference string) (int, bool) {
	for offset := 0; offset < len(evidence); {
		relative := strings.Index(evidence[offset:], reference)
		if relative < 0 {
			return 0, false
		}
		start := offset + relative
		end := start + len(reference)
		if citationBoundaryBefore(evidence, start) {
			tail := strings.TrimPrefix(evidence[end:], "`")
			prefix := ":"
			if strings.HasPrefix(tail, "#L") {
				prefix = "#L"
			}
			if strings.HasPrefix(tail, prefix) {
				digits := tail[len(prefix):]
				i := 0
				for i < len(digits) && digits[i] >= '0' && digits[i] <= '9' {
					i++
				}
				next, _ := utf8.DecodeRuneInString(digits[i:])
				if i > 0 && (i == len(digits) || citationBoundaryAfter(next)) {
					line, err := strconv.Atoi(digits[:i])
					if err == nil && line > 0 {
						return line, true
					}
				}
			}
		}
		offset = start + 1
	}
	return 0, false
}

func citationBoundaryBefore(value string, index int) bool {
	if index == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(value[:index])
	return citationBoundaryAfter(r)
}

func citationBoundaryAfter(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("/\\._-", r)
}

func referencedLineCount(ctx context.Context, root, baseSHA, reference string) (int, error) {
	if !ValidReference(reference) {
		return 0, fmt.Errorf("invalid repository-relative path")
	}
	body, oneLine, err := readCurrentReference(ctx, root, reference)
	if os.IsNotExist(err) && baseSHA != "" {
		body, oneLine, err = readBaseReference(ctx, root, baseSHA, reference)
		if err != nil {
			return 0, err
		}
	}
	if err != nil {
		return 0, err
	}
	if oneLine || len(body) == 0 {
		return 1, nil
	}
	lines := strings.Count(string(body), "\n")
	if body[len(body)-1] != '\n' {
		lines++
	}
	return lines, nil
}

// ValidReference reports whether reference is a safe, clean, relative path —
// the single home of the "is this relative path safe?" belief, shared with
// explanationdocs (architecture review 2026-09-01: the two packages had
// divergent validators judging the same string differently).
func ValidReference(reference string) bool {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(reference)))
	return reference != "" && clean == reference && clean != "." && !filepath.IsAbs(reference) &&
		clean != ".." && !strings.HasPrefix(clean, "../") && !strings.ContainsAny(reference, ":\\\x00")
}

func readCurrentReference(ctx context.Context, root, reference string) ([]byte, bool, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, false, err
	}
	path := filepath.Join(realRoot, filepath.FromSlash(reference))
	info, err := os.Lstat(path)
	if err != nil {
		return nil, false, err
	}
	realParent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return nil, false, err
	}
	rel, err := filepath.Rel(realRoot, realParent)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, false, fmt.Errorf("citation target escapes the worktree")
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			return nil, false, err
		}
		after, err := os.Lstat(path)
		if err != nil || !os.SameFile(info, after) {
			return nil, false, fmt.Errorf("citation symlink changed while reading")
		}
		return []byte(target), true, nil
	case info.IsDir():
		out, err := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--verify", "HEAD").Output()
		if err != nil {
			return nil, false, fmt.Errorf("citation directory is not a readable gitlink: %w", err)
		}
		return []byte(strings.TrimSpace(string(out))), true, nil
	case !info.Mode().IsRegular():
		return nil, false, fmt.Errorf("citation target has unsupported file type")
	case info.Size() > maxCitationBytes:
		return nil, false, fmt.Errorf("citation target exceeds 16 MiB")
	}
	file, err := OpenRegularNoFollow(path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, false, fmt.Errorf("citation target changed type or identity while opening")
	}
	body, err := io.ReadAll(io.LimitReader(file, maxCitationBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(body) > maxCitationBytes {
		return nil, false, fmt.Errorf("citation target exceeds 16 MiB")
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(opened, after) || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return nil, false, fmt.Errorf("citation target changed while reading")
	}
	return body, false, nil
}

func readBaseReference(ctx context.Context, root, baseSHA, reference string) ([]byte, bool, error) {
	tree, err := exec.CommandContext(ctx, "git", "-C", root, "ls-tree", "-z", baseSHA, "--", reference).Output()
	if err != nil {
		return nil, false, fmt.Errorf("inspect deleted base path: %w", err)
	}
	metadata, path, ok := strings.Cut(strings.TrimSuffix(string(tree), "\x00"), "\t")
	fields := strings.Fields(metadata)
	if !ok || path != reference || len(fields) != 3 {
		return nil, false, fmt.Errorf("deleted path is absent from the base")
	}
	mode, objectType, object := fields[0], fields[1], fields[2]
	if mode == "160000" && objectType == "commit" {
		return []byte(object), true, nil
	}
	if objectType != "blob" {
		return nil, false, fmt.Errorf("deleted base path has unsupported git type %s", objectType)
	}
	sizeBody, err := exec.CommandContext(ctx, "git", "-C", root, "cat-file", "-s", object).Output()
	if err != nil {
		return nil, false, fmt.Errorf("size deleted base blob: %w", err)
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(sizeBody)), 10, 64)
	if err != nil || size < 0 || size > maxCitationBytes {
		return nil, false, fmt.Errorf("deleted base blob exceeds 16 MiB or has invalid size")
	}
	body, err := exec.CommandContext(ctx, "git", "-C", root, "cat-file", "blob", object).Output()
	if err != nil {
		return nil, false, fmt.Errorf("read deleted base blob: %w", err)
	}
	if int64(len(body)) != size {
		return nil, false, fmt.Errorf("deleted base blob size changed while reading")
	}
	return body, mode == "120000", nil
}

type fence struct {
	marker byte
	length int
}

func visibleLines(markdown string) []string {
	lines := strings.Split(markdown, "\n")
	out := make([]string, 0, len(lines))
	var active fence
	inComment := false
	for _, raw := range lines {
		if active.length > 0 {
			if closesFence(raw, active) {
				active = fence{}
			}
			continue
		}
		line := stripHTMLComments(raw, &inComment)
		if isIndentedCode(line) {
			continue
		}
		if opened, ok := opensFence(line); ok {
			active = opened
			continue
		}
		out = append(out, line)
	}
	return out
}

func stripHTMLComments(line string, inComment *bool) string {
	var out strings.Builder
	for len(line) > 0 {
		if *inComment {
			end := strings.Index(line, "-->")
			if end < 0 {
				return out.String()
			}
			line = line[end+3:]
			*inComment = false
			continue
		}
		start := strings.Index(line, "<!--")
		if start < 0 {
			out.WriteString(line)
			break
		}
		out.WriteString(line[:start])
		line = line[start+4:]
		*inComment = true
	}
	return out.String()
}

func isIndentedCode(line string) bool {
	return strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ")
}

func opensFence(line string) (fence, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 3 || trimmed[0] != '`' && trimmed[0] != '~' {
		return fence{}, false
	}
	length := markerRun(trimmed, trimmed[0])
	if length < 3 {
		return fence{}, false
	}
	return fence{marker: trimmed[0], length: length}, true
}

func closesFence(line string, active fence) bool {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < active.length || trimmed[0] != active.marker {
		return false
	}
	run := markerRun(trimmed, active.marker)
	return run >= active.length && strings.TrimSpace(trimmed[run:]) == ""
}

func markerRun(value string, marker byte) int {
	for i := 0; i < len(value); i++ {
		if value[i] != marker {
			return i
		}
	}
	return len(value)
}

func levelTwoHeading(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || !strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")), true
}
