package triagedecision

import (
	"encoding/json"
	"strings"
	"testing"
)

// report1707 is cycle 1707's triage-report.md, whose decision file the phase never wrote.
const report1707 = `<!-- challenge-token: 2bb098b9e2c61eb0 -->
<!-- ANCHOR:triage_decision -->
# Triage Decision — Cycle 1707

cycle_size_estimate: small
deliverable_kind: code
phase_skip: []

## top_n (commit to THIS cycle)
- lineage-datestamp-normalization: extend LineageKey to strip date-shaped runs (YYYY[-MM[-DD]]) in addition to the first dotted numeric run

## deferred (carry to NEXT cycle's carryoverTodos)
(none — this cycle is scoped solely to the assigned fleet task)

## dropped (rejected with reason)
(none — fleet_scope restricts this lane to lineage-datestamp-normalization only)

## carryoverTodos warnings (if any)
(none)

## Rationale
Fleet scope pins this lane to the single item.
`

// personaReport follows the persona's template: metadata tails, a files= footprint, a superseded section with
// its template comment.
const personaReport = `# Triage Decision — Cycle 9

cycle_size_estimate: small
deliverable_kind: code
phase_skip: ["tdd"]

## top_n (commit to THIS cycle)
- modelcatalog-write-error-paths: Cover store.Write error branches — priority=H, files=go/internal/modelcatalog/store.go;go/internal/modelcatalog/store_test.go, evidence=scout direct, source=scout

## deferred (carry to NEXT cycle's carryoverTodos)
- ledger-seal-io-coverage: Cover writeSegment branches — priority=M, defer_reason=package variety

## dropped (rejected with reason)
- cycle-311-failed-scout: Bridge artifact timeout — reason=stale; infrastructure transient failure

## superseded (retire an inbox item by id)
- old-item: shipped as new-item in cycle 8 — the change is on HEAD
- old-item: listed twice by mistake
<!-- OPTIONAL. List an inbox item ONLY when its underlying work is already on HEAD -->

## Rationale
One floor this cycle.
`

const topN1707 = "- lineage-datestamp-normalization: extend LineageKey to strip date-shaped runs (YYYY[-MM[-DD]]) in addition to the first dotted numeric run"

func decode(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("document is not JSON: %v\n%s", err, raw)
	}
	return doc
}

func derive(t *testing.T, report string, pin []string) map[string]json.RawMessage {
	t.Helper()
	raw, err := Derive([]byte(report), 1707, pin)
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	return decode(t, raw)
}

func compact(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func TestDerive_ReadsEveryBucketOfTheReport(t *testing.T) {
	doc := derive(t, report1707, []string{"lineage-datestamp-normalization"})

	for key, want := range map[string]string{
		"cycle":                     `1707`,
		"top_n":                     `[{"action":"extend LineageKey to strip date-shaped runs (YYYY[-MM[-DD]]) in addition to the first dotted numeric run","id":"lineage-datestamp-normalization"}]`,
		"deferred":                  `[]`,
		"dropped":                   `[]`,
		"superseded":                `[]`,
		"phase_skip":                `[]`,
		"projected_by_orchestrator": `true`,
	} {
		if got := compact(t, doc[key]); got != want {
			t.Errorf("%s = %s, want %s", key, got, want)
		}
	}
}

func TestDerive_ReadsThePersonasMetadataTails(t *testing.T) {
	doc := derive(t, personaReport, nil)

	for key, want := range map[string]string{
		"top_n":      `[{"action":"Cover store.Write error branches","files":["go/internal/modelcatalog/store.go","go/internal/modelcatalog/store_test.go"],"id":"modelcatalog-write-error-paths"}]`,
		"deferred":   `[{"id":"ledger-seal-io-coverage"}]`,
		"dropped":    `[{"id":"cycle-311-failed-scout","reason":"stale; infrastructure transient failure"}]`,
		"superseded": `["old-item"]`,
		"phase_skip": `["tdd"]`,
	} {
		if got := compact(t, doc[key]); got != want {
			t.Errorf("%s = %s, want %s", key, got, want)
		}
	}
}

func TestDerive_NeverInventsWhatTheReportDoesNotState(t *testing.T) {
	doc := derive(t, report1707, nil)

	for _, key := range []string{"committed_floors", "deferred_floors", "skip_shipped", "skip_rejected", "escalate_block", "unified_commitment"} {
		if _, present := doc[key]; present {
			t.Errorf("%s is present, but the report cannot say it", key)
		}
	}
	if strings.Contains(string(doc["top_n"]), `"files"`) {
		t.Error("a footprint the report does not declare must not be invented")
	}
}

// An absent deferred, dropped or superseded section commits more, never less, so it is empty; the commitment
// itself must be stated.
func TestDerive_TreatsAnAbsentOptionalBucketAsEmpty(t *testing.T) {
	report := "# Triage\n\nphase_skip: []\n\n## top_n\n" + topN1707 + "\n"

	doc := derive(t, report, []string{"lineage-datestamp-normalization"})

	for _, key := range []string{"deferred", "dropped", "superseded"} {
		if got := compact(t, doc[key]); got != `[]` {
			t.Errorf("%s = %s, want []", key, got)
		}
	}
}

// A decision document is the phase's judgment: the host derives it only when every present bucket is stated
// as cards, and the commitment is stated at all.
func TestDerive_DeclinesAnIncompleteOrUnreadableReport(t *testing.T) {
	replace := func(old, new string) string { return strings.Replace(report1707, old, new, 1) }
	noneDeferred := "(none — this cycle is scoped solely to the assigned fleet task)"
	for name, report := range map[string]string{
		"no top_n section":                     replace("## top_n (commit to THIS cycle)", "## chosen"),
		"a heading without a space":            replace("## top_n (commit to THIS cycle)", "##top_n"),
		"a bullet without a colon":             replace(noneDeferred, "- later-item"),
		"a bullet with no id":                  replace("- lineage-datestamp-normalization:", "- :"),
		"a bullet with a non-slug id":          replace(noneDeferred, "- Later_Item: do it later"),
		"prose where a bucket should be":       replace(noneDeferred, "we will see next cycle"),
		"prose mixed into a bucket":            replace(noneDeferred, "- later-item: do it later\nand maybe more"),
		"cards and none together":              replace(noneDeferred, "- later-item: do it later\n(none)"),
		"a present bucket stating nothing":     replace(noneDeferred, ""),
		"an empty top_n stated as prose":       replace(topN1707, "nothing fits this cycle"),
		"a superseded section holding prose":   replace("## Rationale", "## superseded\nnothing to retire\n\n## Rationale"),
		"a phase_skip header that is no array": replace("phase_skip: []", `phase_skip: {"tdd": true}`),
		"empty":                                "",
	} {
		if _, err := Derive([]byte(report), 1707, nil); err == nil {
			t.Errorf("%s: must decline", name)
		}
	}
}

func TestDerive_AcceptsTheHeadingVariantsAgentsWrite(t *testing.T) {
	for name, report := range map[string]string{
		"a bare heading":         strings.Replace(report1707, "## top_n (commit to THIS cycle)", "## top_n", 1),
		"a heading with a colon": strings.Replace(report1707, "## top_n (commit to THIS cycle)", "## top_n:", 1),
		"CRLF line endings":      strings.ReplaceAll(report1707, "\n", "\r\n"),
		"asterisk bullets":       strings.Replace(report1707, "- lineage-datestamp", "* lineage-datestamp", 1),
	} {
		doc := derive(t, report, []string{"lineage-datestamp-normalization"})
		if !strings.Contains(string(doc["top_n"]), `"lineage-datestamp-normalization"`) {
			t.Errorf("%s: top_n = %s", name, doc["top_n"])
		}
	}
}

func TestDerive_AcceptsAnExplicitlyEmptyCommitment(t *testing.T) {
	doc := derive(t, strings.Replace(report1707, topN1707, "(none — the backlog is drained)", 1), nil)

	if got := compact(t, doc["top_n"]); got != `[]` {
		t.Errorf("top_n = %s, want an explicit empty commitment", got)
	}
}

// The lane pin is the lane's assignment; a derived commitment that drops a pinned item would let the host
// skip work the lane was given.
func TestDerive_DeclinesWhenAPinnedItemIsNotCommitted(t *testing.T) {
	_, err := Derive([]byte(report1707), 1707, []string{"lineage-datestamp-normalization", "second-pinned-item"})

	if err == nil || !strings.Contains(err.Error(), "second-pinned-item") {
		t.Fatalf("a missing pinned item must decline and be named, got %v", err)
	}
}

// Project is ship's lenient reading: a missing or unreadable section is empty, and it never declines.
func TestProject_NeverDeclines(t *testing.T) {
	for name, report := range map[string]string{
		"only a commitment": "# Triage\n\n## top_n\n- only-item: do it — files=go/a.go\n",
		"prose in a bucket": "# Triage\n\n## top_n\nnothing parses here\n\n## deferred\n- x: y\n",
		"empty":             "",
	} {
		raw, err := Project(report, 5)
		if err != nil {
			t.Fatalf("%s: Project: %v", name, err)
		}
		doc := decode(t, raw)
		if compact(t, doc["cycle"]) != "5" || compact(t, doc["projected_by_orchestrator"]) != "true" {
			t.Errorf("%s: projected = %s", name, raw)
		}
		for _, key := range []string{"top_n", "deferred", "dropped", "superseded"} {
			if !strings.HasPrefix(strings.TrimSpace(string(doc[key])), "[") {
				t.Errorf("%s: %s = %s, want an array", name, key, doc[key])
			}
		}
	}
}

func TestProject_ReadsTheCardsItCan(t *testing.T) {
	raw, err := Project("# Triage\n\n## top_n\n- only-item: do it — files=go/a.go\n\n## deferred\n- x: y\n", 5)
	if err != nil {
		t.Fatal(err)
	}
	doc := decode(t, raw)
	if got := compact(t, doc["top_n"]); got != `[{"action":"do it","files":["go/a.go"],"id":"only-item"}]` {
		t.Errorf("top_n = %s", got)
	}
	if got := compact(t, doc["deferred"]); got != `[{"id":"x"}]` {
		t.Errorf("deferred = %s", got)
	}
}
