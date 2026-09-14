package ledger

// verify_scope_test.go — cycle-1677: a successful verification must state
// WHICH history it accepted. `OK: chain intact` was one string for two very
// different claims — every byte from genesis, or a strict walk that resumed at
// an operator-adjudicated epoch anchor — and that ambiguity is what let the
// ledger-1740 damage stay invisible. These tests pin the scope VerifyScope /
// VerifyDeepScope report, including the part a sidecar file cannot answer.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// sealedFixture builds the shape the epoch-anchor remedy exists for: a valid
// genesis + one line, then PRESERVED historical damage, then a hash-valid
// operator seal (entry_seq 4), then a valid tail. Returns the lines and the
// seal's own SHA.
func sealedFixture() (lines []string, sealSHA string) {
	base, _ := chainLines()
	g, a := base[0], base[1]
	damaged := fmt.Sprintf(`{"ts":"2026-05-01T00:02:00Z","cycle":1,"role":"builder","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"%s"}`, sha256Of("rewritten-by-a-predecessor"))
	seal := operatorSealLine(sha256Of(damaged))
	tail := fmt.Sprintf(`{"ts":"2026-05-01T00:05:00Z","cycle":1,"role":"auditor","kind":"phase","exit_code":0,"entry_seq":5,"prev_hash":"%s"}`, sha256Of(seal))
	return []string{g, a, damaged, seal, tail}, sha256Of(seal)
}

// writeLedgerDir materialises lines as a ledger directory with a matching tip.
func writeLedgerDir(t *testing.T, lines []string, lastSeq int) string {
	t.Helper()
	dir := t.TempDir()
	var raw string
	for _, l := range lines {
		raw += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write ledger.jsonl: %v", err)
	}
	tip := fmt.Sprintf("%d:%s", lastSeq, sha256Of(lines[len(lines)-1]))
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatalf("write ledger.tip: %v", err)
	}
	return dir
}

// writeSidecarAnchor records an out-of-band ledger-anchor.json bound to lineSHA.
func writeSidecarAnchor(t *testing.T, dir string, seq int, lineSHA string) {
	t.Helper()
	body := fmt.Sprintf(`{"anchor_seq":%d,"anchor_line_sha256":%q,"recorded_at":"2026-05-01T00:00:00Z","note":"test"}`, seq, lineSHA)
	if err := os.WriteFile(filepath.Join(dir, "ledger-anchor.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write ledger-anchor.json: %v", err)
	}
}

// TestVerifyScope_FullStrictChainReportsNoAnchor: a chain with no anchor
// anywhere validated every byte, so it must claim no sealed prefix. The zero
// VerifiedScope is how "I checked everything" is said.
func TestVerifyScope_FullStrictChainReportsNoAnchor(t *testing.T) {
	lines, _ := chainLines()
	dir := writeLedgerDir(t, lines, 3)

	scope, err := New(dir).VerifyScope(context.Background())
	if err != nil {
		t.Fatalf("VerifyScope: %v", err)
	}
	if scope.AnchorLineSHA != "" {
		t.Errorf("a fully strict chain must report no epoch anchor; got sha %q seq %d", scope.AnchorLineSHA, scope.AnchorSeq)
	}
}

// TestVerifyScope_ReportsTheSealItResumedFrom: the walk resumed at the in-band
// operator seal, so that is the line the scope must name — by the seal's own
// SHA and its own entry_seq, both read out of the ledger.
func TestVerifyScope_ReportsTheSealItResumedFrom(t *testing.T) {
	lines, sealSHA := sealedFixture()
	dir := writeLedgerDir(t, lines, 5)

	scope, err := New(dir).VerifyScope(context.Background())
	if err != nil {
		t.Fatalf("VerifyScope over a sealed prefix: %v", err)
	}
	if scope.AnchorLineSHA != sealSHA {
		t.Errorf("scope names line %q, want the seal %q", scope.AnchorLineSHA, sealSHA)
	}
	if scope.AnchorSeq != 4 {
		t.Errorf("scope reports entry_seq=%d, want the seal's own 4", scope.AnchorSeq)
	}
}

// TestVerifyScope_SeqIsReadFromTheAnchorLineNotTheSidecar is the regression
// this cycle's provenance turns on, and it is not hypothetical: on the live
// 141k-line ledger the sidecar records anchor_seq=113890 while an in-band seal
// has since moved the effective anchor to entry_seq=136212. Reporting the
// sidecar's number would misname the trusted prefix by ~22k lines — in the
// reassuring direction — so the seq must come from the resolved LINE.
func TestVerifyScope_SeqIsReadFromTheAnchorLineNotTheSidecar(t *testing.T) {
	lines, sealSHA := sealedFixture()
	dir := writeLedgerDir(t, lines, 5)
	// The sidecar points at line 1 (entry_seq 1); the seal sits past it.
	writeSidecarAnchor(t, dir, 1, sha256Of(lines[1]))

	scope, err := New(dir).VerifyScope(context.Background())
	if err != nil {
		t.Fatalf("VerifyScope: %v", err)
	}
	if scope.AnchorLineSHA != sealSHA || scope.AnchorSeq != 4 {
		t.Errorf("the LATER in-band seal must win: got sha %q seq %d, want %q / 4", scope.AnchorLineSHA, scope.AnchorSeq, sealSHA)
	}
	if scope.AnchorSeq == 1 {
		t.Error("scope reported the sidecar's stale anchor_seq instead of the line the walk actually resumed from")
	}
}

// TestVerifyScope_SidecarOnlyAnchorReportsThatLine: with no in-band seal the
// sidecar line IS the anchor, and its seq still comes from the line's own
// entry_seq rather than the JSON field beside it.
func TestVerifyScope_SidecarOnlyAnchorReportsThatLine(t *testing.T) {
	lines, _ := chainLines()
	dir := writeLedgerDir(t, lines, 3)
	// Deliberately disagree with the line: the JSON says 99, the line says 1.
	writeSidecarAnchor(t, dir, 99, sha256Of(lines[1]))

	scope, err := New(dir).VerifyScope(context.Background())
	if err != nil {
		t.Fatalf("VerifyScope: %v", err)
	}
	if scope.AnchorLineSHA != sha256Of(lines[1]) {
		t.Errorf("scope names %q, want the sidecar's bound line %q", scope.AnchorLineSHA, sha256Of(lines[1]))
	}
	if scope.AnchorSeq != 1 {
		t.Errorf("scope reports entry_seq=%d, want the anchored LINE's 1 (not the sidecar's 99)", scope.AnchorSeq)
	}
}

// TestVerifyDeepScope_ReportsTheSameScopeAsVerifyScope: an operator's two
// verification commands must not disagree about what was verified (#373, the
// wired-into-one-path-only defect).
func TestVerifyDeepScope_ReportsTheSameScopeAsVerifyScope(t *testing.T) {
	lines, sealSHA := sealedFixture()
	dir := writeLedgerDir(t, lines, 5)
	l := New(dir)

	shallow, err := l.VerifyScope(context.Background())
	if err != nil {
		t.Fatalf("VerifyScope: %v", err)
	}
	deep, err := l.VerifyDeepScope(context.Background())
	if err != nil {
		t.Fatalf("VerifyDeepScope: %v", err)
	}
	if deep != shallow {
		t.Errorf("--deep reports %+v but the default path reports %+v", deep, shallow)
	}
	if deep.AnchorLineSHA != sealSHA {
		t.Errorf("VerifyDeepScope names line %q, want the seal %q", deep.AnchorLineSHA, sealSHA)
	}
}

// TestVerifyScope_BrokenChainClaimsNoScope: a break one line PAST the seal is
// still a break — a seal covers the prefix behind it, never the tail ahead of
// it — and a rejected chain must not hand back a sealed-prefix claim that a
// caller could print as reassurance.
func TestVerifyScope_BrokenChainClaimsNoScope(t *testing.T) {
	lines, _ := sealedFixture()
	lines[4] = fmt.Sprintf(`{"ts":"2026-05-01T00:05:00Z","cycle":1,"role":"auditor","kind":"phase","exit_code":0,"entry_seq":5,"prev_hash":"%s"}`, sha256Of("forged-tail"))
	dir := writeLedgerDir(t, lines, 5)
	l := New(dir)

	scope, err := l.VerifyScope(context.Background())
	if err == nil {
		t.Fatalf("a break after the last eligible seal must NOT verify; got scope %+v", scope)
	}
	if scope != (VerifiedScope{}) {
		t.Errorf("a failed verification must claim no scope; got %+v", scope)
	}
	if deep, derr := l.VerifyDeepScope(context.Background()); derr == nil {
		t.Errorf("--deep must reject the same forged tail; got scope %+v", deep)
	}
}
