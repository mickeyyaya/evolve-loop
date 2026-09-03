package core

// repair_brief_test.go — R2: the repair brief carries the auditor's findings,
// not only the gate strings, and names the ones that persisted from the
// previous round; the previous attempt's prompt is archived. Grammar is
// reportdoc's (the dashboard renders the same list).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const briefRound1 = "# Audit Report — Cycle 1605 (round 1)\n\n## Verdict\n\n**FAIL**\n\n## Issues\n\n" +
	"### H1 (HIGH) — caller-proof hard floor violated: decisionsample exports have no callers\nb\n\n" +
	"### M1 (MEDIUM) — explanation names an area not in the diff\nb\n"
const briefRound2 = "# Audit Report — Cycle 1605 (round 2)\n\n## Verdict\n\n**FAIL**\n\n## Issues\n\n" +
	"### H1 (HIGH) — caller-proof hard floor violated for the second consecutive round\nb\n\n" +
	"### H2 (HIGH) — a scout-selected slug has no eval\nb\n\n### L1 (LOW, scope — disclosed)\nb\n"

func writeBriefFixture(t *testing.T, ws, name, body string) {
	t.Helper()
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestComposeRepairBrief_GateReasonsThenFindingsThenPersisted(t *testing.T) {
	ws := t.TempDir()
	writeAuditFailReason(t, ws, "audit", "EGPS: acs-verdict.json ship_eligible=false")
	auditName := phasecontract.ArtifactFilename(string(PhaseAudit))
	writeBriefFixture(t, ws, phasecontract.RoundArchiveFilename(auditName, 1), briefRound1)
	writeBriefFixture(t, ws, auditName, briefRound2)

	brief := composeRepairBrief(CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 2, AuditDispatches: 2})
	gate := strings.Index(brief, "EGPS: acs-verdict.json ship_eligible=false")
	h1 := strings.Index(brief, "H1 (HIGH) — caller-proof hard floor violated for the second consecutive round")
	h2 := strings.Index(brief, "H2 (HIGH) — a scout-selected slug has no eval")
	if gate < 0 || h1 < 0 || h2 < 0 || !(gate < h1 && h1 < h2) {
		t.Fatalf("brief must carry gate reasons, then the findings in severity order:\n%s", brief)
	}
	if !strings.Contains(brief, "auditor findings (audit round 2") {
		t.Fatalf("brief must name the round:\n%s", brief)
	}
	if !strings.Contains(brief[h1:h2], "PERSISTED from the previous round") {
		t.Fatalf("H1 was cited in round 1 and again in round 2 — it must be marked persisted:\n%s", brief)
	}
	if strings.Contains(brief[h2:], "PERSISTED") {
		t.Fatalf("H2 is new in round 2 and must not be marked persisted:\n%s", brief)
	}
	if strings.Contains(brief, "L1") || strings.Contains(brief, "scope — disclosed") {
		t.Fatalf("LOW findings are advisory and must not consume the builder's brief:\n%s", brief)
	}
}

func TestComposeRepairBrief_FindingsAloneStillSeed(t *testing.T) {
	ws := t.TempDir()
	writeBriefFixture(t, ws, phasecontract.ArtifactFilename(string(PhaseAudit)), briefRound1)
	cs := CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1, AuditDispatches: 1}
	brief := composeRepairBrief(cs)
	if !strings.Contains(brief, "H1 (HIGH)") || strings.Contains(brief, "PERSISTED") {
		t.Fatalf("round-1 report with no gate reasons must still brief the findings, none persisted:\n%s", brief)
	}
	got := seedAuditRepairContext(map[string]string{}, PhaseBuild, cs)
	if !strings.Contains(got[CtxKeyAuditRepairFindings], "H1 (HIGH)") {
		t.Fatalf("seed must carry the findings brief even without audit-fail-reason.json: %+v", got)
	}
	if brief := composeRepairBrief(CycleState{WorkspacePath: t.TempDir(), AuditRepairActive: true}); brief != "" {
		t.Fatalf("no artifacts ⇒ empty brief, got %q", brief)
	}
}

func TestArchiveRepairPrompts_RenamesOnceNeverClobbers(t *testing.T) {
	ws := t.TempDir()
	writeBriefFixture(t, ws, "build-prompt.txt", "round one prompt")
	archiveRepairPrompts(CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1}, PhaseBuild)
	archived := filepath.Join(ws, phasecontract.RoundArchiveFilename("build-prompt.txt", 1))
	if b, err := os.ReadFile(archived); err != nil || string(b) != "round one prompt" {
		t.Fatalf("round-1 prompt not archived: %v %q", err, b)
	}
	if _, err := os.Stat(filepath.Join(ws, "build-prompt.txt")); !os.IsNotExist(err) {
		t.Fatal("live prompt must be moved, not copied (the bridge rewrites it)")
	}
	// A counter rollback re-archives round 1: the first evidence wins.
	writeBriefFixture(t, ws, "build-prompt.txt", "a later prompt")
	archiveRepairPrompts(CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1}, PhaseBuild)
	if b, _ := os.ReadFile(archived); string(b) != "round one prompt" {
		t.Fatalf("existing archive clobbered: %q", b)
	}
	archiveRepairPrompts(CycleState{WorkspacePath: ws, AuditRepairActive: true}, PhaseBuild)                                // round 0: no-op
	archiveRepairPrompts(CycleState{AuditRepairActive: true, AuditRepairAttempts: 1}, PhaseBuild)                           // no workspace: no-op
	archiveRepairPrompts(CycleState{WorkspacePath: t.TempDir(), AuditRepairActive: true, AuditRepairAttempts: 1}, PhaseTDD) // absent prompt: quiet
	archiveRepairPrompts(CycleState{WorkspacePath: ws, AuditRepairAttempts: 1}, PhaseBuild)                                 // repair not active: self-guarded no-op
	archiveRepairPrompts(CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1}, PhaseAudit)        // audit is never repair-seeded: no-op
}

// findingsAuditRunner is a live-shape auditor: round 1 writes a real-format
// FAIL report with headings (and, standing in for the bridge, the build prompt
// the round was given); round 2 PASSes. The build fakeRunner captures the
// requests, so the round-2 brief is asserted on the dispatched Context.
type findingsAuditRunner struct {
	t    *testing.T
	runs int
	ws   string
}

func (r *findingsAuditRunner) Name() string { return string(PhaseAudit) }

func (r *findingsAuditRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.runs++
	r.ws = req.Workspace
	if r.runs == 1 {
		writeBriefFixture(r.t, req.Workspace, "build-prompt.txt", "the round-1 build prompt")
		report := briefRound1 + "\n" + phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL",
			&phasecontract.FailureBlock{Class: "code-audit-fail", Defects: []string{"H1 caller-proof hard floor violated"}}) + "\n"
		writeBriefFixture(r.t, req.Workspace, phasecontract.ArtifactFilename(string(PhaseAudit)), report)
		return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictFAIL, ArtifactsDir: req.Workspace}, nil
	}
	return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

// TestRepairRoundDispatch_BriefCarriesAuditorFindingsAndArchivesPrompt is the
// wiring proof through RunCycle: the repair-round build is TOLD the auditor's
// H1 (not only gate strings) and the round-1 prompt is archived.
func TestRepairRoundDispatch_BriefCarriesAuditorFindingsAndArchivesPrompt(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(map[Phase]string{PhaseRetro: VerdictFAIL})
	ar := &findingsAuditRunner{t: t}
	runners[PhaseAudit] = ar
	o := NewOrchestrator(st, &fakeLedger{}, runners)
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if ar.runs < 2 {
		t.Fatalf("audit ran %d time(s); no repair round happened", ar.runs)
	}
	fr := runners[PhaseBuild].(*fakeRunner)
	if len(fr.requests) < 2 {
		t.Fatalf("build dispatched %d time(s); the repair round never re-dispatched build", len(fr.requests))
	}
	brief := fr.requests[len(fr.requests)-1].Context[CtxKeyAuditRepairFindings]
	if !strings.Contains(brief, "H1 (HIGH) — caller-proof hard floor violated") {
		t.Fatalf("repair-round build was not told the auditor's H1:\n%q", brief)
	}
	if _, err := os.Stat(filepath.Join(ar.ws, phasecontract.RoundArchiveFilename("build-prompt.txt", 1))); err != nil {
		t.Fatalf("round-1 build prompt not archived: %v", err)
	}
}

// TestAuditorFindingsBrief_UnreadableReportWarns — a report that EXISTS but
// cannot be read is the "looked in the wrong place" class, and must be
// reported on stderr like the gate reader beside it; plain absence stays quiet.
func TestAuditorFindingsBrief_UnreadableReportWarns(t *testing.T) {
	ws := t.TempDir()
	auditName := phasecontract.ArtifactFilename(string(PhaseAudit))
	if err := os.MkdirAll(filepath.Join(ws, auditName), 0o755); err != nil { // a directory where the file should be
		t.Fatal(err)
	}
	var brief string
	stderr := captureStderr(t, func() { brief = auditorFindingsBrief(ws, 1) })
	if brief != "" {
		t.Fatalf("unreadable report must yield no brief, got %q", brief)
	}
	if !strings.Contains(stderr, "WARN audit-repair") || !strings.Contains(stderr, auditName) {
		t.Fatalf("unreadable report must be reported loudly, stderr=%q", stderr)
	}
	if got := captureStderr(t, func() { _ = auditorFindingsBrief(t.TempDir(), 1) }); strings.Contains(got, "WARN") {
		t.Fatalf("a merely absent report is legitimate and must stay quiet, stderr=%q", got)
	}
}

// TestTruncateFindings_CutsAtLineBoundary — the brief budget cut lands on a
// line break, never mid-rune or mid-finding (the em dashes and ellipses the
// auditor findings carry are multi-byte).
func TestTruncateFindings_CutsAtLineBoundary(t *testing.T) {
	line := strings.Repeat("x", 100) + " — finding…\n" // 100 ASCII + multi-byte tail
	s := strings.Repeat(line, maxFindingsBytes/len(line)+3)
	got := truncateFindings(s)
	body, marker, ok := strings.Cut(got, "\n…[truncated ")
	if !ok || !strings.HasSuffix(marker, " bytes]") {
		t.Fatalf("truncation marker missing: %q", got[len(got)-80:])
	}
	if !strings.HasSuffix(body, "finding…") {
		t.Fatalf("cut must land on a line boundary, body ends %q", body[len(body)-30:])
	}
	if len(body) > maxFindingsBytes || !utf8.ValidString(got) {
		t.Fatalf("truncated brief must stay within budget and valid UTF-8 (len=%d)", len(body))
	}
	if short := "a\nb"; truncateFindings(short) != short {
		t.Fatal("under-budget input must pass through untouched")
	}
}
