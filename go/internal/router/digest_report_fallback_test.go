package router

import (
	"os"
	"strings"
	"testing"
)

func mkWorkspace(t *testing.T) string {
	t.Helper()
	ws := runWorkspacePath(t.TempDir(), 900)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	return ws
}

func writeWorkspaceFile(t *testing.T, ws, name, content string) {
	t.Helper()
	if err := os.WriteFile(ws+"/"+name, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestDigest_Scout_FallsBackToReportWhenHandoffAbsent(t *testing.T) {
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "scout-report.md", "# Scout Report\n\nbacklog scanned; task selected.\n")

	sig, err := Digest(ws, []string{"scout"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if !sig.Scout.Present {
		t.Fatal("Digest(no handoff, scout-report.md present).Scout.Present = false, want true (report fallback)")
	}
	if len(sig.DigestDegraded) != 0 {
		t.Errorf("healthy report fallback must not degrade; DigestDegraded = %v", sig.DigestDegraded)
	}
}

func TestDigest_Scout_NoArtifactsStaysCleanAbsence(t *testing.T) {
	ws := mkWorkspace(t)

	sig, err := Digest(ws, []string{"scout"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if sig.Scout.Present {
		t.Fatal("Digest(no artifacts).Scout.Present = true, want false")
	}
	if len(sig.DigestDegraded) != 0 {
		t.Errorf("clean absence must not degrade; DigestDegraded = %v", sig.DigestDegraded)
	}
}

func TestDigest_Audit_FallsBackToACSVerdictWhenHandoffAbsent(t *testing.T) {
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "acs-verdict.json",
		`{"schema_version":"1.0","cycle":900,"green_count":10,"red_count":0,"verdict":"PASS","ship_eligible":true}`)

	sig, err := Digest(ws, []string{"audit"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if !sig.Audit.Present {
		t.Fatal("Digest(no handoff, acs-verdict.json present).Audit.Present = false, want true (acs-verdict fallback)")
	}
	if sig.Audit.Verdict != "PASS" {
		t.Errorf("Audit.Verdict = %q, want PASS (from acs-verdict.json)", sig.Audit.Verdict)
	}
	if sig.Audit.RedCount != 0 {
		t.Errorf("Audit.RedCount = %d, want 0", sig.Audit.RedCount)
	}
}

func TestDigest_Audit_FAILVerdictCarriedHonestly(t *testing.T) {
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "acs-verdict.json",
		`{"schema_version":"1.0","cycle":900,"green_count":8,"red_count":2,"verdict":"FAIL","ship_eligible":false}`)

	sig, err := Digest(ws, []string{"audit"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if !sig.Audit.Present {
		t.Fatal("a FAIL acs-verdict still proves the audit RAN — Present must be true")
	}
	if sig.Audit.Verdict != "FAIL" || sig.Audit.RedCount != 2 {
		t.Errorf("Audit = {Verdict:%q RedCount:%d}, want {FAIL 2} — the fallback must carry red results honestly", sig.Audit.Verdict, sig.Audit.RedCount)
	}
}

func TestDigest_Audit_VerdictlessACSVerdictDegradesLoudly(t *testing.T) {
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "acs-verdict.json", `{"red_count": 0, "yellow_count": 0, "green_count": 1}`)

	sig, err := Digest(ws, []string{"audit"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if sig.Audit.Present {
		t.Error("verdict-less acs-verdict must not report Present:true (an unsatisfiable anchor)")
	}
	found := false
	for _, d := range sig.DigestDegraded {
		if strings.Contains(d, "verdict") {
			found = true
		}
	}
	if !found {
		t.Errorf("schema-drifted acs-verdict must degrade LOUDLY naming the missing verdict; DigestDegraded = %v", sig.DigestDegraded)
	}
}

func TestDigest_Audit_CorruptACSVerdictDegradesLoudly(t *testing.T) {
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "acs-verdict.json", `{"verdict": TRUNCATED`)

	sig, err := Digest(ws, []string{"audit"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if sig.Audit.Present {
		t.Fatal("corrupt acs-verdict.json must not report Present:true")
	}
	found := false
	for _, d := range sig.DigestDegraded {
		if strings.Contains(strings.ToLower(d), "audit") || strings.Contains(d, "acs-verdict") {
			found = true
		}
	}
	if !found {
		t.Errorf("corrupt acs-verdict fallback must degrade loudly; DigestDegraded = %v", sig.DigestDegraded)
	}
}

func TestDigest_Audit_HandoffStillWinsOverFallback(t *testing.T) {
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "handoff-audit.json", `{"verdict":"WARN","red_count":1,"confidence":0.7}`)
	writeWorkspaceFile(t, ws, "acs-verdict.json", `{"verdict":"PASS","red_count":0}`)

	sig, err := Digest(ws, []string{"audit"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if sig.Audit.Verdict != "WARN" || sig.Audit.RedCount != 1 {
		t.Errorf("handoff must stay authoritative over the fallback; got {Verdict:%q RedCount:%d}, want {WARN 1}", sig.Audit.Verdict, sig.Audit.RedCount)
	}
}
