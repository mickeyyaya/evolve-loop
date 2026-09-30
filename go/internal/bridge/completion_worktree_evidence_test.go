package bridge

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const staleBuildReport = "<!-- challenge-token: tok-claude-tmux -->\n# Build Report\n## Verdict\nPASS\nfrom the earlier attempt\n"

const busyClaudePane = tmuxPromptMarkerDefault + "\n✻ Working… (esc to interrupt)\n"

type actingAgentTmux struct {
	*fakeTmux
	onPrompt func()
	onNudge  func()
}

func (a *actingAgentTmux) PasteBuffer(ctx context.Context, session string) error {
	if a.onPrompt != nil {
		a.onPrompt()
	}
	return a.fakeTmux.PasteBuffer(ctx, session)
}

func (a *actingAgentTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if a.onNudge != nil && strings.Contains(keys, "was not rewritten") {
		a.onNudge()
	}
	return a.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func (a *actingAgentTmux) nudges() int {
	count := 0
	for _, keys := range a.sentKeys {
		if strings.Contains(keys, "was not rewritten") {
			count++
		}
	}
	return count
}

func idleAgent(onPrompt func()) *actingAgentTmux {
	return &actingAgentTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}, onPrompt: onPrompt}
}

type evidenceLaunch struct {
	fx         launchFixture
	worktree   string
	completion core.CompletionContract
}

type evidenceResult struct {
	code   int
	stderr string
	events []signalcenter.Event
}

func committedWorktree(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "docs", "explain.md"), "# Explanation\n## Changed Areas\n")
	writeFile(t, filepath.Join(repo.Dir, "src", "feature.go"), "package feature\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	return repo.Dir
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func correctionOfABuild(t *testing.T) evidenceLaunch {
	t.Helper()
	fx := newFixture(t, "claude-tmux", "")
	writeArtifact(t, fx.artifact, staleBuildReport, fixedMTime)
	return evidenceLaunch{fx: fx, worktree: committedWorktree(t), completion: core.CompletionWorktreeEvidence}
}

func (l evidenceLaunch) run(t *testing.T, tmux TmuxController) evidenceResult {
	t.Helper()
	center := signalcenter.New()
	eng := NewEngine(Deps{
		Tmux:            tmux,
		Sleep:           func(time.Duration) {},
		LookupEnv:       mapLookup(nil),
		CaptureBaseline: captureArtifactBaseline,
		SandboxWrap:     noSandboxWrap(),
		Signals:         center,
	})
	args := l.fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=7", "--worktree="+l.worktree)
	if l.completion != "" {
		args = append(args, "--completion="+string(l.completion))
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(ctx, args, nil, &stdout, &stderr)
	center.Flush()
	return evidenceResult{code: code, stderr: stderr.String(), events: center.Recent()}
}

func (r evidenceResult) evidenceEvents() []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range r.events {
		if e.Code == CodeCompletedOnWorktreeEvidence {
			out = append(out, e)
		}
	}
	return out
}

func TestWorktreeEvidence_AFixOutsideAnUntouchedDeliverableCompletesWithoutOperatorAction(t *testing.T) {
	launch := correctionOfABuild(t)
	tmux := idleAgent(func() {
		writeFile(t, filepath.Join(launch.worktree, "docs", "explain.md"), "# Explanation\n## Changed Areas\n- `src/feature.go` — what and why\n")
	})
	res := launch.run(t, tmux)
	if res.code != ExitOK {
		t.Fatalf("a correction whose agent fixed a non-deliverable file and went idle must complete: exit=%d stderr=%q", res.code, res.stderr)
	}
	if got, err := os.ReadFile(launch.fx.artifact); err != nil || string(got) != staleBuildReport {
		t.Fatalf("the carried deliverable must reach the host's verify untouched: %q (%v)", got, err)
	}
	if n := tmux.nudges(); n != 0 {
		t.Errorf("worktree evidence completes at the first idle checkpoint, before any nudge; %d nudge(s) sent", n)
	}
}

func TestWorktreeEvidence_TheBridgeReportsTheCarriedDeliverableAndTheAgentsPaths(t *testing.T) {
	launch := correctionOfABuild(t)
	res := launch.run(t, idleAgent(func() {
		writeFile(t, filepath.Join(launch.worktree, "docs", "explain.md"), "# Explanation\n- what and why\n")
	}))
	if res.code != ExitOK {
		t.Fatalf("exit=%d stderr=%q", res.code, res.stderr)
	}
	for _, want := range []string{launch.fx.artifact, "was not rewritten", "docs/explain.md"} {
		if !strings.Contains(res.stderr, want) {
			t.Errorf("the completion note must name %q: %q", want, res.stderr)
		}
	}
	events := res.evidenceEvents()
	if len(events) != 1 {
		t.Fatalf("want one %s signal, got %d in %+v", CodeCompletedOnWorktreeEvidence, len(events), res.events)
	}
	fields := events[0].Fields
	if fields["deliverable"] != launch.fx.artifact || fields["changed_paths"] != "1" || fields["paths"] != "docs/explain.md" {
		t.Errorf("the signal must carry the deliverable and the worktree delta: %+v", fields)
	}
	if events[0].Phase != "build" || events[0].Cycle != 7 || events[0].Severity != signalcenter.SeverityWarn {
		t.Errorf("the signal must be a WARN bound to the dispatch identity: %+v", events[0])
	}
}

func TestWorktreeEvidence_AnAgentThatChangesNothingNeverCompletesOnTheStaleDeliverable(t *testing.T) {
	launch := correctionOfABuild(t)
	tmux := idleAgent(nil)
	res := launch.run(t, tmux)
	if res.code != ExitArtifactTimeout {
		t.Fatalf("an agent that changed nothing must not complete on the stale deliverable: exit=%d stderr=%q", res.code, res.stderr)
	}
	if n := tmux.nudges(); n != 1 {
		t.Errorf("with no evidence the explaining nudge is still the one reminder; got %d", n)
	}
	if n := len(res.evidenceEvents()); n != 0 {
		t.Errorf("no worktree-evidence completion may be reported; got %d", n)
	}
}

func TestWorktreeEvidence_AFirstDispatchStillRefusesALeftoverDespiteWorktreeEdits(t *testing.T) {
	launch := correctionOfABuild(t)
	launch.completion = ""
	tmux := idleAgent(func() {
		writeFile(t, filepath.Join(launch.worktree, "src", "feature.go"), "package feature\n\nfunc Built() {}\n")
	})
	res := launch.run(t, tmux)
	if res.code != ExitArtifactTimeout || tmux.nudges() != 1 {
		t.Fatalf("the artifact contract must keep refusing a pre-dispatch leftover at every idle checkpoint whatever the worktree shows: exit=%d nudges=%d stderr=%q", res.code, tmux.nudges(), res.stderr)
	}
}

func TestWorktreeEvidence_HostWritesNeverCountAsAgentAction(t *testing.T) {
	worktree := committedWorktree(t)
	ws := filepath.Join(worktree, "runs", "cycle-7")
	fx := launchFixture{
		ws:         ws,
		profile:    writeProfile(t, t.TempDir(), "test-claude-tmux", ""),
		promptFile: filepath.Join(ws, "prompt.txt"),
		artifact:   filepath.Join(ws, "build-report.md"),
		stdoutLog:  filepath.Join(ws, "stdout.log"),
		stderrLog:  filepath.Join(ws, "stderr.log"),
		token:      "tok-claude-tmux",
	}
	writeFile(t, fx.promptFile, "Fix the rejected build deliverable.\n")
	writeArtifact(t, fx.artifact, staleBuildReport, fixedMTime)
	launch := evidenceLaunch{fx: fx, worktree: worktree, completion: core.CompletionWorktreeEvidence}
	tmux := idleAgent(func() {
		writeFile(t, filepath.Join(ws, "challenge-token.txt"), "tok-rotated\n")
		writeFile(t, filepath.Join(ws, "llm-calls.ndjson"), "{\"phase\":\"build\"}\n")
		writeFile(t, filepath.Join(worktree, ".evolve", "cycle-state.json"), "{\"phase\":\"build\"}\n")
	})
	res := launch.run(t, tmux)
	if res.code != ExitArtifactTimeout || tmux.nudges() != 1 {
		t.Fatalf("challenge token, telemetry and .evolve/ state are host writes, never the agent's action, at every idle checkpoint: exit=%d nudges=%d stderr=%q", res.code, tmux.nudges(), res.stderr)
	}
	if n := len(res.evidenceEvents()); n != 0 {
		t.Errorf("host writes produced %d worktree-evidence completion(s)", n)
	}
}

func TestWorktreeEvidence_ABusyAgentIsNotCompletedMidTurn(t *testing.T) {
	launch := correctionOfABuild(t)
	tmux := &actingAgentTmux{fakeTmux: &fakeTmux{paneSeq: []string{busyClaudePane}}, onPrompt: func() {
		writeFile(t, filepath.Join(launch.worktree, "src", "feature.go"), "package feature\n\nfunc HalfDone() {}\n")
	}}
	res := launch.run(t, tmux)
	if res.code != ExitArtifactTimeout || !strings.Contains(res.stderr, "→ pause") {
		t.Fatalf("a paused but busy agent is never completed on worktree evidence: exit=%d stderr=%q", res.code, res.stderr)
	}
	if n := len(res.evidenceEvents()); n != 0 {
		t.Errorf("a busy agent was completed on worktree evidence %d time(s)", n)
	}
}

func TestWorktreeEvidence_AnAgentThatActsAfterTheNudgeCompletesAtTheNextIdle(t *testing.T) {
	launch := correctionOfABuild(t)
	tmux := idleAgent(nil)
	tmux.onNudge = func() {
		writeFile(t, filepath.Join(launch.worktree, "docs", "explain.md"), "# Explanation\n- fixed after the reminder\n")
	}
	res := launch.run(t, tmux)
	if res.code != ExitOK {
		t.Fatalf("an agent that fixes the worktree after the nudge completes at the next idle checkpoint: exit=%d stderr=%q", res.code, res.stderr)
	}
	if n := tmux.nudges(); n != 1 {
		t.Errorf("want exactly the one nudge before the evidence appeared; got %d", n)
	}
}

func TestWorktreeEvidence_ARewrittenDeliverableStillCompletesByTheArtifactWindow(t *testing.T) {
	launch := correctionOfABuild(t)
	res := launch.run(t, idleAgent(func() {
		writeArtifact(t, launch.fx.artifact, staleBuildReport+"\n## Correction 1\nfixed the explanation doc\n", fixedMTime.Add(time.Hour))
	}))
	if res.code != ExitOK || !strings.Contains(res.stderr, "artifact appeared") {
		t.Fatalf("a rewritten deliverable completes by the ordinary window: exit=%d stderr=%q", res.code, res.stderr)
	}
	if n := len(res.evidenceEvents()); n != 0 {
		t.Errorf("a rewritten deliverable needs no worktree evidence; %d signal(s) emitted", n)
	}
}

func TestWorktreeEvidence_NoDispatchSnapshotMeansNoEvidenceEvenOnceGitRecovers(t *testing.T) {
	launch := correctionOfABuild(t)
	launch.worktree = t.TempDir()
	recovered := gittest.Fixture(t)
	res := launch.run(t, idleAgent(func() {
		if err := os.Rename(filepath.Join(recovered.Dir, ".git"), filepath.Join(launch.worktree, ".git")); err != nil {
			t.Errorf("git recovers in the worktree: %v", err)
		}
		writeFile(t, filepath.Join(launch.worktree, "feature.go"), "package feature\n")
	}))
	if res.code != ExitArtifactTimeout {
		t.Fatalf("with no dispatch snapshot there is nothing to compare, so the stale deliverable must not complete: exit=%d stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "worktree snapshot failed") {
		t.Errorf("the failed snapshot must be reported, not swallowed: %q", res.stderr)
	}
}

func TestWorktreeSnapshot_ChangedSinceSeesEveryAgentEditAndNothingElse(t *testing.T) {
	repo := gittest.Fixture(t)
	for _, name := range []string{"clean.go", "dirty.go", "revert.go", "untouched.go"} {
		writeFile(t, filepath.Join(repo.Dir, name), "package x\n")
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	writeFile(t, filepath.Join(repo.Dir, "dirty.go"), "package x\n\nvar A = 1\n")
	writeFile(t, filepath.Join(repo.Dir, "revert.go"), "package x\n\nvar R = 1\n")
	writeFile(t, filepath.Join(repo.Dir, "untouched.go"), "package x\n\nvar U = 1\n")
	cfg := &Config{Worktree: repo.Dir, Workspace: t.TempDir(), Artifact: filepath.Join(t.TempDir(), "build-report.md")}
	deps := Deps{}.withDefaults()
	before := captureWorktreeSnapshot(context.Background(), cfg, deps)
	if !before.captured {
		t.Fatal("a readable worktree must be captured")
	}

	writeFile(t, filepath.Join(repo.Dir, "dirty.go"), "package x\n\nvar A = 22\n")
	if err := os.Remove(filepath.Join(repo.Dir, "clean.go")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo.Dir, "revert.go"), "package x\n")
	writeFile(t, filepath.Join(repo.Dir, "new.go"), "package x\n")

	got := captureWorktreeSnapshot(context.Background(), cfg, deps).changedSince(before)
	want := []string{"clean.go", "dirty.go", "new.go", "revert.go"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("changedSince = %v, want %v (a deleted, re-edited, created and reverted file each count; an untouched dirty file does not)", got, want)
	}
}

func evidenceDetector(t *testing.T, secondaries ...string) (*worktreeEvidenceDetector, *Config) {
	t.Helper()
	cfg := evidenceConfig(t, secondaries...)
	return detectorOver(t, cfg), cfg
}

func evidenceConfig(t *testing.T, secondaries ...string) *Config {
	t.Helper()
	ws := t.TempDir()
	cfg := &Config{
		Workspace: ws, Worktree: committedWorktree(t), Artifact: filepath.Join(ws, "build-report.md"),
		SecondaryArtifacts: secondaries, Completion: core.CompletionWorktreeEvidence,
	}
	writeArtifact(t, cfg.Artifact, staleBuildReport, fixedMTime)
	writeFile(t, filepath.Join(cfg.Worktree, "src", "feature.go"), "package feature\n\nvar Dirty = 1\n")
	return cfg
}

func detectorOver(t *testing.T, cfg *Config) *worktreeEvidenceDetector {
	t.Helper()
	deps := Deps{Stderr: io.Discard}.withDefaults()
	base := captureArtifactBaseline(cfg)
	base.worktree = captureWorktreeEvidenceBaseline(context.Background(), cfg, deps)
	detector, ok := newCompletionDetector(cfg.Completion, cfg, deps, tmuxLaunch{}, base).(*worktreeEvidenceDetector)
	if !ok || !base.worktree.captured {
		t.Fatalf("the worktree-evidence contract must build its detector over a captured snapshot (ok=%v)", ok)
	}
	return detector
}

func completesOnIdle(t *testing.T, d *worktreeEvidenceDetector) bool {
	t.Helper()
	ready, _, _, err := d.completeOnIdle(context.Background())
	if err != nil {
		t.Fatalf("completeOnIdle: %v", err)
	}
	return ready
}

func TestWorktreeEvidenceDetector_ARewriteStillSettlingAtIdleIsLeftToTheStabilityWindow(t *testing.T) {
	d, cfg := evidenceDetector(t)
	writeFile(t, filepath.Join(cfg.Worktree, "docs", "explain.md"), "# Explanation\n- what and why\n")
	writeArtifact(t, cfg.Artifact, staleBuildReport+"## Correction 1\n", fixedMTime.Add(time.Hour))
	if completesOnIdle(t, d) {
		t.Fatal("a deliverable rewritten since dispatch is not carried: completing here would skip its stability window")
	}
	for tick := 1; tick <= artifactStableTicks; tick++ {
		ready, _, _, err := d.poll(context.Background())
		if err != nil || ready != (tick == artifactStableTicks) {
			t.Fatalf("tick %d: ready=%v err=%v — the rewrite completes by the ordinary window", tick, ready, err)
		}
	}
}

func TestWorktreeEvidenceDetector_WaitsForTheContractsSecondaryDeliverables(t *testing.T) {
	secondary := filepath.Join(t.TempDir(), "explanation.md")
	d, cfg := evidenceDetector(t, secondary)
	writeFile(t, filepath.Join(cfg.Worktree, "docs", "explain.md"), "# Explanation\n- what and why\n")
	if completesOnIdle(t, d) {
		t.Fatal("a missing secondary deliverable must hold worktree-evidence completion like it holds the artifact window")
	}
	writeFile(t, secondary, "# Explanation\n")
	if !completesOnIdle(t, d) {
		t.Fatal("with every secondary present the evidence completes")
	}
}

func TestWorktreeEvidenceDetector_AnIdleSnapshotGitCannotTakeIsNoEvidence(t *testing.T) {
	d, cfg := evidenceDetector(t)
	if err := os.RemoveAll(filepath.Join(cfg.Worktree, ".git")); err != nil {
		t.Fatal(err)
	}
	if completesOnIdle(t, d) {
		t.Fatal("a failed idle snapshot must not read as every dispatch-time path having changed")
	}
}

func TestWorktreeEvidenceDetector_AWriteAtTheDeliverablesOwnFallbackIsNotAgentEvidence(t *testing.T) {
	d, cfg := evidenceDetector(t)
	writeFile(t, filepath.Join(cfg.Worktree, filepath.Base(cfg.Artifact)), "# Build Report\nwritten at the cwd fallback\n")
	if completesOnIdle(t, d) {
		t.Fatal("a write at the deliverable's own fallback location counted as worktree evidence")
	}
}

func TestWorktreeEvidenceDetector_AnEditInsideAnUntrackedDirectoryIsAgentEvidence(t *testing.T) {
	cfg := evidenceConfig(t)
	untracked := filepath.Join(cfg.Worktree, "newpkg", "x.go")
	writeFile(t, untracked, "package newpkg\n")
	d := detectorOver(t, cfg)
	writeFile(t, untracked, "package newpkg\n\nvar X = 1\n")
	if !completesOnIdle(t, d) {
		t.Fatal("an edit to a file inside a directory that was already untracked at dispatch must count as worktree evidence")
	}
}
