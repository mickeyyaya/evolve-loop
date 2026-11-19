package incidentkb_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
)

func sampleRecordDoc() string {
	return `---
schema: incident-record/1
id: 2026-11-19-sample
title: sample incident
level: record
status: fixed
selfheal: inc-2026-11-19-0001
fingerprint: sample:fp
signals: ["cycle:10:1"]
prs: [101]
items: [sample-item]
actions:
  - {id: A1, type: prevent, owner: console, item: sample-item, pr: 101, done: "done check", state: done}
effectiveness:
  metric: {code: SHIP_INTEGRATION_EJECTED, filters: [{field: paths, op: contains, value: docs/research-index.md}], unit: wave, since: abc123}
  baseline: {value: 4, units: 2}
  target: 0
  checks: 3
  results: []
verified_at: ""
---

# sample incident

## Summary

<!-- GENERATED:status BEGIN -->
<!-- GENERATED:status END -->

## Trigger and level

## Timeline

## Key logs

## Root cause and contributing factors

## Analysis

## Approaches taken

## Actions

<!-- GENERATED:actions BEGIN -->
<!-- GENERATED:actions END -->

## Effectiveness

<!-- GENERATED:effectiveness BEGIN -->
<!-- GENERATED:effectiveness END -->

## Lessons
`
}

func TestParseAndRender_BlockYAMLAndResults(t *testing.T) {
	t.Parallel()
	doc := "---\r\n" + `schema: incident-record/1 # inline comment
id: '2026-11-19-block'
title: "block: style"
level: log
status: verified
signals:
  - cycle:1:2
prs:
  - 42
items:
  - item-one
actions:
  - id: A1
    type: mitigate
    owner: lane
    item: item-one
    pr: 42
    done: "tests | green"
    state: done
effectiveness:
  metric:
    code: CODE_X
    unit: cycle
    since: deadbeef
    filters:
      - field: code
        op: eq
        value: CODE_X
  baseline:
    value: 5
    units: 1
  target: 0
  checks: 2
  results:
    - {check: 1, observed: 0, verdict: pass, at: 'it''s ok'}
verified_at: "2026-11-19T01:00:00Z"
---

## Summary

<!-- GENERATED:status BEGIN -->
<!-- GENERATED:status END -->

## Trigger and level

## Timeline

## Key logs

## Root cause and contributing factors

## Analysis

## Approaches taken

## Actions

<!-- GENERATED:actions BEGIN -->
<!-- GENERATED:actions END -->

## Effectiveness

<!-- GENERATED:effectiveness BEGIN -->
<!-- GENERATED:effectiveness END -->

## Lessons
`
	rec, err := incidentkb.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if rec.Title != "block: style" || len(rec.Actions) != 1 || len(rec.Effectiveness.Results) != 1 {
		t.Fatalf("unexpected parsed record: %+v", rec)
	}
	rendered, err := incidentkb.Render([]byte(doc))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	text := string(rendered)
	if !strings.Contains(text, "verified_at: 2026-11-19T01:00:00Z") || !strings.Contains(text, `tests \| green`) || !strings.Contains(text, "| 1 | 0 | pass | it's ok |") {
		t.Errorf("rendered output missing expected fields:\n%s", text)
	}
	if probs := incidentkb.CheckBytes("2026-11-19-block", rendered); len(probs) != 0 {
		t.Errorf("CheckBytes problems: %v", probs)
	}
}

func TestParse_ErrorsAndEdgeCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		doc  string
		want string
	}{
		{"no frontmatter", "# plain\n", "missing frontmatter"},
		{"empty frontmatter", "---\n---\n", "empty"},
		{"unclosed frontmatter", "---\nschema: incident-record/1\n", "closing"},
		{"trailing after fence", "---\nschema: incident-record/1\n--- extra\n", "trailing"},
		{"bad indent", "---\n  schema: x\n---\n", "indent"},
		{"no colon", "---\njusttext\n---\n", "key: value"},
		{"mix inline and block", "---\nsignals: [a]\n  - b\n---\n", "mix"},
		{"bad seq item", "---\nsignals:\n  - a\n  notdash\n---\n", "- item"},
		{"flow bad kv", "---\neffectiveness: {bad}\n---\n", "key: value"},
		{"unmatched brace", "---\nactions: [{id: a}] ]\n---\n", "delimiter"},
		{"unterminated quote", "---\nsignals: [\"abc]\n---\n", "unterminated"},
		{"bad int", "---\nprs: [abc]\n---\n", "integer"},
		{"non-string scalar", "---\ntitle: [a]\n---\n", "string"},
		{"non-seq list", "---\nsignals: abc\n---\n", "sequence"},
		{"non-map effectiveness", "---\neffectiveness: abc\n---\n", "mapping"},
		{"unknown top", "---\nbogus: 1\n---\n", "bogus"},
		{"unknown action", "---\nactions: [{bogus: 1}]\n---\n", "bogus"},
		{"bad action pr", "---\nactions: [{pr: x}]\n---\n", "integer"},
		{"unknown eff", "---\neffectiveness:\n  bogus: 1\n---\n", "bogus"},
		{"bad eff target", "---\neffectiveness:\n  target: x\n---\n", "integer"},
		{"unknown metric", "---\neffectiveness:\n  metric: {bogus: 1}\n---\n", "bogus"},
		{"bad metric code", "---\neffectiveness:\n  metric: {code: []}\n---\n", "string"},
		{"unknown filter", "---\neffectiveness:\n  metric:\n    filters: [{bogus: 1}]\n---\n", "bogus"},
		{"bad filter field", "---\neffectiveness:\n  metric:\n    filters: [{field: []}]\n---\n", "string"},
		{"unknown baseline", "---\neffectiveness:\n  baseline: {bogus: 1}\n---\n", "bogus"},
		{"bad baseline val", "---\neffectiveness:\n  baseline: {value: x}\n---\n", "integer"},
		{"unknown result", "---\neffectiveness:\n  results: [{bogus: 1}]\n---\n", "bogus"},
		{"bad result check", "---\neffectiveness:\n  results: [{check: x}]\n---\n", "integer"},
		{"nested indent", "---\neffectiveness:\n    metric: {}\n  target: 0\n---\n", "indent"},
		{"seq flow item error", "---\nactions:\n  - {bad}\n---\n", "key: value"},
		{"seq flow item child", "---\nactions:\n  - {id: a}\n    extra: 1\n---\n", "nested block"},
		{"nested flow seq error", "---\nactions: [{bad}]\n---\n", "key: value"},
		{"nested flow map error", "---\neffectiveness: {metric: {bad}}\n---\n", "key: value"},
		{"nested flow list quote error", "---\neffectiveness: {metric: [\"abc]}\n---\n", "unterminated"},
		{"actions non-seq", "---\nactions: x\n---\n", "sequence"},
		{"actions non-map item", "---\nactions: [x]\n---\n", "mapping"},
		{"metric non-map", "---\neffectiveness:\n  metric: x\n---\n", "mapping"},
		{"filters non-seq", "---\neffectiveness:\n  metric:\n    filters: x\n---\n", "sequence"},
		{"filters non-map item", "---\neffectiveness:\n  metric:\n    filters: [x]\n---\n", "mapping"},
		{"baseline non-map", "---\neffectiveness:\n  baseline: x\n---\n", "mapping"},
		{"results non-seq", "---\neffectiveness:\n  results: x\n---\n", "sequence"},
		{"results non-map item", "---\neffectiveness:\n  results: [x]\n---\n", "mapping"},
		{"prs non-seq", "---\nprs: x\n---\n", "sequence"},
		{"prs non-int elem", "---\nprs: [[]]\n---\n", "string"},
		{"signals non-str elem", "---\nsignals: [[]]\n---\n", "string"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if _, err := incidentkb.Parse([]byte(c.doc)); err == nil || !strings.Contains(strings.ToLower(err.Error()), c.want) {
				t.Errorf("Parse(%q) = %v, want %q", c.name, err, c.want)
			}
		})
	}
	if _, err := incidentkb.Parse([]byte("---\n---\n")); err == nil {
		t.Fatal("empty frontmatter parsed")
	}
	doc := "---\nschema: incident-record/1\nid: 2026-11-19-a\ntitle: \"a\\\"b\"\nselfheal: \"bad\\q\"\nfingerprint: 'x:y'\n\"prs\": []\n'items': []\nlevel: record\nstatus: open\nactions:\n  -\n    id: A1\neffectiveness: {}\n---\n"
	rec, err := incidentkb.Parse([]byte(doc))
	if err != nil || rec.Title != `a"b` || rec.SelfHeal != `bad\q` || len(rec.Actions) != 1 || rec.Actions[0].ID != "A1" {
		t.Fatalf("Parse edge doc = %+v, %v", rec, err)
	}
	if _, err := incidentkb.Parse([]byte("---\neffectiveness:\n---\n")); err == nil {
		t.Errorf("empty effectiveness string should fail mapping decode")
	}
}

func TestRender_EmptyAndErrorBranches(t *testing.T) {
	t.Parallel()
	emptyDoc := `---
schema: incident-record/1
id: 2026-11-19-empty
title: empty
level: record
status: open
actions: []
effectiveness: {}
---
<!-- GENERATED:status BEGIN --><!-- GENERATED:status END -->
<!-- GENERATED:actions BEGIN --><!-- GENERATED:actions END -->
<!-- GENERATED:effectiveness BEGIN --><!-- GENERATED:effectiveness END -->
`
	out, err := incidentkb.Render([]byte(emptyDoc))
	if err != nil {
		t.Fatalf("Render empty: %v", err)
	}
	if !strings.Contains(string(out), "_No actions._") || !strings.Contains(string(out), "filters: none") {
		t.Errorf("unexpected empty render:\n%s", out)
	}
	if _, err := incidentkb.Render([]byte("# no fm\n")); err == nil {
		t.Errorf("Render accepted doc without frontmatter")
	}
	if _, err := incidentkb.Render([]byte(strings.Replace(sampleRecordDoc(), "schema: incident-record/1", "schema: bad", 1))); err == nil {
		t.Errorf("Render accepted invalid schema")
	}
	noBegin := strings.Replace(sampleRecordDoc(), "<!-- GENERATED:actions BEGIN -->", "", 1)
	if _, err := incidentkb.Render([]byte(noBegin)); !errors.Is(err, incidentkb.ErrMissingMarker) {
		t.Errorf("Render missing begin marker = %v", err)
	}
	noEnd := strings.Replace(sampleRecordDoc(), "<!-- GENERATED:status END -->", "", 1)
	if _, err := incidentkb.Render([]byte(noEnd)); !errors.Is(err, incidentkb.ErrMissingMarker) {
		t.Errorf("Render missing end marker = %v", err)
	}
	overlap := `---
schema: incident-record/1
id: 2026-11-19-overlap
title: overlap
level: record
status: open
---
<!-- GENERATED:status BEGIN -->
<!-- GENERATED:actions BEGIN -->
<!-- GENERATED:status END -->
<!-- GENERATED:actions END -->
<!-- GENERATED:effectiveness BEGIN --><!-- GENERATED:effectiveness END -->
`
	if _, err := incidentkb.Render([]byte(overlap)); !errors.Is(err, incidentkb.ErrOverlappingMarkers) {
		t.Errorf("Render overlap = %v", err)
	}
}

func TestCheckAndList_AllBranches(t *testing.T) {
	t.Parallel()
	if !incidentkb.IsLegacy("cycle-102.md", []byte("# old")) || incidentkb.IsLegacy("REGRESSION-COVERAGE-INDEX.md", []byte("# idx")) {
		t.Errorf("IsLegacy mismatch")
	}
	if probs := incidentkb.CheckBytes("2026-11-19-none", []byte("# none\n")); len(probs) == 0 {
		t.Errorf("expected missing frontmatter")
	}
	badRec := `---
schema: bad
id: ""
title: ""
level: urgent
status: banana
effectiveness:
  metric: {unit: hour, filters: [{field: a, op: nope, value: b}]}
---
<!-- GENERATED:status BEGIN -->drift<!-- GENERATED:status END -->
<!-- GENERATED:actions BEGIN -->drift<!-- GENERATED:actions END -->
`
	probs := incidentkb.CheckBytes("2026-11-19-x", []byte(badRec))
	if p2 := incidentkb.CheckBytes("2026-11-19-x", []byte(strings.Replace(badRec, `id: ""`, "id: 2026-11-19-other", 1))); !strings.Contains(strings.Join(p2, ";"), "does not match") {
		t.Errorf("expected id mismatch, got %v", p2)
	}
	if p3 := incidentkb.CheckBytes("2026-11-19-x", []byte(strings.Replace(badRec, `id: ""`, "id: bad_id", 1))); !strings.Contains(strings.Join(p3, ";"), "invalid id") {
		t.Errorf("expected invalid id, got %v", p3)
	}
	joined := strings.Join(probs, "\n")
	for _, want := range []string{"schema", "id", "title", "level", "status", "unit", "op", "Summary", "effectiveness", "rendered status"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in problems:\n%s", want, joined)
		}
	}
	rendered, err := incidentkb.Render([]byte(sampleRecordDoc()))
	if err != nil {
		t.Fatal(err)
	}
	swapped := strings.Replace(string(rendered), "## Analysis\n\n## Approaches taken\n", "## Approaches taken\n\n## Analysis\n", 1)
	if p := incidentkb.CheckBytes("2026-11-19-sample", []byte(swapped)); !strings.Contains(strings.Join(p, ";"), "out of order") {
		t.Errorf("expected out of order, got %v", p)
	}

	root := t.TempDir()
	if rep, err := incidentkb.CheckAll(root); err != nil || len(rep.Records) != 0 {
		t.Errorf("empty CheckAll = %+v, %v", rep, err)
	}
	if rows, err := incidentkb.List(root, "", ""); err != nil || len(rows) != 0 {
		t.Errorf("empty List = %+v, %v", rows, err)
	}
	if _, _, err := incidentkb.Check(root, "bad"); err == nil {
		t.Errorf("Check accepted bad id")
	}
	if _, _, err := incidentkb.Check(root, "2026-11-19-missing"); err == nil {
		t.Errorf("Check accepted missing file")
	}
	dir := incidentkb.Dir(root)
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "REGRESSION-COVERAGE-INDEX.md"), []byte("# idx\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-11-18-old.md"), []byte("# old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-11-19-sample.md"), rendered, 0o600); err != nil {
		t.Fatal(err)
	}
	if rep, err := incidentkb.CheckAll(root); err != nil || len(rep.Records) != 1 || len(rep.Legacy) != 1 {
		t.Errorf("CheckAll with skipped = %+v, %v", rep, err)
	}
	if _, legacy, err := incidentkb.Check(root, "2026-11-18-old"); err != nil || !legacy {
		t.Errorf("Check legacy = %v, %v", legacy, err)
	}
	if rows, err := incidentkb.List(root, "open", ""); err != nil || len(rows) != 0 {
		t.Errorf("List status open = %+v, %v", rows, err)
	}
	if rows, err := incidentkb.List(root, "", "OTHER"); err != nil || len(rows) != 0 {
		t.Errorf("List code OTHER = %+v, %v", rows, err)
	}
	if rows, err := incidentkb.List(root, "", ""); err != nil || len(rows) != 2 {
		t.Errorf("List all = %+v, %v", rows, err)
	}

	unreadableDir := filepath.Join(dir, "2026-11-19-bad.md")
	if err := os.WriteFile(unreadableDir, []byte("---\nbogus: 1\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := incidentkb.List(root, "", ""); err == nil {
		t.Errorf("List accepted invalid frontmatter")
	}
	_ = os.Remove(unreadableDir)
	if err := os.Chmod(filepath.Join(dir, "2026-11-18-old.md"), 0o000); err == nil {
		if _, err := incidentkb.CheckAll(root); err == nil && os.Geteuid() != 0 {
			t.Errorf("CheckAll ignored unreadable file")
		}
		if _, err := incidentkb.List(root, "", ""); err == nil && os.Geteuid() != 0 {
			t.Errorf("List ignored unreadable file")
		}
		_ = os.Chmod(filepath.Join(dir, "2026-11-18-old.md"), 0o600)
	}

	fileAsDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(fileAsDir, "docs"), 0o750)
	_ = os.WriteFile(incidentkb.Dir(fileAsDir), []byte("x"), 0o600)
	if _, err := incidentkb.CheckAll(fileAsDir); err == nil {
		t.Errorf("CheckAll accepted non-dir")
	}
	if _, err := incidentkb.List(fileAsDir, "", ""); err == nil {
		t.Errorf("List accepted non-dir")
	}
}
