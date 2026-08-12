package deliverable

// salvage_extract.go — the SECOND deliverable of the schema-aligned salvage
// layer (docs/research/deliverable-alignment-2026-08/README.md, portfolio item
// `schema-aligned-salvage-layer`): EXTRACTION, gated on cycle-1389's measured
// 9% (15/167) recoverable-malformed baseline (bad-verdict-baseline.jsonl,
// salvage_instrument.go's own header). That measurement cleared the item's
// "instrumentation before extraction" precondition (README §6/§7); this file
// is the extraction/coercion stage the inbox item's `fix` text describes.
//
// Per README §3.3 (BAML SAP citation): lenient, LOGGED, bounded coercion of
// bytes already present in Result.Content — reformatting only, never
// inventing a field value — with a hard refusal on genuine ambiguity (the
// inbox item's own "fail only on genuine ambiguity" constraint). A REFUSED
// salvage never touches the Result; an APPROVED one returns the repaired bytes
// it verified, so the approval and the bytes it was granted over can never
// diverge (cycle-1441 audit H1).

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	evolvelog "github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// salvageAppliedFile is the JSONL sidecar a SUCCESSFUL salvage is logged to —
// distinct from (and in addition to) baselineFile, which records every
// bad_verdict classification unconditionally. salvage-applied.jsonl records
// only the subset salvage actually recovered, so "how many did we salvage"
// never needs to be re-derived from the baseline's superset.
const salvageAppliedFile = "salvage-applied.jsonl"

// salvageAppliedEventType is the `event_type` recordSalvageApplied stamps and
// CountSalvageApplied filters on — one constant, writer and reader, never two
// literals that can drift apart.
const salvageAppliedEventType = "salvage_applied"

// verdictKeyRE matches the `"verdict":` key itself. Unlike verdictObjRE it
// makes no attempt to delimit the object carrying the key — delimiting is the
// brace-aware scanner's job below.
//
// Case-INSENSITIVE, matching how encoding/json resolves the field on the
// re-verify pass: a count that is stricter than the decoder is a count that
// cannot see a decoy the decoder will act on (see verdictKeyCount).
var verdictKeyRE = regexp.MustCompile(`(?i)"verdict"\s*:`)

// verdictSpan is one verdict-bearing candidate located in the content:
// a top-level balanced `{...}` object whose text carries a "verdict" key.
// end is exclusive; end == -1 marks an UNTERMINATED object (the content ran
// out before the braces balanced), which is a candidate that exists but
// cannot be resolved into bytes.
type verdictSpan struct{ start, end int }

// verdictCandidates locates every verdict-bearing candidate ANYWHERE in the
// content, counting OBJECTS rather than the containers they sit in.
//
// The regex-only predecessor counted one candidate per FENCE and used
// verdictObjRE (`\{[^{}]*"verdict"\s*:[^{}]*\}`), whose character classes
// cannot cross a brace. Two candidates sharing one fence therefore counted as
// one, and a verdict object carrying a nested block — every ADR-0039 FAIL,
// which nests a structured `failure` object — was invisible entirely, so a
// stray PASS beside a substantive FAIL read as unambiguous and was laundered
// into an approval (cycle-1399 audit dfa28f113d8269306a5fe304b8091bbb2, HIGH).
// Counting balanced objects fixes both halves with one primitive: nesting is
// tracked by depth, and containers stop mattering.
//
// stringAware selects whether a brace inside a JSON string literal moves the
// depth. Both readings are run (see candidateCount): string-awareness is right
// for JSON, but a markdown deliverable is PROSE with JSON in it, and prose
// quoting cannot be trusted to be balanced.
//
// The scan fails CLOSED on truncation: an unterminated object carrying a
// verdict key is still returned as a candidate, so it makes the content
// ambiguous rather than disappearing from the count.
func verdictCandidates(content string, stringAware bool) []verdictSpan {
	var out []verdictSpan
	depth, start := 0, -1
	inStr, esc := false, false
	for i := 0; i < len(content); i++ {
		ch := content[i]
		if !stringAware {
			// Quotes are not structure in this reading — only braces are.
			switch ch {
			case '{':
				if depth == 0 {
					start = i
				}
				depth++
			case '}':
				if depth == 0 {
					continue
				}
				depth--
				if depth == 0 && start >= 0 {
					if verdictKeyRE.MatchString(content[start : i+1]) {
						out = append(out, verdictSpan{start: start, end: i + 1})
					}
					start = -1
				}
			}
			continue
		}
		if inStr {
			switch {
			case esc:
				esc = false
			case ch == '\\':
				esc = true
			case ch == '"':
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue // stray closer in prose
			}
			depth--
			if depth == 0 && start >= 0 {
				if verdictKeyRE.MatchString(content[start : i+1]) {
					out = append(out, verdictSpan{start: start, end: i + 1})
				}
				start = -1
			}
		}
	}
	if depth > 0 && start >= 0 && verdictKeyRE.MatchString(content[start:]) {
		out = append(out, verdictSpan{start: start, end: -1})
	}
	return out
}

// candidateCount reports how many independent verdict-bearing candidates the
// content carries. SalvageVerdict's ambiguity rule is "more than one candidate
// ANYWHERE in the content" (the inbox item's explicit hard constraint), so a
// sentinel payload coexisting with a stray bare object, two objects in one
// fence, and two objects in two fences are all equally ambiguous.
//
// It takes the LARGER of the two readings, because a counter that can only
// UNDERCOUNT implements "refuse when the attacker permits", not "refuse on
// ambiguity". The string-aware reading alone was exactly that: one unpaired `"`
// in prose left the scan believing every later byte was inside a string, so
// BOTH candidates of a two-candidate report vanished and `> 1` never fired —
// while ClassifyBadVerdict's step 2 computes parity FENCE-LOCALLY
// (verdictObjSpan runs over the fence body alone) and still qualified the
// stray fenced PASS, laundering a phase's own malformed FAIL into an approval
// (cycle-1424 audit d4982b388c4982275303ee68529b9313d, CRITICAL, reproduced
// through Reviewer.Review). The brace-only reading cannot be silenced by
// quoting; the string-aware reading still catches the mirror case where a
// string CONTAINS a stray brace and the brace-only reading would merge two
// objects into one. Neither reading dominates, so ambiguity is what EITHER can
// see — the same fail-closed asymmetry the classifier is built on:
// over-counting costs a salvage, under-counting costs the gate.
//
// Both of those readings are nevertheless STATEFUL scans, and cycle 1432's
// probe showed each has its own one-character silencer in ordinary prose: an
// unpaired `"` desynchronises the string-aware reading (aware=0), and a single
// unmatched `{` opens a depth the brace-only reading never closes, absorbing
// every later object into one truncation-fallback span (raw=1). With one of
// each in prose, `max(0, 1) == 1` is not `> 1`, the refusal never fires, and a
// fenced decoy PASS beside the phase's own displaced FAIL is laundered into an
// approval (cycle-1424 CRITICAL d4982b388c4982275303ee68529b9313d, still open
// through cycle 1428's quote-only fix; re-probed at the gate seam by
// go/acs/cycle1432/predicates_test.go TestC1432_001/002).
//
// verdictKeyCount is the third reading, and the answer to "a reading that
// cannot be desynchronised by an unbalanced delimiter": it carries NO scan
// state at all, so no earlier byte can change what a later one means. It
// counts the `"verdict":` keys themselves — one per verdict-bearing payload,
// whatever containers, nesting or stray delimiters surround them — which is
// the property the two structural scans trade away for span offsets. It is
// only ever consulted through the max, so like the brace-only reading it can
// add refusals and never remove one; a stray `"verdict":` mentioned in prose
// costs a salvage, which is the side of the asymmetry this gate is built on.
func candidateCount(content string) int {
	counts := []int{
		len(verdictCandidates(content, true)),
		len(verdictCandidates(content, false)),
		verdictKeyCount(content),
	}
	max := 0
	for _, n := range counts {
		if n > max {
			max = n
		}
	}
	return max
}

// verdictKeyCount is candidateCount's stateless reading: the number of
// `"verdict":` keys in the content. Deliberately not brace- or string-aware —
// that awareness is exactly the scan state an unbalanced delimiter hijacks.
//
// It counts the key the way the RE-VERIFY pass reads it, not the way the byte
// literal spells it. encoding/json matches struct fields case-INSENSITIVELY
// and decodes `\uXXXX` escapes, so `"Verdict":` and `"verdict":` are
// verdict keys to the decoder while a byte-literal count sees none of them —
// a decoy the ambiguity guard cannot see but the decoder can act on
// (cycle-1432 audit d4fa6591dcd07c365884c64925a8e3dbe, CRITICAL C1). Counting
// is only ever consulted through candidateCount's max, so widening it can add
// refusals and never remove one: an obfuscated key in prose costs a salvage,
// which is the side of the asymmetry this gate is built on.
func verdictKeyCount(content string) int {
	return len(verdictKeyRE.FindAllStringIndex(unescapeJSONShort(content), -1))
}

// unescapeJSONShort rewrites `\uXXXX` escapes of ASCII characters back to the
// bytes encoding/json will decode them to, so a key spelled `"verdict"`
// counts as the key it becomes. Non-ASCII escapes are left alone: they cannot
// spell any character of `verdict`, and rewriting them would change offsets
// this counting reading deliberately does not own. Length-preserving is not
// required — verdictKeyCount counts, it never returns spans.
func unescapeJSONShort(s string) string {
	if !strings.Contains(s, `\u`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if i+6 <= len(s) && s[i] == '\\' && s[i+1] == 'u' {
			if n, err := strconv.ParseUint(s[i+2:i+6], 16, 32); err == nil && n < 0x80 {
				b.WriteByte(byte(n))
				i += 6
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// firstVerdictObject is deliberately GONE: it was the repairer's own extraction
// primitive, and a repairer that extracts is a repairer that can disagree with
// the classifier. Candidate COUNTING (verdictCandidates, above) stays — that is
// the ambiguity guard's job and is a different question from which bytes to lift.

// SalvageVerdict is the extraction/coercion stage: for a Result whose SOLE
// reported problem is a bad_verdict AND whose content classifies as
// recoverable (ClassifyBadVerdict) AND carries exactly one verdict-bearing
// candidate anywhere in the content AND whose REPAIRED bytes re-verify clean,
// it returns a Result with OK=true and no Violations, applied=true. On any
// other outcome it refuses: returns res UNCHANGED, applied=false.
//
// SalvageVerdict never invents a field value: every field on a refused/absent
// Result is byte-identical to the input, and on a successful salvage only OK,
// Violations and Content change — Content becoming the repaired bytes the
// re-verify pass below approved, never a re-authored payload.
//
// Persisting those bytes back to ArtifactPath is the CALLER's decision, and the
// production gate (Reviewer.Review) makes it: see persistSalvagedArtifact in
// reviewer.go. This function stays free of I/O so the extraction contract is
// testable as a pure byte transform.
//
// It resolves the phase contract through the built-in registry at StageOff;
// the production gate calls salvageVerdictWith with its OWN resolver and
// PhaseIO stage, so a catalog/minted phase re-verifies under the same
// contract that rejected it.
func SalvageVerdict(res Result) (Result, bool) {
	return salvageVerdictWith(res, phasecontract.BuiltinResolver{}, phasecontract.Roots{}, config.StageOff)
}

// salvageVerdictWith is SalvageVerdict with the resolver, roots and PhaseIO
// stage threaded from the caller — the seam the Reviewer uses so the re-verify
// pass (below) runs the SAME contract resolution and the SAME roots the
// original Verify ran.
func salvageVerdictWith(res Result, resolver phasecontract.Resolver, roots phasecontract.Roots, phaseIO config.Stage) (Result, bool) {
	// SOLE violation, never membership. hasCode ("is a bad_verdict in there
	// anywhere") let a bad_verdict co-occurring with missing_section or
	// missing_challenge_token be salvaged wholesale — OK forced true and ALL
	// violations erased, including the cycle-269 anti-forgery proof-of-read
	// check. That is a report-forgery bypass on the gate's own decision seam
	// (cycle-1392 audit CRITICAL-1, probe-confirmed). Salvage repairs the
	// VERDICT and nothing else, so it may only ever act when the verdict is
	// the one and only thing wrong. Negative predicate:
	// go/acs/cycle1397/predicates_test.go TestC1397_001_SalvageVerdict_MultiViolationNeverSalvaged.
	if !res.onlyViolation(CodeBadVerdict) {
		return res, false
	}
	cls := ClassifyBadVerdict(res.Content)
	if !cls.Recoverable {
		return res, false
	}
	if candidateCount(res.Content) > 1 {
		// Genuine ambiguity: refuse rather than silently pick one candidate —
		// the item's explicit hard constraint (scout Hypothesis 2).
		return res, false
	}
	repaired, ok := repairVerdict(res.Content, cls)
	if !ok {
		return res, false
	}
	c, ok := resolver.Resolve(res.Phase)
	if !ok {
		// Cannot re-verify what we cannot resolve. Salvage fails CLOSED (the
		// gate's own fail-OPEN posture already ran upstream in Verify).
		return res, false
	}
	// Re-verify the REPAIRED bytes instead of hand-setting OK=true. Flipping
	// OK from the classification alone skipped every content check the strict
	// parse never reached — most sharply RequireFailureContext, which turned a
	// malformed FAIL sentinel with no ADR-0039 failure block into an approval
	// (cycle-1392 audit MEDIUM-3). The caller's OWN roots are used, not an
	// empty set: with empty roots the roots-dependent checks (challenge-token
	// echo, stray-in-worktree) resolve their paths against the process CWD, so
	// a challenge-token.txt planted in whatever directory the loop happens to
	// run from could fail the re-verify and deny an otherwise valid salvage
	// (cycle-1399 audit d8b22040, LOW — fail-closed, but attacker-influenced).
	// Re-verifying under the same roots the original Verify used removes the
	// CWD dependency entirely, and cannot newly block anything: those checks
	// already ran over these same bytes upstream, and any violation they raised
	// makes the result non-sole — refused above, before repair.
	var check Result
	verifyMarkdown(&check, c, repaired, roots, phaseIO)
	check.finish()
	if !check.OK {
		return res, false
	}
	salvaged := res
	salvaged.OK = true
	salvaged.Violations = nil
	// Carry the bytes the re-verify above actually approved. Returning the
	// ORIGINAL Content with only OK/Violations flipped made the gate report
	// OK=true over a byte stream that was never the byte stream it verified:
	// every consumer of Result.Content — and, via the Reviewer's write-back,
	// every downstream phase re-reading ArtifactPath — still saw the malformed
	// original, so a FAIL sentinel the strict parse could not read could be
	// re-resolved to PASS by a prose scan further down the pipeline
	// (cycle-1441 audit H1, HIGH). Approved bytes and returned bytes are now
	// the same bytes, by construction.
	salvaged.Content = repaired
	return salvaged, true
}

// repairVerdict rebuilds the deliverable's bytes with the single recovered
// verdict payload re-emitted, in place, as a canonical evolve-verdict sentinel
// comment. It is REFORMATTING only (README §3.3): the payload's own bytes are
// carried across verbatim except for the trailing commas JSON forbids, and no
// field value is invented. The result is a CANDIDATE for re-verification: it
// becomes Result.Content only after that re-verify passes, never before.
//
// It repairs the span cls QUALIFIED and nothing else. It deliberately performs
// no search of its own: its predecessor re-derived a span per pattern — the
// trailing-comma branch took `sentinelPayloadRE.FindStringSubmatch(content)`,
// the FIRST match, with no string-literal check the classifier had already
// applied — so a report quoting a decoy sentinel got CLASSIFIED on its genuine
// malformed FAIL and REPAIRED on the decoy PASS, and the gate approved
// (cycle-1406 audit CRITICAL-1). Adding a second quoting check here would have
// closed that instance and left two independently-drifting notions of an
// admissible span; taking the offsets instead means "repair a span the
// classifier rejected" has no expressible form.
//
// Returns ok=false when the qualified payload cannot be recovered into valid
// JSON (or the span does not address these bytes), which is itself a refusal:
// an unrepairable candidate never reaches the re-verify pass.
func repairVerdict(content string, cls BadVerdictClassification) (string, bool) {
	if !cls.Recoverable || !cls.span.addresses(len(content)) || !cls.payload.addresses(len(content)) {
		return "", false
	}
	payload := content[cls.payload.start:cls.payload.end]
	if cls.Pattern == SalvagePatternTrailingComma {
		// Drop the comma, keep the closing brace/bracket the match consumed.
		payload = trailingCommaRE.ReplaceAllStringFunc(payload, func(s string) string { return s[1:] })
	}
	if !json.Valid([]byte(payload)) {
		return "", false
	}
	return content[:cls.span.start] + sentinelLine(payload) + content[cls.span.end:], true
}

// addresses reports whether the span is a resolvable half-open range over n
// bytes. It rejects the end == -1 unterminated marker verdictCandidates uses,
// so an unresolvable candidate can never be sliced.
func (v verdictSpan) addresses(n int) bool {
	return v.start >= 0 && v.end >= v.start && v.end <= n
}

// sentinelLine wraps an already-valid JSON payload in the sentinel markers
// phasecontract.ParseVerdictSentinelFull reads. Deliberately NOT
// phasecontract.RenderVerdictSentinel: that renderer marshals a fresh
// VerdictSentinel from (phase, verdict) and would DROP every other key the
// agent actually wrote. Salvage relocates the agent's bytes; it does not
// re-author them.
func sentinelLine(payload string) string { return "<!-- evolve-verdict: " + payload + " -->" }

// recordSalvageApplied appends one record to salvage-applied.jsonl when (and
// only when) salvage actually recovered a verdict. Mirrors
// recordBadVerdictBaseline's fail-safe posture: a write failure is logged and
// swallowed, never allowed to influence the gate's decision.
func recordSalvageApplied(roots phasecontract.Roots, phase string, pattern SalvagePattern, logf func(string, ...any)) {
	if roots.EvolveDir == "" {
		return
	}
	w := evolvelog.NewSidecarWriter(filepath.Join(roots.EvolveDir, salvageAppliedFile))
	err := w.EmitAbnormal(evolvelog.Event{
		EventType: salvageAppliedEventType,
		Severity:  "info",
		Fields: map[string]any{
			"phase":   phase,
			"pattern": string(pattern),
			"run":     salvageRunID,
		},
	})
	if err != nil && logf != nil {
		logf("[contract-gate] WARN could not append salvage-applied record: %v", err)
	}
}

// SalvageSummaryLine renders a one-line "Salvaged verdicts: N (pattern
// breakdown)" summary from the salvage-applied.jsonl sidecar under evolveDir —
// the SAME sidecar recordSalvageApplied writes, so the count is always
// single-sourced from the real production wiring, never a second counter.
// Returns "" when the sidecar is absent or carries zero records — the inbox
// item's explicit "omit the section entirely at 0" (no zero-noise).
func SalvageSummaryLine(evolveDir string) string {
	// Streamed with the same per-line cap the sibling readers use
	// (salvage_report.go), not a whole-file ReadFile: this runs on the gate's
	// hot path for EVERY salvage, over a never-rotated append-only sidecar, so
	// a slurp made each salvage pay for the entire history of prior ones
	// (go-reviewer MEDIUM).
	f, err := os.Open(filepath.Join(evolveDir, salvageAppliedFile))
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	type appliedRec struct{ pattern, run string }
	recs := make([]appliedRec, 0, 8)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m struct {
			Pattern string `json:"pattern"`
			Run     string `json:"run"`
		}
		if json.Unmarshal([]byte(line), &m) != nil {
			// A record a killed writer truncated mid-line. Skipped by the
			// breakdown, and — since the headline is derived from this same
			// slice — skipped by the total too. The predecessor computed
			// `total := len(lines)` over the raw split while the breakdown
			// skipped blanks and truncations, so the two numbers came from
			// different populations and the coercion count operators read
			// overstated itself (cycle-1399 audit dae44191, MEDIUM).
			continue
		}
		recs = append(recs, appliedRec{pattern: m.Pattern, run: m.Run})
	}
	// A scanner error means the tail was never seen; reporting a partial count
	// as a total is the one dishonest option, so the line is omitted instead.
	if sc.Err() != nil {
		return ""
	}
	// Scope to THIS run. The sidecar is a repo-level, never-rotated file, so
	// its all-time total was being logged by reviewer.go as the current phase's
	// figure — a real number with a false frame, which an operator reads as
	// "this cycle coerced N verdicts" (cycle-1399 audit d8b22040, LOW). Records
	// this run appended carry its id; when NONE do (a legacy sidecar written
	// before the tag existed) the whole file is reported rather than nothing,
	// so an older checkout degrades to the previous behaviour instead of going
	// silent.
	scoped := make([]appliedRec, 0, len(recs))
	for _, r := range recs {
		if r.run == salvageRunID {
			scoped = append(scoped, r)
		}
	}
	if len(scoped) > 0 {
		recs = scoped
	}
	if len(recs) == 0 {
		return ""
	}
	counts := map[string]int{}
	order := make([]string, 0, len(recs))
	for _, r := range recs {
		p := operatorPatternLabel(r.pattern)
		if _, seen := counts[p]; !seen {
			order = append(order, p)
		}
		counts[p]++
	}
	parts := make([]string, 0, len(order))
	for _, p := range order {
		parts = append(parts, fmt.Sprintf("%s=%d", p, counts[p]))
	}
	return fmt.Sprintf("Salvaged verdicts: %d (%s)", len(recs), strings.Join(parts, ", "))
}

// salvageRunID identifies the process that appended a salvage-applied record,
// so SalvageSummaryLine can report this run's salvages rather than the
// un-rotated sidecar's all-time total. A pid is enough: the renderer only ever
// runs in the same process that just appended.
var salvageRunID = strconv.Itoa(os.Getpid())

// unknownPatternBucket is the single label every un-recognised `pattern` value
// collapses into. Bounded cardinality is the point: an untrusted appender can
// make the operator line say "some record carried a pattern we do not mint",
// and nothing else.
const unknownPatternBucket = "unknown"

// operatorPatternLabel decides what an untrusted sidecar `pattern` value may
// call itself in the single-line `[contract-gate]` operator log.
//
// Its predecessor (sanitizeLogField) checked the value against a CHARACTER
// CLASS `^[A-Za-z0-9_.-]+$` and Go-quoted anything else. That closed the
// control-character half of cycle-1399 audit dae44191 (a newline forged a
// second operator line) but not the other half: `salvage-applied.jsonl` is a
// repo-level file any phase can append to, and a forged value that is merely
// alphanumeric — `circuit-open-notice` — passed the class untouched and
// rendered as a breakdown entry indistinguishable from a real one, fabricating
// gate state in the one stream an operator judges the gate's health from
// (carryover todo-salvage-summary-line-rejects-untrusted-sidecar-text, HIGH).
//
// Membership in the closed SalvagePattern set is the check the shape actually
// warrants: the renderer mints these strings itself (recordSalvageApplied), so
// anything else is by definition not ours. Known constants render bare and
// legible; everything else folds into one `unknown` bucket — which subsumes the
// character-class guard (no control character can survive a closed allowlist)
// while still COUNTING the record, so a bogus pattern shows up as evidence
// rather than vanishing.
func operatorPatternLabel(pattern string) string {
	switch SalvagePattern(pattern) {
	case SalvagePatternFencedJSON, SalvagePatternTrailingComma, SalvagePatternDisplaced:
		return pattern
	}
	return unknownPatternBucket
}
