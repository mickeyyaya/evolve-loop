package incidentkb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

var (
	validIDRE     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-[a-z0-9][a-z0-9-]*$`)
	sessionLogRE  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-session-log\.md$`)
	datedPrefixRE = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-`)
)

func ValidID(id string) bool { return validIDRE.MatchString(id) }

func IsSkipped(filename string) bool {
	base := filepath.Base(filename)
	return base == "REGRESSION-COVERAGE-INDEX.md" || sessionLogRE.MatchString(base)
}

func IsRecordFile(filename string) bool {
	return strings.HasSuffix(filename, ".md") && !IsSkipped(filename)
}

func IsLegacy(filename string, raw []byte) bool {
	if IsSkipped(filename) {
		return false
	}
	if _, _, hasFM, _ := splitFrontmatter(raw); hasFM {
		return false
	}
	m := datedPrefixRE.FindStringSubmatch(filepath.Base(filename))
	if len(m) != 2 {
		return true
	}
	return m[1] < LegacyCutoff
}

func Dir(root string) string {
	return filepath.Join(root, filepath.FromSlash(RelDir))
}

func RecordPath(root, id string) string {
	return filepath.Join(Dir(root), id+".md")
}

func CheckBytes(id string, raw []byte) []string {
	rec, err := Parse(raw)
	if err != nil {
		return []string{err.Error()}
	}
	problems := validateFrontmatter(rec, id)
	problems = append(problems, validateSections(rec.Body)...)
	problems = append(problems, validateRenderedParts(string(raw), rec)...)
	return problems
}

func Check(root, id string) ([]string, bool, error) {
	if !ValidID(id) {
		return nil, false, fmt.Errorf("incident id %q must look like yyyy-mm-dd-slug", id)
	}
	path := RecordPath(root, id)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	if IsLegacy(filepath.Base(path), raw) {
		return nil, true, nil
	}
	return CheckBytes(id, raw), false, nil
}

func walkIncidentFiles(root string, fn func(name, id string, raw []byte, legacy bool) error) error {
	dir := Dir(root)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !IsRecordFile(e.Name()) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		id := strings.TrimSuffix(e.Name(), ".md")
		if err := fn(e.Name(), id, raw, IsLegacy(e.Name(), raw)); err != nil {
			return err
		}
	}
	return nil
}

func CheckAll(root string) (CheckReport, error) {
	report := CheckReport{Problems: map[string][]string{}}
	err := walkIncidentFiles(root, func(_ string, id string, raw []byte, legacy bool) error {
		if legacy {
			report.Legacy = append(report.Legacy, id)
			return nil
		}
		report.Records = append(report.Records, id)
		report.Problems[id] = CheckBytes(id, raw)
		return nil
	})
	if err != nil {
		return CheckReport{}, err
	}
	sort.Strings(report.Legacy)
	sort.Strings(report.Records)
	return report, nil
}

func List(root, statusFilter, codeFilter string) ([]ListEntry, error) {
	out := make([]ListEntry, 0)
	err := walkIncidentFiles(root, func(name, id string, raw []byte, legacy bool) error {
		if legacy {
			if statusFilter == "" && codeFilter == "" {
				out = append(out, ListEntry{ID: id, Legacy: true})
			}
			return nil
		}
		rec, err := Parse(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if statusFilter != "" && rec.Status != statusFilter {
			return nil
		}
		if codeFilter != "" && rec.Effectiveness.Metric.Code != codeFilter {
			return nil
		}
		out = append(out, ListEntry{
			ID:     id,
			Status: rec.Status,
			Legacy: false,
			Title:  rec.Title,
			Level:  rec.Level,
			Code:   rec.Effectiveness.Metric.Code,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func validateFrontmatter(rec Record, expectedID string) []string {
	var problems []string
	if rec.Schema != Schema {
		problems = append(problems, fmt.Sprintf("invalid schema %q, want %q", rec.Schema, Schema))
	}
	if rec.ID == "" {
		problems = append(problems, "missing id")
	} else if !ValidID(rec.ID) {
		problems = append(problems, fmt.Sprintf("invalid id %q (want yyyy-mm-dd-slug)", rec.ID))
	} else if expectedID != "" && rec.ID != expectedID {
		problems = append(problems, fmt.Sprintf("id %q does not match filename %q", rec.ID, expectedID))
	}
	if strings.TrimSpace(rec.Title) == "" {
		problems = append(problems, "missing title")
	}
	if rec.Level != "record" && rec.Level != "log" {
		problems = append(problems, fmt.Sprintf("invalid level %q (want record or log)", rec.Level))
	}
	if !validStatus(rec.Status) {
		problems = append(problems, fmt.Sprintf("invalid status %q", rec.Status))
	}
	problems = append(problems, validateEffectiveness(rec.Effectiveness)...)
	return problems
}

func validStatus(s string) bool {
	switch s {
	case "open", "fixed", "verified", "reopened", "unverifiable":
		return true
	default:
		return false
	}
}

func validateEffectiveness(eff Effectiveness) []string {
	var problems []string
	if u := eff.Metric.Unit; u != "" && u != "cycle" && u != "wave" {
		problems = append(problems, fmt.Sprintf("invalid effectiveness.metric.unit %q (want cycle or wave)", u))
	}
	for _, f := range eff.Metric.Filters {
		if f.Op != "eq" && f.Op != "contains" && f.Op != "prefix" {
			problems = append(problems, fmt.Sprintf("invalid effectiveness.metric.filters op %q (want eq, contains, or prefix)", f.Op))
		}
	}
	return problems
}

func validateSections(body string) []string {
	var headings []string
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "## ") {
			headings = append(headings, strings.TrimSpace(line[3:]))
		}
	}
	var problems []string
	idx := 0
	for _, req := range RequiredSections {
		found := -1
		for j := idx; j < len(headings); j++ {
			if headings[j] == req {
				found = j
				break
			}
		}
		if found >= 0 {
			idx = found + 1
			continue
		}
		if slices.Contains(headings[:idx], req) {
			problems = append(problems, fmt.Sprintf("section heading \"## %s\" is out of order", req))
		} else {
			problems = append(problems, fmt.Sprintf("missing section heading \"## %s\"", req))
		}
	}
	return problems
}

func validateRenderedParts(doc string, rec Record) []string {
	var problems []string
	for _, part := range generatedParts {
		sp, err := findMarkerSpan(doc, part)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		want := "\n" + renderPart(rec, part) + "\n"
		if doc[sp.contentS:sp.contentE] != want {
			problems = append(problems, fmt.Sprintf("rendered %s section does not match frontmatter", part))
		}
	}
	return problems
}
