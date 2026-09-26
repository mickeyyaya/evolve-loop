package triagedecision

import (
	"regexp"
	"strings"
)

// idSlugRE rejects non-slug ids: promotion moves an id out of the inbox, so an id parsed from prose must never
// pass.
var idSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// A files= field ends at the next ", key=" field, so its value may itself contain commas.
var (
	filesFieldRE        = regexp.MustCompile(`\bfiles\s*[=:]\s*`)
	nextMetadataFieldRE = regexp.MustCompile(`,\s*[A-Za-z_][A-Za-z0-9_]*=`)
	dropReasonRE        = regexp.MustCompile(`reason=(.*)$`)
	commentRE           = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// Item is one "- {id}: {rest}" card with a slug id.
type Item struct {
	ID, Rest string
}

// Section is one bucket of the report as written.
type Section struct {
	Items []Item
	// Rejected are bullets without a slug id and a colon.
	Rejected []string
	// Prose are non-blank lines that are neither bullets nor a "(none …)" line.
	Prose []string
	// None reports a "(none …)" line, which states an empty bucket.
	None bool
}

// SectionBody returns the text under the "## <heading>" line, up to the next heading. The heading may carry
// trailing words ("## top_n (commit to THIS cycle)").
func SectionBody(report, heading string) (string, bool) {
	prefix := "## " + heading
	lines := strings.SplitAfter(report, "\n")
	start := -1
	for i, line := range lines {
		if isHeading(line, prefix) {
			start = i
			break
		}
	}
	if start < 0 {
		return "", false
	}
	var b strings.Builder
	for _, line := range lines[start+1:] {
		if strings.HasPrefix(line, "## ") {
			break
		}
		b.WriteString(line)
	}
	return b.String(), true
}

func isHeading(line, prefix string) bool {
	rest, ok := strings.CutPrefix(line, prefix)
	if !ok {
		return false
	}
	rest = strings.TrimRight(rest, "\r\n")
	return rest == "" || !isWordByte(rest[0])
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// ParseSection reads a bucket's lines. HTML comments are not content.
func ParseSection(body string) Section {
	var s Section
	for _, raw := range strings.Split(commentRE.ReplaceAllString(body, ""), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
		case strings.HasPrefix(line, "(none"):
			s.None = true
		case strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* "):
			id, rest, ok := splitID(strings.TrimSpace(line[2:]))
			if !ok {
				s.Rejected = append(s.Rejected, line)
				continue
			}
			s.Items = append(s.Items, Item{ID: id, Rest: rest})
		default:
			s.Prose = append(s.Prose, line)
		}
	}
	return s
}

func splitID(text string) (id, rest string, ok bool) {
	i := strings.IndexByte(text, ':')
	if i < 0 {
		return "", "", false
	}
	id = strings.TrimSpace(text[:i])
	if !idSlugRE.MatchString(id) {
		return "", "", false
	}
	return id, strings.TrimSpace(text[i+1:]), true
}

// ActionOf is the text before the em-dash metadata separator ("{action} — priority=…").
func ActionOf(rest string) string {
	if i := strings.Index(rest, "—"); i >= 0 {
		return strings.TrimSpace(rest[:i])
	}
	return strings.TrimSpace(rest)
}

// ReasonOf is the value of a dropped card's reason= field.
func ReasonOf(rest string) string {
	if m := dropReasonRE.FindStringSubmatch(rest); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// SplitDeclaredFiles returns every files= field's tokens and the card with those fields removed. A field spans
// to the next ", key=" or end of line, because agents separate paths with spaces and commas too.
func SplitDeclaredFiles(rest string) (tokens []string, stripped string) {
	var kept strings.Builder
	remaining := rest
	for {
		start := filesFieldRE.FindStringIndex(remaining)
		if start == nil {
			kept.WriteString(remaining)
			break
		}
		value := remaining[start[1]:]
		end := len(value)
		if m := nextMetadataFieldRE.FindStringIndex(value); m != nil {
			end = m[0]
		}
		if nl := strings.IndexByte(value[:end], '\n'); nl >= 0 {
			end = nl
		}
		tokens = append(tokens, strings.FieldsFunc(value[:end], isFilesSeparator)...)
		kept.WriteString(remaining[:start[0]])
		kept.WriteString(" ")
		remaining = value[end:]
	}
	return tokens, kept.String()
}

func isFilesSeparator(r rune) bool {
	return r == ';' || r == ',' || r == ' ' || r == '\t'
}

// FilesOf keeps only usable repo-relative paths, since the planner matches exactly.
func FilesOf(rest string) []string {
	tokens, _ := SplitDeclaredFiles(rest)
	var files []string
	seen := map[string]bool{}
	for _, tok := range tokens {
		p, ok := DeclaredFilePath(tok)
		if !ok || seen[p] {
			continue
		}
		seen[p] = true
		files = append(files, p)
	}
	return files
}

// DeclaredFilePath trims agent punctuation and rejects placeholders, globs, absolute or ".." paths, and bare
// names.
func DeclaredFilePath(tok string) (string, bool) {
	p := strings.Trim(tok, "[]()\"'`,;:. \t")
	if p == "" || strings.ContainsAny(p, "{}*?<>") {
		return "", false
	}
	if strings.HasPrefix(p, "/") || strings.Contains(p, "..") || !strings.Contains(p, "/") {
		return "", false
	}
	return p, true
}
