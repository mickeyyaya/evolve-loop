package ledger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// sealedFixture: genesis, one line, preserved damage, a hash-valid operator seal (entry_seq 4), a valid tail.
func sealedFixture() (lines []string, sealSHA string) {
	base, _ := chainLines()
	g, a := base[0], base[1]
	damaged := fmt.Sprintf(`{"ts":"2026-05-01T00:02:00Z","cycle":1,"role":"builder","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"%s"}`, sha256Of("rewritten-by-a-predecessor"))
	seal := operatorSealLine(sha256Of(damaged))
	tail := fmt.Sprintf(`{"ts":"2026-05-01T00:05:00Z","cycle":1,"role":"auditor","kind":"phase","exit_code":0,"entry_seq":5,"prev_hash":"%s"}`, sha256Of(seal))
	return []string{g, a, damaged, seal, tail}, sha256Of(seal)
}

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

func writeSidecarAnchor(t *testing.T, dir string, seq int, lineSHA string) {
	t.Helper()
	body := fmt.Sprintf(`{"anchor_seq":%d,"anchor_line_sha256":%q,"recorded_at":"2026-05-01T00:00:00Z","note":"test"}`, seq, lineSHA)
	if err := os.WriteFile(filepath.Join(dir, "ledger-anchor.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write ledger-anchor.json: %v", err)
	}
}

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
