package main

// cmd_loop_boot_refresh_test.go — the binary-lag class (2026-08-05 retro):
// fixes land on main but the loop keeps executing a frozen binary until an
// operator manually rebuilds. Measured cost overnight: the sentinel tail-anchor
// fix landed at cycle-1301 while cycles 1302–1309 ran the OLD parser — three
// wasted lane-cycles and the cycle-1309 identical-fingerprint batch HALT, on a
// defect already fixed in the repo. These tests pin the boot-time self-heal:
// detect staleness → rebuild → re-exec, every step fail-open (a refresh
// failure WARNs and boots the old binary — never bricks the loop).

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"crypto/sha256"
	"encoding/hex"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// swapRefreshSeams installs fakes for every bootBinaryRefresh seam and
// restores them on cleanup. Each fake records its invocation.
type refreshSpy struct {
	headCalls, deltaCalls, rebuildCalls, execCalls int
	orderLog                                       []string
}

func swapRefreshSeams(t *testing.T, head string, headErr error, delta bool, deltaErr error, rebuildErr error, execErr error) *refreshSpy {
	t.Helper()
	spy := &refreshSpy{}
	prevHead, prevDelta, prevRebuild, prevExec := bootRefreshHeadFn, bootRefreshSourceDeltaFn, bootRefreshRebuildFn, bootRefreshExecFn
	t.Cleanup(func() {
		bootRefreshHeadFn, bootRefreshSourceDeltaFn, bootRefreshRebuildFn, bootRefreshExecFn = prevHead, prevDelta, prevRebuild, prevExec
	})
	bootRefreshHeadFn = func(projectRoot string) (string, error) {
		spy.headCalls++
		return head, headErr
	}
	bootRefreshSourceDeltaFn = func(projectRoot, from, to string) (bool, error) {
		spy.deltaCalls++
		return delta, deltaErr
	}
	bootRefreshRebuildFn = func(projectRoot string, stderr io.Writer) error {
		spy.rebuildCalls++
		spy.orderLog = append(spy.orderLog, "rebuild")
		return rebuildErr
	}
	bootRefreshExecFn = func() error {
		spy.execCalls++
		spy.orderLog = append(spy.orderLog, "exec")
		return execErr
	}
	return spy
}

func swapBinaryCommit(t *testing.T, commit string) {
	t.Helper()
	prev := bootRefreshBinaryCommitFn
	t.Cleanup(func() { bootRefreshBinaryCommitFn = prev })
	bootRefreshBinaryCommitFn = func() string { return commit }
}

func swapRepinSuccess(t *testing.T) {
	t.Helper()
	prev := bootRefreshRepinFn
	t.Cleanup(func() { bootRefreshRepinFn = prev })
	bootRefreshRepinFn = func(cfg loopConfig, stderr io.Writer) bool { return true }
}

func swapRepinReal(t *testing.T) {
	t.Helper()
	prev := bootRefreshRepinFn
	t.Cleanup(func() { bootRefreshRepinFn = prev })
	bootRefreshRepinFn = attemptBootRepin
}

func swapExecTargetOK(t *testing.T) {
	t.Helper()
	prev := bootRefreshExecTargetFn
	t.Cleanup(func() { bootRefreshExecTargetFn = prev })
	bootRefreshExecTargetFn = func(projectRoot string) (bool, string, error) { return true, "go/bin/evolve", nil }
}

func writeMarker(t *testing.T, evolveDir, head string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(evolveDir, bootRefreshMarkerFile), []byte(head+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readMarker(t *testing.T, evolveDir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(evolveDir, bootRefreshMarkerFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func sha256Hex(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestBootBinaryRefresh_UpToDateIsNoop(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "abc123def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	var errBuf bytes.Buffer
	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: t.TempDir()}, &errBuf)
	if res.Stale || spy.rebuildCalls != 0 || spy.execCalls != 0 {
		t.Fatalf("up-to-date binary must be a no-op: %+v spy=%+v", res, spy)
	}
}

func TestBootBinaryRefresh_StaleWithGoDeltaRebuildsThenExecs(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	swapRepinSuccess(t)
	swapExecTargetOK(t)
	evolveDir := t.TempDir()
	var errBuf bytes.Buffer
	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: evolveDir}, &errBuf)
	if !res.Stale || !res.Rebuilt {
		t.Fatalf("stale+go-delta must rebuild: %+v", res)
	}
	if spy.rebuildCalls != 1 || spy.execCalls != 1 {
		t.Fatalf("want exactly one rebuild then one exec, got %+v", spy)
	}
	if strings.Join(spy.orderLog, ",") != "rebuild,exec" {
		t.Fatalf("exec must follow rebuild, got order %v", spy.orderLog)
	}
	if got := readMarker(t, evolveDir); got != "eeee23def456fffffffffffffffffffffffffff0" {
		t.Fatalf("marker file must carry the healed-to HEAD before exec (refresh-loop prevention), got %q", got)
	}
}

func TestBootBinaryRefresh_StaleDocsOnlyDeltaSkipsRebuild(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, false, nil, nil, nil)
	var errBuf bytes.Buffer
	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: t.TempDir()}, &errBuf)
	if res.Rebuilt || spy.rebuildCalls != 0 {
		t.Fatalf("docs-only delta must not rebuild: %+v spy=%+v", res, spy)
	}
	if !strings.Contains(errBuf.String(), "no go/ source delta") {
		t.Fatalf("docs-only skip must be announced, got: %s", errBuf.String())
	}
}

func TestBootBinaryRefresh_EmptyCommitIsUnverifiableSkip(t *testing.T) {
	swapBinaryCommit(t, "")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	var errBuf bytes.Buffer
	_ = bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: t.TempDir()}, &errBuf)
	if spy.rebuildCalls != 0 || spy.execCalls != 0 {
		t.Fatalf("empty build-commit is unverifiable — must skip, spy=%+v", spy)
	}
	if !strings.Contains(errBuf.String(), "unverifiable") {
		t.Fatalf("unverifiable skip must WARN, got: %s", errBuf.String())
	}
}

func TestBootBinaryRefresh_SecondRefreshAttemptAvertsLoop(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	// Marker carries the healed-to HEAD (value semantics): refusing is correct
	// ONLY when we are still stale at the SAME target — a no-op rebuild.
	evolveDir := t.TempDir()
	writeMarker(t, evolveDir, "eeee23def456fffffffffffffffffffffffffff0")
	var errBuf bytes.Buffer
	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: evolveDir}, &errBuf)
	if spy.rebuildCalls != 0 || spy.execCalls != 0 || res.Rebuilt {
		t.Fatalf("marker set + still stale must NOT re-exec again (refresh loop), spy=%+v", spy)
	}
	if !strings.Contains(errBuf.String(), "still stale after refresh") {
		t.Fatalf("averted loop must WARN loudly, got: %s", errBuf.String())
	}
}

func TestBootBinaryRefresh_RebuildFailureFailsOpen(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, os.ErrPermission, nil)
	swapRepinSuccess(t)
	swapExecTargetOK(t)
	var errBuf bytes.Buffer
	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: t.TempDir()}, &errBuf)
	if spy.execCalls != 0 {
		t.Fatal("exec must not run after a failed rebuild")
	}
	if res.Rebuilt {
		t.Fatalf("failed rebuild must not report Rebuilt: %+v", res)
	}
	if !strings.Contains(errBuf.String(), "WARN") {
		t.Fatalf("rebuild failure must WARN and boot the old binary, got: %s", errBuf.String())
	}
}

func TestBootBinaryRefresh_ExecFailureFailsOpen(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, os.ErrPermission)
	swapRepinSuccess(t)
	swapExecTargetOK(t)
	var errBuf bytes.Buffer
	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: t.TempDir()}, &errBuf)
	if !res.Rebuilt {
		t.Fatalf("rebuild succeeded — result must say so even when exec fails: %+v", res)
	}
	if !strings.Contains(errBuf.String(), "WARN") {
		t.Fatalf("exec failure must WARN and continue on the old binary, got: %s", errBuf.String())
	}
}

// TestRunLoop_BootRefreshRunsBeforeRecovery is the WIRING PROOF through the
// LIVE runLoop path (mirrors TestRunLoop_InvokesBootRecoveryBeforeGate): the
// refresh seam must fire on the fresh-boot path, BEFORE boot recovery — a
// stale binary should not run recovery logic either. bootRecoverFn is faked to
// HaltSelfSHA so runLoop returns immediately after the two seams — proving
// order without running a batch.
func TestRunLoop_BootRefreshRunsBeforeRecovery(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var order []string
	prevRefresh, prevRecover := bootBinaryRefreshFn, bootRecoverFn
	t.Cleanup(func() { bootBinaryRefreshFn, bootRecoverFn = prevRefresh, prevRecover })
	bootBinaryRefreshFn = func(cfg loopConfig, stderr io.Writer) bootBinaryRefreshResult {
		order = append(order, "refresh")
		if cfg.ProjectRoot != projectRoot {
			t.Errorf("refresh must receive the loop's project root; got %q want %q", cfg.ProjectRoot, projectRoot)
		}
		return bootBinaryRefreshResult{}
	}
	bootRecoverFn = func(_ context.Context, _ loopConfig, _ core.Ledger, _ io.Writer) bootRecoveryResult {
		order = append(order, "recover")
		return bootRecoveryResult{HaltSelfSHA: true}
	}

	prevDeps := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = prevDeps })
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		return orchDeps{Storage: &fixtures.FakeStorage{}, Ledger: newFakeLedger()}
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "anything",
		"--cycles", "1",
		"--force-fresh",
	}, nil, &stdout, &stderr)

	if rc != 2 {
		t.Fatalf("HaltSelfSHA short-circuit expected rc=2, got %d; stderr=%q", rc, stderr.String())
	}
	if strings.Join(order, ",") != "refresh,recover" {
		t.Fatalf("boot order must be refresh THEN recover, got %v", order)
	}
}

func TestBootBinaryRefresh_PolicyOffDisables(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"boot":{"binary_refresh":"off"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var errBuf bytes.Buffer
	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: evolveDir}, &errBuf)
	if spy.rebuildCalls != 0 || spy.execCalls != 0 || res.Rebuilt {
		t.Fatalf("boot.binary_refresh=off must disable the self-heal entirely, spy=%+v res=%+v", spy, res)
	}
	if !strings.Contains(errBuf.String(), "binary_refresh=off") {
		t.Fatalf("policy-off must be announced so a stale boot is attributable, got: %s", errBuf.String())
	}
}

// TestBootRefreshDefaultAdapters_RealGit pins the thin real-git adapters the
// seam tests fake: HEAD resolution, short-SHA range acceptance, and the go/
// pathspec discrimination — against a real fixture repo (the reviewer attack
// surface: does `git diff short..full -- go/` behave as assumed?).
func TestBootRefreshDefaultAdapters_RealGit(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	mustWrite := func(rel, content string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("go/a.go", "package a")
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	baseFull, err := defaultBootRefreshHead(root)
	if err != nil || len(baseFull) != 40 {
		t.Fatalf("head: %q %v", baseFull, err)
	}
	baseShort := baseFull[:12] // the linker stamps 12-hex

	mustWrite("docs/x.md", "docs only")
	run("add", "-A")
	run("commit", "-q", "-m", "docs")
	head1, _ := defaultBootRefreshHead(root)
	delta, err := defaultBootRefreshSourceDelta(root, baseShort, head1)
	if err != nil {
		t.Fatalf("short-SHA range must be accepted: %v", err)
	}
	if delta {
		t.Fatal("docs-only commit must report NO go/ delta")
	}

	mustWrite("go/b.go", "package a")
	run("add", "-A")
	run("commit", "-q", "-m", "code")
	head2, _ := defaultBootRefreshHead(root)
	delta, err = defaultBootRefreshSourceDelta(root, baseShort, head2)
	if err != nil || !delta {
		t.Fatalf("go/-touching range must report a delta: %v %v", delta, err)
	}

	if _, err := defaultBootRefreshSourceDelta(root, "feedfacefeed", head2); err == nil {
		t.Fatal("unknown from-commit must error (feeds the fail-open WARN path)")
	}
}

func TestBootBinaryRefresh_NewStalenessAfterPriorHealReheals(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	swapRepinSuccess(t)
	swapExecTargetOK(t)
	// Prior heal targeted a DIFFERENT head — this is legitimate new staleness,
	// not a refresh loop (the reviewer's chain-mode finding). File marker is
	// consume-once and REPLACED with the new target (the env-var design was
	// retired: darwin resolves duplicate env first-wins, and the flag-ceiling
	// gate forbids new EVOLVE_* readers).
	evolveDir := t.TempDir()
	writeMarker(t, evolveDir, "0ldhead0000000000000000000000000000000000")
	var errBuf bytes.Buffer
	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: evolveDir}, &errBuf)
	if spy.rebuildCalls != 1 || spy.execCalls != 1 || !res.Rebuilt {
		t.Fatalf("new staleness after a prior heal must re-heal, spy=%+v res=%+v", spy, res)
	}
	if got := readMarker(t, evolveDir); got != "eeee23def456fffffffffffffffffffffffffff0" {
		t.Fatalf("marker must be REPLACED with the NEW healed-to head, got %q", got)
	}
}

// Exec failure must consume the marker: the old binary continues this boot,
// and the NEXT launch should judge staleness fresh, not inherit this
// attempt's target.
func TestBootBinaryRefresh_ExecFailureRemovesMarker(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, os.ErrPermission)
	swapRepinSuccess(t)
	swapExecTargetOK(t)
	evolveDir := t.TempDir()
	var errBuf bytes.Buffer
	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: evolveDir}, &errBuf)
	if !res.Rebuilt {
		t.Fatalf("rebuild succeeded: %+v", res)
	}
	if got := readMarker(t, evolveDir); got != "" {
		t.Fatalf("failed exec must remove the marker, found %q", got)
	}
}

// N2: a PINLESS plane (no expected_ship_sha in state.json) has nothing to
// reconcile — the heal must proceed to exec without invoking the repin, and
// the child boots cleanly (no pin -> no mismatch -> no halt).
func TestBootBinaryRefresh_PinlessPlaneExecsWithoutRepin(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	repinCalls := 0
	prevRepin := bootRefreshRepinFn
	t.Cleanup(func() { bootRefreshRepinFn = prevRepin })
	bootRefreshRepinFn = func(cfg loopConfig, stderr io.Writer) bool { repinCalls++; return false }
	swapExecTargetOK(t)
	evolveDir := t.TempDir() // no state.json at all
	var errBuf bytes.Buffer
	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: evolveDir}, &errBuf)
	if repinCalls != 0 {
		t.Fatal("pinless plane must not invoke the repin (nothing to reconcile)")
	}
	if !res.Rebuilt || spy.execCalls != 1 {
		t.Fatalf("pinless heal must complete rebuild+exec: %+v spy=%+v stderr=%s", res, spy, errBuf.String())
	}
}

// F1 (the reviewer's CRITICAL): a rebuild changes the on-disk binary hash; the
// within-version SELF_SHA classifier reads an unreconciled pin as TAMPERING
// and halts pre-scout. The refresh must reconcile the pin through the SAME
// provenance-gated primitive the boot heal uses (attemptBootRepin ->
// phaseintegrity.RepinIfDrifted) after the rebuild and BEFORE exec.
func TestBootBinaryRefresh_ReconcilesShipPinViaRealPrimitive(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	binDir := filepath.Join(projectRoot, "go", "bin")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(binDir, "evolve")
	if err := os.WriteFile(binPath, []byte("OLD BINARY"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldSum := sha256Hex(t, binPath)
	statePath := filepath.Join(evolveDir, "state.json")
	if err := os.WriteFile(statePath, []byte(`{"expected_ship_sha":"`+oldSum+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	// The fake rebuild SIMULATES the hash change the real one causes.
	bootRefreshRebuildFn = func(root string, stderr io.Writer) error {
		spy.rebuildCalls++
		spy.orderLog = append(spy.orderLog, "rebuild")
		return os.WriteFile(binPath, []byte("NEW BINARY CONTENT"), 0o755)
	}
	// Provenance seam: the running stamp is HEAD-ancestral (true on a lagging
	// plane by construction). The REAL RepinIfDrifted runs underneath.
	prevProv := shipRepinProvenanceFn
	t.Cleanup(func() { shipRepinProvenanceFn = prevProv })
	shipRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "abc123def456", func(string) bool { return true }
	}
	swapRepinReal(t)
	swapExecTargetOK(t)

	var errBuf bytes.Buffer
	res := bootBinaryRefresh(loopConfig{ProjectRoot: projectRoot, EvolveDir: evolveDir}, &errBuf)
	if !res.Rebuilt || spy.execCalls != 1 {
		t.Fatalf("heal must complete: %+v spy=%+v stderr=%s", res, spy, errBuf.String())
	}
	newSum := sha256Hex(t, binPath)
	b, _ := os.ReadFile(statePath)
	if !strings.Contains(string(b), newSum) {
		t.Fatalf("expected_ship_sha must be reconciled to the REBUILT binary before exec (else the child's within-version classifier HALTs as tampered)\nstate=%s want-sha=%s", b, newSum)
	}
	if strings.Join(spy.orderLog, ",") != "rebuild,exec" {
		t.Fatalf("order must be rebuild,repin,exec (repin between), got %v", spy.orderLog)
	}
}

// F1 decline path: unverifiable provenance must NOT exec (the child would boot
// into the tamper halt); it WARNs with the operator recipe and keeps the old
// binary running — the pin now legitimately flags the foreign on-disk file.
func TestBootBinaryRefresh_RepinDeclineSkipsExec(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	binDir := filepath.Join(projectRoot, "go", "bin")
	os.MkdirAll(evolveDir, 0o755)
	os.MkdirAll(binDir, 0o755)
	binPath := filepath.Join(binDir, "evolve")
	os.WriteFile(binPath, []byte("OLD"), 0o755)
	os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(`{"expected_ship_sha":"`+sha256Hex(t, binPath)+`"}`), 0o644)

	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	bootRefreshRebuildFn = func(root string, stderr io.Writer) error {
		spy.rebuildCalls++
		return os.WriteFile(binPath, []byte("NEW"), 0o755)
	}
	prevProv := shipRepinProvenanceFn
	t.Cleanup(func() { shipRepinProvenanceFn = prevProv })
	shipRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "abc123def456", func(string) bool { return false } // NOT ancestral
	}
	swapRepinReal(t)
	swapExecTargetOK(t)

	var errBuf bytes.Buffer
	_ = bootBinaryRefresh(loopConfig{ProjectRoot: projectRoot, EvolveDir: evolveDir}, &errBuf)
	if spy.execCalls != 0 {
		t.Fatal("declined repin must NOT exec — the child would halt as tampered")
	}
	if !strings.Contains(errBuf.String(), "reset-sha") {
		t.Fatalf("decline must print the operator recipe, got: %s", errBuf.String())
	}
}

// F5: a loop launched from a non-plane binary (installed copy) must refuse the
// self-heal BEFORE rebuilding — rebuilding the plane copy while exec'ing the
// old path is a silent no-op heal plus a mined pin.
func TestBootBinaryRefresh_NonPlaneExecutableRefusesHeal(t *testing.T) {
	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	prevTarget := bootRefreshExecTargetFn
	t.Cleanup(func() { bootRefreshExecTargetFn = prevTarget })
	bootRefreshExecTargetFn = func(projectRoot string) (bool, string, error) {
		return false, "/usr/local/bin/evolve", nil
	}
	var errBuf bytes.Buffer
	res := bootBinaryRefresh(loopConfig{ProjectRoot: "/proj", EvolveDir: t.TempDir()}, &errBuf)
	if spy.rebuildCalls != 0 || res.Rebuilt {
		t.Fatalf("non-plane launch must refuse BEFORE rebuild, spy=%+v", spy)
	}
	if !strings.Contains(errBuf.String(), "non-plane") {
		t.Fatalf("refusal must be announced, got: %s", errBuf.String())
	}
}
