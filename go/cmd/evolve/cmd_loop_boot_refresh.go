package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

// bootRefreshMarkerFile (under EvolveDir) records the healed-to HEAD of the
// last boot refresh, consumed exactly once by the next boot. A FILE, not an
// env var, deliberately: the flag-ceiling gate forbids new EVOLVE_* readers,
// and darwin resolves duplicate env entries first-wins, making an appended
// env marker invisible to the child.
const bootRefreshMarkerFile = "boot-refresh-marker"

type bootBinaryRefreshResult struct {
	Stale   bool // binary build-commit differs from HEAD with a go/ source delta
	Rebuilt bool // the canonical rebuild succeeded (exec follows unless it errors)
}

// Seams (package vars, mirrors bootRecoverFn) so the refresh logic is
// deterministic under test: git reads, the rebuild, and the process re-exec
// are all injectable.
var (
	bootBinaryRefreshFn       = bootBinaryRefresh
	bootRefreshBinaryCommitFn = version.Commit
	bootRefreshHeadFn         = defaultBootRefreshHead
	bootRefreshSourceDeltaFn  = defaultBootRefreshSourceDelta
	bootRefreshRebuildFn      = defaultBootRefreshRebuild
	bootRefreshExecFn         = defaultBootRefreshExec
	// bootRefreshRepinFn reconciles the ship pin to the rebuilt binary through
	// the same provenance-gated primitive the boot heal uses.
	bootRefreshRepinFn = attemptBootRepin
	// bootRefreshExecTargetFn reports whether the running executable is the
	// plane binary the rebuild writes (go/bin/evolve).
	bootRefreshExecTargetFn = defaultBootRefreshExecTarget
	// bootRefreshFleetLaneFn reports whether another fleet lane is
	// concurrently active, since the plane binary is shared by every lane.
	bootRefreshFleetLaneFn = defaultBootRefreshFleetLane
)

// defaultBootRefreshFleetLane reports a concurrently active fleet lane from
// the shared per-run .lease heartbeat (internal/runlease). An unreadable or
// unparsable lease is an error, not "no lane active".
func defaultBootRefreshFleetLane(cfg loopConfig) (bool, error) {
	runsDir := filepath.Join(cfg.EvolveDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // no runs ever recorded — nothing can be live
		}
		return false, err
	}
	now := time.Now()
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		l, ok, rerr := runlease.Read(filepath.Join(runsDir, e.Name()))
		if rerr != nil {
			return false, rerr
		}
		if ok && runlease.Fresh(l, now, runlease.DefaultTTL) {
			return true, nil
		}
	}
	return false, nil
}

func defaultBootRefreshExecTarget(projectRoot string) (bool, string, error) {
	self, err := os.Executable()
	if err != nil {
		return false, "", err
	}
	selfReal, err := filepath.EvalSymlinks(self)
	if err != nil {
		return false, self, err
	}
	planeBin, err := filepath.EvalSymlinks(filepath.Join(projectRoot, "go", "bin", "evolve"))
	if err != nil {
		return false, selfReal, err
	}
	return selfReal == planeBin, selfReal, nil
}

func defaultBootRefreshHead(projectRoot string) (string, error) {
	out, err := sysexec.Command(context.Background(), "git", "-C", projectRoot, "rev-parse", "HEAD").Output()
	return strings.TrimSpace(string(out)), err
}

// defaultBootRefreshSourceDelta reports whether binaryCommit..head touches
// go/, the only tree the binary embeds.
func defaultBootRefreshSourceDelta(projectRoot, from, to string) (bool, error) {
	out, err := sysexec.Command(context.Background(), "git", "-C", projectRoot, "diff", "--name-only", from+".."+to, "--", "go/").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// defaultBootRefreshRebuild runs the canonical build target so the ldflags
// stamp (version/commit/builtAt) is owned by exactly one place, the Makefile.
func defaultBootRefreshRebuild(projectRoot string, stderr io.Writer) error {
	cmd := sysexec.Command(context.Background(), "make", "-C", "go", "build")
	cmd.Dir = projectRoot
	cmd.Stdout = stderr // build chatter is diagnostics, not loop stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// defaultBootRefreshExec replaces the current process with the freshly built
// binary at the same executable path, same argv and environment (the
// loop-prevention marker travels as a consume-once FILE, not env). On
// success it never returns.
func defaultBootRefreshExec() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(self, os.Args, os.Environ())
}

// shipPinPresent reports whether state.json carries an expected_ship_sha pin.
// An unreadable state file is treated as PRESENT (conservative: the guarded
// repin path runs and its decline blocks the exec — never exec into a state
// we could not judge).
func shipPinPresent(cfg loopConfig) bool {
	b, err := os.ReadFile(filepath.Join(cfg.EvolveDir, "state.json"))
	if err != nil {
		return !os.IsNotExist(err)
	}
	var st struct {
		ExpectedShipSHA string `json:"expected_ship_sha"`
	}
	if json.Unmarshal(b, &st) != nil {
		return true
	}
	return st.ExpectedShipSHA != ""
}

func bootBinaryRefresh(cfg loopConfig, stderr io.Writer) bootBinaryRefreshResult {
	var res bootBinaryRefreshResult
	projectRoot := cfg.ProjectRoot

	// A policy load error resolves to the compiled default ("auto") — the
	// self-heal is integrity posture, so a malformed policy must not disable it.
	if pol, _ := policy.Load(filepath.Join(cfg.EvolveDir, "policy.json")); pol.BootBinaryRefresh() == "off" {
		fmt.Fprintf(stderr, "[loop] boot-refresh: policy boot.binary_refresh=off — staleness self-heal disabled by operator\n")
		return res
	}

	binCommit := bootRefreshBinaryCommitFn()
	if binCommit == "" {
		fmt.Fprintf(stderr, "[loop] boot-refresh: binary build-commit is empty — staleness unverifiable, booting as-is\n")
		return res
	}
	head, err := bootRefreshHeadFn(projectRoot)
	if err != nil || head == "" {
		fmt.Fprintf(stderr, "[loop] boot-refresh: cannot resolve HEAD (%v) — skipping staleness check\n", err)
		return res
	}
	// Dev builds stamp a short (12-hex) commit; HEAD is full-width. Prefix
	// equality in either direction means the binary matches the checkout.
	if strings.HasPrefix(head, binCommit) || strings.HasPrefix(binCommit, head) {
		return res
	}

	delta, err := bootRefreshSourceDeltaFn(projectRoot, binCommit, head)
	if err != nil {
		// Unknown/absent commit (stripped stamp, rewritten history): loudly
		// unverifiable, never a boot blocker.
		fmt.Fprintf(stderr, "[loop] boot-refresh: WARN delta %s..%s unverifiable (%v) — booting as-is\n", binCommit, head[:12], err)
		return res
	}
	if !delta {
		fmt.Fprintf(stderr, "[loop] boot-refresh: binary %s behind HEAD %s but no go/ source delta — refresh unnecessary\n", binCommit, head[:12])
		return res
	}
	res.Stale = true

	// Checked before the marker is consumed, so a skipped boot leaves the
	// next one's staleness judgment untouched.
	laneActive, lerr := bootRefreshFleetLaneFn(cfg)
	if lerr != nil {
		fmt.Fprintf(stderr, "[loop] boot-refresh: WARN fleet-lane check unverifiable (%v) — cannot prove the plane is idle, so refusing to rebuild the shared binary; booting as-is\n", lerr)
		return res
	}
	if laneActive {
		fmt.Fprintf(stderr, "[loop] boot-refresh: a concurrent fleet lane is active (fresh run lease) — refusing to rebuild the plane binary mid-batch; the heal lands at the next idle boot\n")
		return res
	}

	markerPath := filepath.Join(cfg.EvolveDir, bootRefreshMarkerFile)
	priorB, _ := os.ReadFile(markerPath)
	_ = os.Remove(markerPath)
	if prior := strings.TrimSpace(string(priorB)); prior == head {
		fmt.Fprintf(stderr, "[loop] boot-refresh: WARN still stale after refresh (binary %s, HEAD %s) — rebuild did not change the stamp; refusing a second re-exec and booting as-is\n", binCommit, head[:12])
		return res
	}

	ok, execPath, terr := bootRefreshExecTargetFn(projectRoot)
	if terr != nil || !ok {
		fmt.Fprintf(stderr, "[loop] boot-refresh: WARN running from non-plane executable %q (err=%v) — self-heal only applies to plane launches (go/bin/evolve); booting as-is\n", execPath, terr)
		return res
	}

	fmt.Fprintf(stderr, "[loop] boot-refresh: binary %s is behind HEAD %s with a go/ delta — rebuilding and re-exec'ing (fixes landed by the loop become live NOW, not at the next manual refresh)\n", binCommit, head[:12])
	if err := bootRefreshRebuildFn(projectRoot, stderr); err != nil {
		fmt.Fprintf(stderr, "[loop] boot-refresh: WARN rebuild failed (%v) — booting the old binary\n", err)
		return res
	}
	res.Rebuilt = true

	if shipPinPresent(cfg) {
		if !bootRefreshRepinFn(cfg, stderr) {
			fmt.Fprintf(stderr, "[loop] boot-refresh: NOT exec'ing — the pin above stayed unreconciled, so the child would halt as tampered\n")
			return res
		}
	}

	tmp := markerPath + ".tmp"
	if werr := os.WriteFile(tmp, []byte(head+"\n"), 0o644); werr == nil {
		_ = os.Rename(tmp, markerPath)
	}
	if err := bootRefreshExecFn(); err != nil {
		_ = os.Remove(markerPath)
		fmt.Fprintf(stderr, "[loop] boot-refresh: WARN re-exec failed (%v) — booting the old binary (rebuilt copy is on disk and pin-reconciled for the next launch)\n", err)
	}
	return res
}
