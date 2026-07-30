package core

// dossier_producer_failure_test.go — dossier-carries-failure-reason
// (pipeline-integrity): a FAIL cycle's committed dossier was content-free
// ("see audit-report.md"), so the knowledge base recorded THAT a cycle failed
// but not WHY — convergence briefs and cross-batch forensics needed workspace
// archaeology. The failure identity already exists on disk by dossier-write
// time (<workspace>/failure-digest.json from ensureFailureDigest +
// <workspace>/audit-fail-reason.json from the coherence floor / fallback
// writer), so writeCycleDossier must carry it into
// knowledge-base/cycles/cycle-N.{json,md}. Best-effort: absent artifacts
// never block the write; a PASS dossier's shape is untouched.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

const testFailFingerprint = "audit|gate-block|ab12cd34ef56"

// writeFailureArtifacts drops production-shaped failure artifacts into ws —
// the same schemas ensureFailureDigest and the coherence floor write.
func writeFailureArtifacts(t *testing.T, ws string, reasons []string) {
	t.Helper()
	rb, err := json.Marshal(map[string]any{
		"schema_version": 1,
		"phase":          "audit",
		"reasons":        reasons,
	})
	if err != nil {
		t.Fatalf("marshal audit-fail-reason: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), rb, 0o644); err != nil {
		t.Fatalf("write audit-fail-reason.json: %v", err)
	}
	db, err := json.Marshal(map[string]any{
		"cycle":       9,
		"fingerprint": testFailFingerprint,
		"pre_class":   "gate-block",
		"recurrence":  2,
	})
	if err != nil {
		t.Fatalf("marshal failure-digest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "failure-digest.json"), db, 0o644); err != nil {
		t.Fatalf("write failure-digest.json: %v", err)
	}
}

// readDossierPair reads cycle-N.{json,md} from root's knowledge base, returning
// the raw JSON as a map (so absence of a key is assertable) plus the markdown.
func readDossierPair(t *testing.T, root string, cycle int) (map[string]any, string) {
	t.Helper()
	base := filepath.Join(root, "knowledge-base", "cycles", fmt.Sprintf("cycle-%d", cycle))
	jb, err := os.ReadFile(base + ".json")
	if err != nil {
		t.Fatalf("dossier json not written: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(jb, &m); err != nil {
		t.Fatalf("dossier json unparseable: %v", err)
	}
	mb, err := os.ReadFile(base + ".md")
	if err != nil {
		t.Fatalf("dossier md not written: %v", err)
	}
	return m, string(mb)
}

// TestDossierFailure_FailCarriesIdentity is the core RED of the item: a FAIL
// dossier carries the digest fingerprint + pre_class + the real reasons[], not
// just the content-free "see audit-report" pointer.
func TestDossierFailure_FailCarriesIdentity(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	ws := t.TempDir()
	reasons := []string{
		"EGPS gate blocked ship: red_count=2 (TestCN_004_Contract)",
		"predicate TestCN_007 failed to compile",
	}
	writeFailureArtifacts(t, ws, reasons)

	if err := writeCycleDossier(nil, root, ws, 9, "fix Z", "run9", VerdictFAIL, nil, nil); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	m, md := readDossierPair(t, root, 9)

	fb, ok := m["failure"].(map[string]any)
	if !ok {
		t.Fatalf("FAIL dossier carries no failure block; keys=%v", dossierTopLevelKeys(m))
	}
	if got := fb["fingerprint"]; got != testFailFingerprint {
		t.Errorf("failure.fingerprint = %v, want %q", got, testFailFingerprint)
	}
	if got := fb["pre_class"]; got != "gate-block" {
		t.Errorf("failure.pre_class = %v, want %q", got, "gate-block")
	}
	rs, ok := fb["reasons"].([]any)
	if !ok || len(rs) != len(reasons) {
		t.Fatalf("failure.reasons = %v, want %d reasons", fb["reasons"], len(reasons))
	}
	for i, want := range reasons {
		if rs[i] != want {
			t.Errorf("failure.reasons[%d] = %v, want %q", i, rs[i], want)
		}
	}
	if !strings.Contains(md, testFailFingerprint) {
		t.Errorf("dossier md must carry the failure fingerprint; got:\n%s", md)
	}
	if !strings.Contains(md, reasons[0]) {
		t.Errorf("dossier md must carry the failure reasons; got:\n%s", md)
	}
}

// TestDossierFailure_ReasonsTruncatedAndCapped bounds the carried evidence:
// each reason is truncated to ~200 chars and at most 5 reasons are carried, so
// a pathological reason set can never bloat the committed knowledge base.
func TestDossierFailure_ReasonsTruncatedAndCapped(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	ws := t.TempDir()
	var reasons []string
	for i := 0; i < 7; i++ {
		reasons = append(reasons, fmt.Sprintf("r%d-", i)+strings.Repeat("x", 300))
	}
	writeFailureArtifacts(t, ws, reasons)

	if err := writeCycleDossier(nil, root, ws, 10, "fix W", "run10", VerdictFAIL, nil, nil); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	m, _ := readDossierPair(t, root, 10)
	fb, ok := m["failure"].(map[string]any)
	if !ok {
		t.Fatal("FAIL dossier carries no failure block")
	}
	rs, ok := fb["reasons"].([]any)
	if !ok {
		t.Fatal("failure block carries no reasons")
	}
	if len(rs) != 5 {
		t.Errorf("failure.reasons carries %d entries, want cap of 5", len(rs))
	}
	for i, r := range rs {
		s, _ := r.(string)
		if len(s) > 200 {
			t.Errorf("failure.reasons[%d] is %d chars, want <= 200", i, len(s))
		}
		if !strings.HasPrefix(s, fmt.Sprintf("r%d-", i)) {
			t.Errorf("failure.reasons[%d] lost its head under truncation: %q", i, s[:8])
		}
	}
}

// TestDossierFailure_AbsentArtifactsDegrade proves best-effort: a FAIL cycle
// whose workspace carries neither artifact still writes a valid dossier — the
// failure block is simply absent, never a blocked write.
func TestDossierFailure_AbsentArtifactsDegrade(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	if err := writeCycleDossier(nil, root, t.TempDir(), 11, "fix V", "run11", VerdictFAIL, nil, nil); err != nil {
		t.Fatalf("writeCycleDossier must not fail on absent failure artifacts: %v", err)
	}
	m, md := readDossierPair(t, root, 11)
	if _, present := m["failure"]; present {
		t.Errorf("no artifacts on disk ⇒ no failure block; got %v", m["failure"])
	}
	if strings.Contains(md, "## Failure") {
		t.Errorf("no artifacts on disk ⇒ no Failure section in md")
	}
	// The written pair must still parse + validate as before.
	jb, err := os.ReadFile(filepath.Join(root, "knowledge-base", "cycles", "cycle-11.json"))
	if err != nil {
		t.Fatalf("read dossier: %v", err)
	}
	d, err := dossier.ParseJSON(jb)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := d.Validate(); err != nil {
		t.Errorf("degraded FAIL dossier must stay valid: %v", err)
	}
}

// TestDossierFailure_PassKeepsShape proves a PASS dossier's byte-shape is
// unchanged even when stale failure artifacts linger in the workspace (a
// retried cycle that eventually passed): no failure block, no md section.
func TestDossierFailure_PassKeepsShape(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	ws := t.TempDir()
	writeFailureArtifacts(t, ws, []string{"stale reason from a retried attempt"})

	if err := writeCycleDossier(nil, root, ws, 12, "improve U", "run12", CycleOutcomeShippedViaBuild, nil, nil); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	m, md := readDossierPair(t, root, 12)
	if _, present := m["failure"]; present {
		t.Errorf("PASS dossier must carry no failure block; got %v", m["failure"])
	}
	if strings.Contains(md, "## Failure") {
		t.Errorf("PASS dossier md must carry no Failure section")
	}
}

// dossierTopLevelKeys lists a decoded object's keys for failure messages.
func dossierTopLevelKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
