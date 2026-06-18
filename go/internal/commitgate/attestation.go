package commitgate

import "strings"

// Attestation is .commit-gate/attestation.json — the tree-SHA-bound proof that
// review + lint + targeted tests passed before this exact tree is committed.
//
// The ship-gate reader (go/internal/phases/ship/commitgate.go) only consumes
// TreeStateSHA and ReviewersRun, but Marshal emits ALL five fields in the bash
// runner's byte-exact layout (field order, 2-space indent, inline arrays,
// trailing newline) so the differential parity test can compare the two writers
// byte-for-byte before B2 deletes the bash runner.
type Attestation struct {
	// TreeStateSHA is sha256(`git diff HEAD`) — the binding the reader verifies
	// against the staged tree.
	TreeStateSHA string `json:"tree_state_sha"`
	// TS is the UTC write time (RFC3339 "...Z"), informational only.
	TS string `json:"ts"`
	// ChecksPassed lists the lint/test checks that passed, in execution order.
	ChecksPassed []string `json:"checks_passed"`
	// ReviewersRun is the raw --reviewers list (empties dropped). The reader
	// turns it into Reviewed-by git trailers.
	ReviewersRun []string `json:"reviewers_run"`
	// Tool is the sha256 binary that produced the SHA ("shasum" | "sha256sum").
	Tool string `json:"tool"`
}

// Marshal renders the attestation in the bash runner's exact byte layout:
//
//	{
//	  "tree_state_sha": "<sha>",
//	  "ts": "<ts>",
//	  "checks_passed": [<inline,csv>],
//	  "reviewers_run": [<inline,csv>],
//	  "tool": "<tool>"
//	}
//
// with a trailing newline (the heredoc adds one after the closing brace). The
// arrays are inline with no inter-element spacing — `["a","b"]` — exactly as the
// bash awk builders produce. Values are NOT JSON-escaped: the bash runner does
// not escape them either, and the inputs (a hex SHA, an RFC3339 timestamp,
// check tokens, and reviewer names already stripped of whitespace/newlines) are
// all JSON-safe, so the two writers stay byte-identical.
func (a *Attestation) Marshal() []byte {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "tree_state_sha": "` + a.TreeStateSHA + "\",\n")
	b.WriteString(`  "ts": "` + a.TS + "\",\n")
	b.WriteString(`  "checks_passed": [` + jsonStringArray(a.ChecksPassed) + "],\n")
	b.WriteString(`  "reviewers_run": [` + jsonStringArray(a.ReviewersRun) + "],\n")
	b.WriteString(`  "tool": "` + a.Tool + "\"\n")
	b.WriteString("}\n")
	return []byte(b.String())
}

// jsonStringArray joins items as a quoted, comma-separated, space-free list —
// `"a","b","c"` (empty for no items) — the inner body of the inline JSON arrays.
func jsonStringArray(items []string) string {
	if len(items) == 0 {
		return ""
	}
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = `"` + it + `"`
	}
	return strings.Join(quoted, ",")
}

// splitReviewers turns the raw --reviewers CSV into reviewers_run: split on
// comma, drop empties, keep the original spelling (namespace prefixes intact).
// This mirrors the bash awk builder, which records the raw $REVIEWERS verbatim —
// only the PRECONDITION check (reviewersSatisfied) normalizes prefixes.
func splitReviewers(csv string) []string {
	var out []string
	for _, r := range strings.Split(csv, ",") {
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}
