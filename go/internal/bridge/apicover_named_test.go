package bridge

// apicover_named_test.go — public-API coverage closure (ADR-0050 Phase 5) for
// the test-support + DTO surface of internal/bridge that no prior test NAMED:
//
//   - FakeTmuxController.PaneCommand  — invoked + asserted (was UNCOVERED)
//   - FakeTmuxController.JiggleWindow — actually CALLED + effect asserted
//     (was FALSE-GREEN: only mentioned in a render_wedge_test.go comment)
//   - PaneCommander interface         — satisfaction proven + exercised
//   - Report / ArtifactRef / FileRef  — bound via the real BuildReport producer
//   - DoctorReport / DoctorResult / AuthInfo / BinaryInfo / DeepProbe — bound
//     via the real (*Engine).Doctor producer
//
// Each DTO is bound to a producer's OUTPUT (not a bare literal) and the
// producer-set fields are asserted, so the test verifies real wiring, not type
// shape alone. The methods are executed so they cannot regress to a false-green.

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestFakeTmuxController_PaneCommand_ReturnsScriptedAndRecords invokes
// PaneCommand (the previously UNCOVERED PaneCommander method): it must return
// the scripted PaneCmd value and append a "panecmd" event so callers that drive
// the boot handshake / post-paste spill check observe the probe in Events.
func TestFakeTmuxController_PaneCommand_ReturnsScriptedAndRecords(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &FakeTmuxController{PaneCmd: "node"} // a healthy claude REPL reports "node"

	got, err := f.PaneCommand(ctx, "sess")
	if err != nil {
		t.Fatalf("PaneCommand err = %v, want nil", err)
	}
	if got != "node" {
		t.Fatalf("PaneCommand = %q, want %q (the scripted PaneCmd)", got, "node")
	}
	if len(f.Events) != 1 || f.Events[0] != "panecmd" {
		t.Fatalf("Events = %v, want exactly [panecmd]", f.Events)
	}
}

// TestPaneCommander_SatisfiedAndExercised proves *FakeTmuxController satisfies
// the exported PaneCommander interface (the optional capability the handshake
// type-asserts for) and exercises it THROUGH the interface, so a controller
// that silently dropped the method would fail to compile here.
func TestPaneCommander_SatisfiedAndExercised(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var pc PaneCommander = &FakeTmuxController{PaneCmd: "zsh"} // a wedged shell reports "zsh"

	got, err := pc.PaneCommand(ctx, "sess")
	if err != nil {
		t.Fatalf("PaneCommander.PaneCommand err = %v, want nil", err)
	}
	if got != "zsh" {
		t.Fatalf("PaneCommander.PaneCommand = %q, want %q", got, "zsh")
	}
}

// TestFakeTmuxController_JiggleWindow_RecordsEffect CALLS JiggleWindow (the
// previously FALSE-GREEN windowJiggler method — named only in a comment before)
// and asserts its observable effect: a "jiggle:<session>" event, the signal the
// blank-pane render-wedge recovery path relies on.
func TestFakeTmuxController_JiggleWindow_RecordsEffect(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &FakeTmuxController{}

	if err := f.JiggleWindow(ctx, "wedged-sess"); err != nil {
		t.Fatalf("JiggleWindow err = %v, want nil", err)
	}
	if len(f.Events) != 1 || f.Events[0] != "jiggle:wedged-sess" {
		t.Fatalf("Events = %v, want exactly [jiggle:wedged-sess]", f.Events)
	}
}

// TestReport_BoundByBuildReport binds Report + ArtifactRef + FileRef to the
// REAL producer (BuildReport) over a constructed workspace and asserts the
// producer-derived fields: a present artifact containing the challenge token
// yields verdict "complete", ArtifactRef.HasChallengeToken true, and a present
// log file's FileRef carries Exists + a positive SizeBytes.
func TestReport_BoundByBuildReport(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	const token = "tok-abc123"
	mkfile(t, filepath.Join(ws, "artifact.md"), "result\nchallenge: "+token+"\n")
	mkfile(t, filepath.Join(ws, "challenge-token.txt"), token+"\n")
	mkfile(t, filepath.Join(ws, "stdout.log"), "hello stdout\n")

	var rep Report // names the Report DTO
	rep, err := BuildReport(ws, "artifact.md", now)
	if err != nil {
		t.Fatalf("BuildReport err = %v, want nil", err)
	}
	if rep.Verdict != "complete" {
		t.Fatalf("Report.Verdict = %q, want %q", rep.Verdict, "complete")
	}
	if rep.ScannedAt != "2026-06-16T12:00:00Z" {
		t.Fatalf("Report.ScannedAt = %q, want the injected UTC time", rep.ScannedAt)
	}

	art := rep.Artifact // ArtifactRef from the producer
	if !art.Exists || art.SizeBytes <= 0 {
		t.Fatalf("ArtifactRef = %+v, want Exists with positive SizeBytes", art)
	}
	if !art.HasChallengeToken {
		t.Fatal("ArtifactRef.HasChallengeToken = false, want true (token is inside the artifact)")
	}

	var stdoutLog FileRef = rep.Logs.StdoutLog // FileRef from the producer's statRef
	if !stdoutLog.Exists || stdoutLog.SizeBytes <= 0 {
		t.Fatalf("FileRef(stdout.log) = %+v, want Exists with positive SizeBytes", stdoutLog)
	}
	if stdoutLog.Path != filepath.Join(ws, "stdout.log") {
		t.Fatalf("FileRef.Path = %q, want the workspace stdout.log path", stdoutLog.Path)
	}
}

// TestReport_TokenMismatchVerdict binds Report + ArtifactRef again through the
// producer for the negative branch: an artifact present but NOT containing the
// token file's value yields "incomplete-token-mismatch" with HasChallengeToken
// false — proving the DTO carries the producer's discriminating state.
func TestReport_TokenMismatchVerdict(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	now := time.Now()

	mkfile(t, filepath.Join(ws, "artifact.md"), "result without the token\n")
	mkfile(t, filepath.Join(ws, "challenge-token.txt"), "expected-token\n")

	rep, err := BuildReport(ws, "artifact.md", now)
	if err != nil {
		t.Fatalf("BuildReport err = %v, want nil", err)
	}
	if rep.Verdict != "incomplete-token-mismatch" {
		t.Fatalf("Report.Verdict = %q, want incomplete-token-mismatch", rep.Verdict)
	}
	var art ArtifactRef = rep.Artifact // names the ArtifactRef DTO
	if !art.Exists || art.HasChallengeToken {
		t.Fatalf("ArtifactRef = %+v, want Exists=true HasChallengeToken=false", art)
	}
}

// TestDoctorReport_BoundByDoctor binds DoctorReport + DoctorResult + AuthInfo +
// BinaryInfo + DeepProbe to the REAL producer ((*Engine).Doctor) for a single
// fully-configured claude-p CLI (binary present, credentials file, deep probe
// passing) and asserts the producer-populated fields of each DTO.
func TestDoctorReport_BoundByDoctor(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	mkfile(t, filepath.Join(home, ".claude", ".credentials.json"), "{}")

	eng := doctorEngine(map[string]string{"HOME": home}, map[string]bool{"claude": true}, 0, nil)

	var rep DoctorReport // names the DoctorReport DTO
	rep, code := eng.Doctor(context.Background(), "claude-p", true /* deep */)
	if code != ExitOK {
		t.Fatalf("Doctor exit = %d, want ExitOK (ready)", code)
	}
	if rep.Host == "" || rep.ScannedAt == "" || !rep.Deep {
		t.Fatalf("DoctorReport = {Host:%q ScannedAt:%q Deep:%v}, want all populated + Deep", rep.Host, rep.ScannedAt, rep.Deep)
	}
	if rep.Summary.Ready != 1 {
		t.Fatalf("DoctorReport.Summary.Ready = %d, want 1", rep.Summary.Ready)
	}
	if len(rep.Results) != 1 {
		t.Fatalf("DoctorReport.Results length = %d, want 1 (filtered to claude-p)", len(rep.Results))
	}

	var res DoctorResult = rep.Results[0] // DoctorResult from the producer
	if res.CLI != "claude-p" || res.Verdict != "ready" {
		t.Fatalf("DoctorResult = {CLI:%q Verdict:%q}, want claude-p/ready", res.CLI, res.Verdict)
	}

	var bin BinaryInfo = res.Binary // BinaryInfo from the producer (LookPath + --version)
	if !bin.Present || bin.Path != "/usr/bin/claude" || bin.Version != "v1.2.3" {
		t.Fatalf("BinaryInfo = %+v, want Present /usr/bin/claude v1.2.3", bin)
	}

	auth := res.Auth // AuthInfo from doctorAuth
	if !auth.Configured || auth.Source != "file:credentials.json" {
		t.Fatalf("AuthInfo = %+v, want Configured via file:credentials.json", auth)
	}

	deep := res.DeepProbe // DeepProbe from doctorDeep (deep=true)
	if !deep.Ran || !deep.Passed {
		t.Fatalf("DeepProbe = %+v, want Ran && Passed", deep)
	}
}

// TestAuthInfo_OllamaOptional binds AuthInfo through the producer for the
// auth-optional branch: local-only ollama reports Configured=false yet
// AuthOptional=true, which is exactly what keeps it OFF the "blocked" verdict.
func TestAuthInfo_OllamaOptional(t *testing.T) {
	t.Parallel()
	eng := doctorEngine(map[string]string{"HOME": t.TempDir()}, nil, 0, nil)

	var auth AuthInfo = eng.doctorAuth("ollama-tmux") // names the AuthInfo DTO
	if auth.Configured {
		t.Fatalf("AuthInfo.Configured = true, want false for local-only ollama")
	}
	if !auth.AuthOptional {
		t.Fatal("AuthInfo.AuthOptional = false, want true (local ollama needs no auth)")
	}
}
