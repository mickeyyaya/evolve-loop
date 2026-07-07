package phasecontract_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// TestParseVerdictSentinelFull_RejectsPlaceholderEcho_MixedWithRealDefect
// probes the boundary between two plausible readings of the guard: "reject
// only when EVERY defect is a placeholder" vs "reject when ANY defect is a
// placeholder". A contract-example echo can only ever appear alongside real
// content if the capture is noisy (never as the agent's deliberate choice),
// so even one wholly-bracketed entry must be disqualifying — mixing in a
// genuine-looking defect must not launder it back to ok=true.
func TestParseVerdictSentinelFull_RejectsPlaceholderEcho_MixedWithRealDefect(t *testing.T) {
	raw := phasecontract.RenderVerdictSentinelWithFailure("build", "FAIL", &phasecontract.FailureBlock{
		Class:   "build_failure",
		Defects: []string{"build failed: import cycle in pkg/foo", "<one line per defect>"},
	})
	if _, ok := phasecontract.ParseVerdictSentinelFull(raw); ok {
		t.Errorf("ok=true for a Defects list containing one genuine entry and one wholly-bracketed placeholder; want ok=false — a single placeholder entry must not be launderable by pairing it with real-looking text")
	}
}

// TestParseVerdictSentinelFull_RejectsPlaceholderEcho_WhitespacePadded checks
// that surrounding whitespace around the bracketed token (plausible if a
// template line is echoed with leading indentation or a trailing space)
// still counts as "wholly" a placeholder.
func TestParseVerdictSentinelFull_RejectsPlaceholderEcho_WhitespacePadded(t *testing.T) {
	raw := phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL", &phasecontract.FailureBlock{
		Class:   "review_failure",
		Defects: []string{"  <one line per defect>\t"},
	})
	if _, ok := phasecontract.ParseVerdictSentinelFull(raw); ok {
		t.Errorf("ok=true for a whitespace-padded placeholder defect; want ok=false — surrounding whitespace must not exempt an otherwise wholly-bracketed placeholder token")
	}
}

// TestParseVerdictSentinelFull_AllowsGenuineContentContainingAngleBrackets is
// the false-positive guard's mirror image: a real defect can legitimately
// mention angle brackets (generics, HTML tags, redirects) without being a
// placeholder echo, because the bracketed span is not the WHOLE string. A
// broad "contains '<'" implementation would wrongly reject this.
func TestParseVerdictSentinelFull_AllowsGenuineContentContainingAngleBrackets(t *testing.T) {
	raw := phasecontract.RenderVerdictSentinelWithFailure("build", "FAIL", &phasecontract.FailureBlock{
		Class:   "build_failure",
		Defects: []string{"type mismatch: expected List<string>, got List<int>"},
	})
	sentinel, ok := phasecontract.ParseVerdictSentinelFull(raw)
	if !ok {
		t.Fatalf("ok=false for a genuine defect that merely mentions angle brackets mid-sentence; want ok=true")
	}
	if sentinel.Failure == nil || len(sentinel.Failure.Defects) != 1 || sentinel.Failure.Defects[0] != "type mismatch: expected List<string>, got List<int>" {
		t.Errorf("Failure block did not round-trip the genuine defect intact: %+v", sentinel.Failure)
	}
}

// TestParseVerdictSentinelFull_RejectsGenericPlaceholderShape_NotJustKnownStrings
// confirms the guard matches the wholly-bracketed SHAPE, not a hardcoded list
// of the two known template strings ("<one line per defect>",
// "<artifact path>"). A future template rewording ("<TODO: ...>") must still
// be caught.
func TestParseVerdictSentinelFull_RejectsGenericPlaceholderShape_NotJustKnownStrings(t *testing.T) {
	raw := phasecontract.RenderVerdictSentinelWithFailure("scout", "FAIL", &phasecontract.FailureBlock{
		Class:         "scout_failure",
		EvidencePaths: []string{"<TODO: fill in the evidence path>"},
	})
	if _, ok := phasecontract.ParseVerdictSentinelFull(raw); ok {
		t.Errorf("ok=true for a differently-worded but still wholly-bracketed placeholder; want ok=false — the guard must key off shape (^<...>$), not a hardcoded string list")
	}
}

// TestParseVerdictSentinelFull_MixedAcrossBothFields exercises Defects and
// EvidencePaths simultaneously, each holding one genuine and one placeholder
// entry — a combinatorial case neither original RED test (which varies only
// one field at a time) covers.
func TestParseVerdictSentinelFull_MixedAcrossBothFields(t *testing.T) {
	raw := phasecontract.RenderVerdictSentinelWithFailure("tdd", "FAIL", &phasecontract.FailureBlock{
		Class:         "tdd_failure",
		Defects:       []string{"assertion mismatch in TestFoo", "<one line per defect>"},
		EvidencePaths: []string{"internal/foo/foo_test.go:42", "<artifact path>"},
	})
	if _, ok := phasecontract.ParseVerdictSentinelFull(raw); ok {
		t.Errorf("ok=true when both Defects and EvidencePaths each mix one real entry with one placeholder; want ok=false")
	}
}

// TestParseVerdictSentinelFull_UnclosedBracket_NotTreatedAsPlaceholder covers
// a malformed-but-real string that starts with '<' but never closes — the
// guard must require a matching closing '>' anchored at end-of-string, not
// merely a leading '<'.
func TestParseVerdictSentinelFull_UnclosedBracket_NotTreatedAsPlaceholder(t *testing.T) {
	raw := phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL", &phasecontract.FailureBlock{
		Class:   "review_failure",
		Defects: []string{"<one line per defect"},
	})
	sentinel, ok := phasecontract.ParseVerdictSentinelFull(raw)
	if !ok {
		t.Fatalf("ok=false for a defect with an unclosed '<' bracket; want ok=true — the placeholder guard must require a closing '>' to avoid over-rejecting malformed-but-real text")
	}
	if sentinel.Failure == nil || len(sentinel.Failure.Defects) != 1 || sentinel.Failure.Defects[0] != "<one line per defect" {
		t.Errorf("Failure block did not round-trip the unclosed-bracket defect intact: %+v", sentinel.Failure)
	}
}

// TestParseVerdictSentinelFull_RoundTripsRealFailureBlockThroughRenderAndParse
// is a producer→consumer lockstep check using only the package's public
// render/parse pair: a fully genuine failure block (multiple real defects and
// evidence paths, non-trivial Class) must survive the render→parse round
// trip byte-for-byte in every field, confirming the placeholder guard adds
// no collateral damage to ordinary failure reporting.
func TestParseVerdictSentinelFull_RoundTripsRealFailureBlockThroughRenderAndParse(t *testing.T) {
	want := &phasecontract.FailureBlock{
		Class:         "eval_failure",
		Defects:       []string{"golden file mismatch in TestBar", "off-by-one in pagination cursor"},
		EvidencePaths: []string{"internal/pagination/cursor_test.go:88", "testdata/golden/bar.json"},
	}
	raw := phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL", want)

	got, ok := phasecontract.ParseVerdictSentinelFull(raw)
	if !ok {
		t.Fatalf("ok=false for a fully genuine, multi-entry failure block; want ok=true")
	}
	if got.Phase != "audit" || got.Verdict != "FAIL" {
		t.Errorf("Phase/Verdict = %q/%q, want audit/FAIL", got.Phase, got.Verdict)
	}
	if got.SchemaVersion != phasecontract.SentinelSchemaVersionFailure {
		t.Errorf("SchemaVersion = %d, want %d (a failure block is present)", got.SchemaVersion, phasecontract.SentinelSchemaVersionFailure)
	}
	if got.Failure == nil {
		t.Fatalf("Failure block is nil after round-trip")
	}
	if got.Failure.Class != want.Class {
		t.Errorf("Failure.Class = %q, want %q", got.Failure.Class, want.Class)
	}
	if len(got.Failure.Defects) != len(want.Defects) {
		t.Fatalf("Failure.Defects len = %d, want %d: %v", len(got.Failure.Defects), len(want.Defects), got.Failure.Defects)
	}
	for i, d := range want.Defects {
		if got.Failure.Defects[i] != d {
			t.Errorf("Defects[%d] = %q, want %q", i, got.Failure.Defects[i], d)
		}
	}
	if len(got.Failure.EvidencePaths) != len(want.EvidencePaths) {
		t.Fatalf("Failure.EvidencePaths len = %d, want %d: %v", len(got.Failure.EvidencePaths), len(want.EvidencePaths), got.Failure.EvidencePaths)
	}
	for i, p := range want.EvidencePaths {
		if got.Failure.EvidencePaths[i] != p {
			t.Errorf("EvidencePaths[%d] = %q, want %q", i, got.Failure.EvidencePaths[i], p)
		}
	}
}
