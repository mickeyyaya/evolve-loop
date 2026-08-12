package deliverable

// salvage_report.go — the FIRST READER of the baseline sidecar
// salvage_instrument.go has been writing since cycle-1389.
//
// The rank-2 portfolio item `schema-aligned-salvage-layer`
// (docs/research/deliverable-alignment-2026-08/README.md §7) gates its
// extraction/coercion stage on a MEASURED recoverable-malformed rate. The
// instrumentation half landed and has appended a record per bad_verdict block
// ever since — but nothing ever read those records back, so the gate was
// blocked on a number no code computed and §7's baseline had to be produced by
// hand. This file computes it: a pure fold over the JSONL, surfaced by
// `evolve salvage report` (go/cmd/evolve/cmd_salvage.go).
//
// Still measurement, not extraction: nothing here coerces a verdict, and the
// summarizer performs no I/O of its own — it reads whatever io.Reader the
// caller opened.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// BadVerdictBaselineFile is the JSONL sidecar's basename under .evolve/. It is
// exported because the CLI surface must open the same file the writer appends
// to — one name, two callers, never two string literals that can drift.
const BadVerdictBaselineFile = "bad-verdict-baseline.jsonl"

// badVerdictEventType is the `event_type` of a classification record. The
// sidecar is shared with other emitters, so the reader MUST filter on it:
// counting a foreign event as a non-recoverable bad verdict would silently
// deflate the very rate this instrumentation exists to measure.
const badVerdictEventType = "bad_verdict_classified"

// BaselineSummary is the aggregate the extraction gate is waiting on. Rate is
// Recoverable/Total, and is 0 (never NaN) for an empty baseline — a fresh
// project root is the normal un-populated state, and a NaN serialises to
// invalid JSON, poisoning the report it appears in.
//
// ByPattern breaks the count down BY SHAPE rather than reporting one
// undifferentiated "bad output" number: the shapes have wildly different
// recovery costs, so a single headline rate cannot size the extraction stage.
// Only patterns that actually occurred appear — a non-recoverable record
// carries the empty pattern and must not manufacture a phantom bucket.
//
// Saved is the number of coercions the extraction stage ACTUALLY performed,
// folded from a different sidecar (salvage-applied.jsonl) by
// CountSalvageApplied. It is deliberately not derivable from Recoverable:
// Recoverable is measured POTENTIAL — what a lenient reader could have
// recovered — while Saved is what the gate did, and a stage that refuses on
// ambiguity or fails the re-verify pass makes the two diverge. Reporting one
// as the other would tell operators the gate is salvaging reports it in fact
// blocked. Always emitted (never omitempty) so a consumer needs no special
// case for the fresh-project-root state.
type BaselineSummary struct {
	Total       int `json:"total"`
	Recoverable int `json:"recoverable"`
	Saved       int `json:"saved"`
	// Malformed is how many sidecar records were unreadable and therefore
	// skipped. Emitted in the envelope because the JSON consumer is exactly
	// the one that cannot see the prose WARN — tolerance that is honest only
	// in prose is silent where it is machine-read (diff-review MEDIUM).
	Malformed int                    `json:"malformed"`
	Rate      float64                `json:"rate"`
	ByPattern map[SalvagePattern]int `json:"by_pattern"`
}

// CountSalvageApplied folds the salvage-applied JSONL into the number of
// coercions the extraction stage actually performed — every run, not just this
// process's (run-scoping is SalvageSummaryLine's job, which answers the
// different question "what did THIS cycle salvage"). Pure: no filesystem
// access, no mutation of its input.
//
// Records of a foreign event_type are skipped: the sidecar is a repo-level file
// any emitter may append to, and counting a foreign line would inflate the one
// number an operator reads as "the gate coerced this many verdicts". Blank
// lines are not records.
//
// TRUST POSTURE (cycle-1442 audit M1/M2). This sidecar is append-only and
// UNAUTHENTICATED, so both of its failure directions are handled explicitly and
// neither is silent:
//
//   - Unreadable line. Returned as the second value (malformed), never fatal.
//     A torn line is an ordinary crash artifact — the in-process summary has
//     always tolerated one — and hard-erroring here discarded the entire
//     already-computed operator report (exit 1, no output) over one bad byte.
//     Reporting the skip count is what keeps tolerance honest: silently
//     dropping lines would make a deliberately torn record a way to HIDE
//     salvages.
//   - Inflated count. Foreign event types do not count, and the number this
//     returns is advisory telemetry, never a gate input — nothing in the
//     decision path reads it.
func CountSalvageApplied(r io.Reader) (saved int, malformed int, err error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec struct {
			EventType string `json:"event_type"`
		}
		if jsonErr := json.Unmarshal([]byte(line), &rec); jsonErr != nil {
			malformed++
			continue
		}
		if rec.EventType == salvageAppliedEventType {
			saved++
		}
	}
	// A scanner error is the one genuinely fatal case: it means the read
	// itself failed (or a single line exceeded the cap), so the remainder of
	// the file was never seen and any count would be an undercount presented
	// as a total.
	if scErr := sc.Err(); scErr != nil {
		return 0, malformed, fmt.Errorf("read %s: %w", salvageAppliedFile, scErr)
	}
	return saved, malformed, nil
}

// SalvageAppliedFile is the applied-coercions sidecar's basename under .evolve/.
// Exported for the same one-name-two-callers reason as BadVerdictBaselineFile:
// the CLI must open the file recordSalvageApplied appends to.
const SalvageAppliedFile = salvageAppliedFile

// SummarizeBadVerdictBaseline folds the bad-verdict baseline JSONL into a
// BaselineSummary. Pure: no filesystem access, no mutation of its input.
//
// A line that does not parse is a LOUD error, never a skipped line. The sidecar
// is append-per-emit, so a killed process can leave a torn record — and a
// summarizer that quietly dropped it would under-count the denominator and bias
// the measurement it exists to produce (rule 12). Blank lines are not records
// and are skipped.
func SummarizeBadVerdictBaseline(r io.Reader) (BaselineSummary, error) {
	sum := BaselineSummary{ByPattern: map[SalvagePattern]int{}}
	sc := bufio.NewScanner(r)
	// Records carry the classifier's Reason plus an artifact path; the default
	// 64KiB token cap is generous but not obviously enough forever.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec struct {
			EventType   string         `json:"event_type"`
			Recoverable bool           `json:"recoverable"`
			Pattern     SalvagePattern `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return BaselineSummary{}, fmt.Errorf(
				"%s line %d is not valid JSON: %w (a torn append is never skipped silently — dropping it would under-count the denominator and bias the rate)",
				BadVerdictBaselineFile, lineNo, err)
		}
		if rec.EventType != badVerdictEventType {
			continue
		}
		sum.Total++
		if rec.Recoverable {
			sum.Recoverable++
		}
		if rec.Pattern != SalvagePatternNone {
			sum.ByPattern[rec.Pattern]++
		}
	}
	if err := sc.Err(); err != nil {
		return BaselineSummary{}, fmt.Errorf("read %s: %w", BadVerdictBaselineFile, err)
	}
	if sum.Total > 0 {
		sum.Rate = float64(sum.Recoverable) / float64(sum.Total)
	}
	return sum, nil
}
