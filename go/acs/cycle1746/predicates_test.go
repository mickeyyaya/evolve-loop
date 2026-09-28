//go:build acs

// Package cycle1746 holds the acceptance predicates for design-doc-status-pass:
// every component row of docs/architecture/logic-first-delivery-design.md §7
// states the status its inbox record or merge commit proves, and no bullet in
// the document is duplicated.
package cycle1746

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	docRel        = "docs/architecture/logic-first-delivery-design.md"
	baseCommit    = "cc16fb89"
	p3MergeCommit = "abc9ced0"
)

var (
	subsectionHeading = regexp.MustCompile(`^### (7\.\d+) `)
	componentID       = regexp.MustCompile(`^[A-Z]\d+[a-z]?$`)
	designDocCitation = regexp.MustCompile(`design doc §(7\.\d+) ([A-Z]\d+[a-z]?)\)`)
	dossierName       = regexp.MustCompile(`^cycle-(\d+)\.json$`)
	boldLabel         = regexp.MustCompile(`^[-*] (\*\*[^*]+\*\*)`)
	prSuffix          = regexp.MustCompile(`\(#(\d+)\)\s*$`)
)

type rowKey struct{ section, id string }

func (k rowKey) String() string { return "§" + k.section + " " + k.id }

type componentRow struct {
	component, status, files string
	count                    int
}

type citation struct {
	key      rowKey
	recordID string
	file     string
	consumed bool
}

func repoRoot(t *testing.T) string {
	t.Helper()
	return acsassert.RepoRoot(t)
}

func docLines(t *testing.T, root string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, docRel))
	if err != nil {
		t.Fatalf("read %s: %v", docRel, err)
	}
	return strings.Split(string(data), "\n")
}

func baseDocLines(t *testing.T, root string) []string {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", baseCommit+":"+docRel)
	if err != nil || code != 0 {
		t.Fatalf("git show %s:%s exit=%d err=%v: %s", baseCommit, docRel, code, err, stderr)
	}
	return strings.Split(out, "\n")
}

func sectionLines(lines []string, headingPrefix string) []string {
	var out []string
	in := false
	for _, l := range lines {
		if strings.HasPrefix(l, "## ") {
			if in {
				break
			}
			in = strings.HasPrefix(l, headingPrefix)
			continue
		}
		if in {
			out = append(out, l)
		}
	}
	return out
}

func headerLines(lines []string) []string {
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(l, "## ") {
			break
		}
		out = append(out, l)
	}
	return out
}

func tableCells(line string) []string {
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

func componentRows(lines []string) map[rowKey]componentRow {
	rows := map[rowKey]componentRow{}
	section := ""
	for _, l := range sectionLines(lines, "## 7. ") {
		if m := subsectionHeading.FindStringSubmatch(l); m != nil {
			section = m[1]
			continue
		}
		if !strings.HasPrefix(l, "| ") {
			continue
		}
		cells := tableCells(l)
		if len(cells) < 6 || !componentID.MatchString(cells[1]) {
			continue
		}
		k := rowKey{section, cells[1]}
		r := rows[k]
		r.component, r.status, r.files = cells[2], cells[len(cells)-3], cells[len(cells)-2]
		r.count++
		rows[k] = r
	}
	return rows
}

func onlyRow(t *testing.T, rows map[rowKey]componentRow, k rowKey) (componentRow, bool) {
	t.Helper()
	r, ok := rows[k]
	if !ok || r.count != 1 {
		t.Errorf("RED: %s must be exactly one §7 table row, found %d", k, r.count)
		return componentRow{}, false
	}
	return r, true
}

func readCitations(t *testing.T, root, dir string, consumed bool) []citation {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, dir, "*.json"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	var out []citation
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if !bytes.Contains(data, []byte("design doc §7.")) {
			continue
		}
		var rec struct{ ID, Source string }
		if err := json.Unmarshal(data, &rec); err != nil {
			t.Fatalf("decode %s: %v", p, err)
		}
		m := designDocCitation.FindStringSubmatch(rec.Source)
		if m == nil {
			continue
		}
		out = append(out, citation{rowKey{m[1], m[2]}, rec.ID, filepath.Base(p), consumed})
	}
	return out
}

func inboxCitations(t *testing.T, root string) (consumed, pending []citation) {
	t.Helper()
	return readCitations(t, root, ".evolve/inbox/consumed", true),
		readCitations(t, root, ".evolve/inbox", false)
}

func passCyclesFor(t *testing.T, root, recordID string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, "knowledge-base/cycles/cycle-*.json"))
	if err != nil {
		t.Fatalf("glob dossiers: %v", err)
	}
	quoted := []byte(`"` + recordID + `"`)
	var cycles []string
	for _, p := range paths {
		m := dossierName.FindStringSubmatch(filepath.Base(p))
		if m == nil {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if !bytes.Contains(data, quoted) {
			continue
		}
		var d struct {
			Tasks        []string `json:"tasks"`
			FinalVerdict string   `json:"final_verdict"`
		}
		if err := json.Unmarshal(data, &d); err != nil {
			t.Fatalf("decode dossier %s: %v", p, err)
		}
		if d.FinalVerdict != "PASS" {
			continue
		}
		for _, task := range d.Tasks {
			if task == recordID {
				cycles = append(cycles, m[1])
			}
		}
	}
	return cycles
}

func claimsShipped(status string) bool {
	s := strings.ToLower(status)
	return strings.HasPrefix(s, "shipped") || strings.HasPrefix(s, "merged")
}

func citesCycle(status string, cycles []string) bool {
	for _, c := range cycles {
		if regexp.MustCompile(`\bcycles? ` + c + `\b`).MatchString(status) {
			return true
		}
	}
	return false
}

func keysOf(cs []citation) map[rowKey]bool {
	out := map[rowKey]bool{}
	for _, c := range cs {
		out[c.key] = true
	}
	return out
}

// acs-predicate: config-check — the deliverable is the design document's text;
// the expected status and cycle are derived from the consumed inbox records and
// the PASS dossiers, never written into this predicate.
func TestC1746_001_ConsumedComponentRowsReadShippedWithTheirCycle(t *testing.T) {
	root := repoRoot(t)
	consumed, _ := inboxCitations(t, root)
	have := keysOf(consumed)
	for _, k := range []rowKey{{"7.7", "W8"}, {"7.9", "G2"}, {"7.9", "G3"}} {
		if !have[k] {
			t.Fatalf("oracle lost: no consumed inbox record cites %s (found %d consumed citations)", k, len(consumed))
		}
	}
	rows := componentRows(docLines(t, root))
	for _, c := range consumed {
		r, ok := onlyRow(t, rows, c.key)
		if !ok {
			continue
		}
		cycles := passCyclesFor(t, root, c.recordID)
		if len(cycles) == 0 {
			t.Errorf("RED: %s — consumed record %s has no PASS dossier listing %q", c.key, c.file, c.recordID)
			continue
		}
		if !claimsShipped(r.status) || !citesCycle(r.status, cycles) {
			t.Errorf("RED: %s reads %q; its record %s was consumed by ship in cycle %s, so the status must start with shipped and cite that cycle",
				c.key, r.status, c.recordID, strings.Join(cycles, "/"))
		}
	}
}

// acs-predicate: config-check — the rows are derived from the pending inbox
// records; a status pass that stamps every row shipped fails here.
func TestC1746_002_PendingComponentRowsDoNotClaimShipped(t *testing.T) {
	root := repoRoot(t)
	_, pending := inboxCitations(t, root)
	if len(pending) == 0 {
		t.Fatalf("oracle lost: no pending inbox record cites a design doc §7 row")
	}
	rows := componentRows(docLines(t, root))
	for _, c := range pending {
		r, ok := onlyRow(t, rows, c.key)
		if !ok {
			continue
		}
		if claimsShipped(r.status) {
			t.Errorf("RED: %s reads %q but its inbox record %s is still pending, so nothing shipped it", c.key, r.status, c.file)
		}
	}
	t.Logf("checked %d pending-record rows", len(pending))
}

// acs-predicate: config-check — G2's status change must keep the §5.13 ceiling
// fact the row already records.
func TestC1746_003_G2KeepsItsCeilingNote(t *testing.T) {
	rows := componentRows(docLines(t, repoRoot(t)))
	r, ok := onlyRow(t, rows, rowKey{"7.9", "G2"})
	if !ok {
		return
	}
	if !strings.Contains(r.status, "an allowance is a ceiling (§5.13)") {
		t.Errorf("RED: §7.9 G2 dropped its ceiling note: %q", r.status)
	}
}

// acs-predicate: config-check — the PR number is read from the merge commit on
// HEAD, the row is P3's own status cell.
func TestC1746_004_P3RowMatchesItsMergeCommit(t *testing.T) {
	root := repoRoot(t)
	if _, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "--is-ancestor", p3MergeCommit, "HEAD"); err != nil || code != 0 {
		t.Fatalf("oracle lost: %s is not an ancestor of HEAD (exit=%d err=%v): %s", p3MergeCommit, code, err, stderr)
	}
	subject, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "log", "-1", "--format=%s", p3MergeCommit)
	if err != nil || code != 0 {
		t.Fatalf("git log %s exit=%d err=%v: %s", p3MergeCommit, code, err, stderr)
	}
	m := prSuffix.FindStringSubmatch(subject)
	if m == nil || !strings.Contains(subject, "P3") {
		t.Fatalf("oracle lost: %s subject %q does not name P3 and its PR", p3MergeCommit, subject)
	}
	rows := componentRows(docLines(t, root))
	r, ok := onlyRow(t, rows, rowKey{"7.1", "P3"})
	if !ok {
		return
	}
	if strings.Contains(strings.ToLower(r.status), "pending") || !strings.Contains(r.status, "#"+m[1]) {
		t.Errorf("RED: §7.1 P3 reads %q; its PR #%s merged as %s, so the status must cite #%s and say nothing is pending",
			r.status, m[1], p3MergeCommit, m[1])
	}
}

type bulletList struct {
	start int
	items []string
}

func bulletLists(lines []string) []bulletList {
	var lists []bulletList
	var cur *bulletList
	fenced := false
	for i, l := range lines {
		if strings.HasPrefix(l, "```") {
			fenced = !fenced
			cur = nil
			continue
		}
		if fenced {
			continue
		}
		trimmed := strings.TrimSpace(l)
		isBullet := strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ")
		switch {
		case isBullet:
			if cur == nil {
				lists = append(lists, bulletList{start: i + 1})
				cur = &lists[len(lists)-1]
			}
			cur.items = append(cur.items, trimmed)
		case strings.HasPrefix(l, "  ") && cur != nil:
		default:
			cur = nil
		}
	}
	return lists
}

func labelOf(item string) string {
	if m := boldLabel.FindStringSubmatch(item); m != nil {
		return m[1]
	}
	return ""
}

func duplicateBullets(lines []string) []string {
	var dups []string
	seenText := map[string]int{}
	for _, list := range bulletLists(lines) {
		seenLabel := map[string]bool{}
		for _, item := range list.items {
			seenText[item]++
			if seenText[item] == 2 {
				dups = append(dups, fmt.Sprintf("identical bullet: %.90q", item))
			}
			lab := labelOf(item)
			if lab == "" {
				continue
			}
			if seenLabel[lab] {
				dups = append(dups, fmt.Sprintf("list at line %d repeats %s", list.start, lab))
			}
			seenLabel[lab] = true
		}
	}
	return dups
}

// acs-predicate: config-check — duplication is a property of the document's
// text; the check parses every bullet list rather than naming one bullet.
func TestC1746_005_NoBulletIsDuplicated(t *testing.T) {
	if dups := duplicateBullets(docLines(t, repoRoot(t))); len(dups) > 0 {
		t.Errorf("RED: %d duplicated bullet(s) remain:\n  %s", len(dups), strings.Join(dups, "\n  "))
	}
}

func countLabel(lines []string, label, mustContain string) (count int, withText bool) {
	for _, list := range bulletLists(lines) {
		for _, item := range list.items {
			if labelOf(item) == label {
				count++
				withText = withText || strings.Contains(item, mustContain)
			}
		}
	}
	return count, withText
}

// acs-predicate: config-check — removing a duplicate must leave one copy; a
// fix that deletes both copies loses the fact and fails here.
func TestC1746_006_DeduplicatedBulletsKeepOneCopy(t *testing.T) {
	lines := docLines(t, repoRoot(t))
	cases := []struct {
		name, label, mustContain string
		scope                    []string
	}{
		{"header", "**Status:**", "living document", headerLines(lines)},
		{"§12", "**One rule for a walled dispatch**", "isQuotaWall", sectionLines(lines, "## 12. ")},
	}
	for _, c := range cases {
		n, withText := countLabel(c.scope, c.label, c.mustContain)
		if n != 1 || !withText {
			t.Errorf("RED: %s must hold exactly one %s bullet naming %q, found %d (text kept: %v)", c.name, c.label, c.mustContain, n, withText)
		}
	}
}

func headings(lines []string) []string {
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(l, "## ") || strings.HasPrefix(l, "### ") {
			out = append(out, l)
		}
	}
	return out
}

func labelSet(lines []string) []string {
	set := map[string]bool{}
	for _, list := range bulletLists(lines) {
		for _, item := range list.items {
			if lab := labelOf(item); lab != "" {
				set[lab] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for lab := range set {
		out = append(out, lab)
	}
	sort.Strings(out)
	return out
}

func missingFrom(base, now []string) []string {
	have := map[string]bool{}
	for _, s := range now {
		have[s] = true
	}
	var missing []string
	for _, s := range base {
		if !have[s] {
			missing = append(missing, s)
		}
	}
	return missing
}

// acs-predicate: config-check — the status pass corrects cells and removes
// duplicates only; every heading, §7 row, component and files cell, and bullet
// label the base document had is still there.
func TestC1746_007_StatusPassKeepsTheRestOfTheDocument(t *testing.T) {
	root := repoRoot(t)
	base, now := baseDocLines(t, root), docLines(t, root)
	if missing := missingFrom(headings(base), headings(now)); len(missing) > 0 {
		t.Errorf("headings removed or renamed: %q", missing)
	}
	baseRows, nowRows := componentRows(base), componentRows(now)
	if len(baseRows) == 0 {
		t.Fatalf("oracle lost: no §7 rows parsed from %s:%s", baseCommit, docRel)
	}
	for k, b := range baseRows {
		n, ok := nowRows[k]
		switch {
		case !ok || n.count != b.count:
			t.Errorf("%s: row count %d at base, %d now", k, b.count, n.count)
		case n.component != b.component || n.files != b.files:
			t.Errorf("%s: component or files cell changed; only the status cell is in scope", k)
		}
	}
	for _, scope := range []struct {
		name      string
		base, now []string
	}{
		{"header", headerLines(base), headerLines(now)},
		{"§12", sectionLines(base, "## 12. "), sectionLines(now, "## 12. ")},
	} {
		if missing := missingFrom(labelSet(scope.base), labelSet(scope.now)); len(missing) > 0 {
			t.Errorf("%s lost bullet(s): %q", scope.name, missing)
		}
	}
}
