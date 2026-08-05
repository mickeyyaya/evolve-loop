package main

// cmd_loop_boot_refresh.go — boot-time binary staleness self-heal (the
// binary-lag class, 2026-08-05 retro: docs/chronicle/2026-08-binary-lag.md).
//
// The loop ships fixes to main but keeps executing the binary it booted with;
// until an operator manually rebuilds, every landed pipeline fix is inert.
// Measured cost in one night: the sentinel tail-anchor fix landed at
// cycle-1301 while cycles 1302–1309 ran the old parser — three wasted lane
// cycles and the cycle-1309 identical-fingerprint batch HALT on a defect the
// repo had already fixed. This is deterministic operator toil, which belongs
// in code.
//
// At boot, BEFORE recovery and the readiness gate: if the running binary's
// embedded build commit is behind the plane HEAD by a delta that touches go/,
// rebuild via the canonical make target and re-exec the fresh binary in
// place. The re-exec'd process's existing boot machinery (auto-repin,
// recovery, preflight) then runs on the new binary. EVERY step is fail-open:
// any failure WARNs and boots the old binary — a stale batch is yesterday's
// status quo, a bricked loop is worse. A consume-once marker FILE
// (.evolve/boot-refresh-marker, value = healed-to HEAD) caps the self-heal
// at one attempt per target so a rebuild that does not change the stamp can
// never re-exec forever.
//
// DOCUMENTED INTENT (adversarial review 2026-08-05, findings 3/4/6):
//   - --resume never refreshes (the resume branch returns before this call):
//     resume is the minimal-perturbation single-cycle protocol — swapping the
//     executor mid-cycle is a bigger risk than one more stale cycle. The heal
//     lands at the next fresh boot.
//   - The refresh runs after runLoop's early boot side effects (plane
//     classification, socket GC, carryover auto-prune), so a healed boot
//     repeats them once in the child. The only non-idempotent one is the
//     carryover cycles_unpicked bump (×2 on heal boots) — accepted: heal
//     boots are rare by construction and the counter self-corrects at the
//     next pick; moving the call earlier would put it before the resume
//     branch and violate the resume exclusion above.
//   - Concurrent double-launch can race two rebuilds of go/bin/evolve
//     (non-atomic tool copy) — accepted: simultaneous loop launches are
//     already excluded operationally (lease at cycle level; single-operator
//     plane), and a torn binary fails ENOEXEC into the fail-open path.
//   - A repin-success + exec-failure boot runs with intra-batch skew: the old
//     loop image continues while subprocesses exec'ing go/bin/evolve get the
//     new code. Traced safe at ship time (verifySelfSHA hashes the file, not
//     the image); bounded to rare exec-failure boots.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

// bootRefreshMarkerFile (under EvolveDir) records the healed-to HEAD of the
// last boot refresh, consumed exactly once by the next boot. A FILE, not an
// env var, deliberately: the flag-ceiling gate forbids new EVOLVE_* readers
// (target: zero), and darwin resolves duplicate env entries first-wins —
// making an appended env marker invisible to the child (adversarial-review
// N1, empirically verified). File semantics have neither hazard.
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
	// bootRefreshRepinFn reconciles state.json:expected_ship_sha to the
	// REBUILT binary through the SAME provenance-gated primitive the
	// across-version boot heal uses (attemptBootRepin ->
	// phaseintegrity.RepinIfDrifted). Without this the re-exec'd child's
	// within-version SELF_SHA classifier reads the fresh hash as TAMPERING
	// and halts pre-scout — the adversarial review's CRITICAL finding.
	bootRefreshRepinFn = attemptBootRepin
	// bootRefreshExecTargetFn reports whether the running executable IS the
	// plane binary the rebuild writes (go/bin/evolve). A loop launched from
	// an installed copy must refuse the self-heal BEFORE rebuilding: it would
	// rebuild one file and re-exec another — a silent no-op heal on a mined
	// pin (review finding 5).
	bootRefreshExecTargetFn = defaultBootRefreshExecTarget
)

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
	out, err := exec.Command("git", "-C", projectRoot, "rev-parse", "HEAD").Output()
	return strings.TrimSpace(string(out)), err
}

// defaultBootRefreshSourceDelta reports whether binaryCommit..head touches
// go/ — the only tree the binary embeds (skills/, agents/, .evolve/ are read
// from disk at runtime, so a docs-or-config-only delta never warrants a
// rebuild).
func defaultBootRefreshSourceDelta(projectRoot, from, to string) (bool, error) {
	out, err := exec.Command("git", "-C", projectRoot, "diff", "--name-only", from+".."+to, "--", "go/").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// defaultBootRefreshRebuild runs the canonical build target so the ldflags
// stamp (version/commit/builtAt) is owned by exactly one place, the Makefile.
func defaultBootRefreshRebuild(projectRoot string, stderr io.Writer) error {
	cmd := exec.Command("make", "-C", "go", "build")
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

	// Operator dial (.evolve/policy.json boot.binary_refresh): "off" pins the
	// current binary deliberately (incident bisects). A load error resolves to
	// the compiled default ("auto") — the self-heal is integrity posture.
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

	// Marker carries the healed-to HEAD (VALUE semantics, review finding 2):
	// refuse only when the prior heal targeted this SAME head — a rebuild
	// that did not change the stamp. New staleness (different head)
	// legitimately re-heals. Consume-once: read + remove, so a stale marker
	// can never outlive the boot that observed it.
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

	// Reconcile the ship pin to the rebuilt binary BEFORE exec (review
	// finding 1, CRITICAL): provenance-gated via the shared primitive; a
	// decline means the running stamp is not HEAD-ancestral — a foreign
	// binary — so the child would boot into the tamper halt. Keep the old
	// binary running and leave the pin flagging the on-disk file.
	// Tri-state (re-review N2): a PINLESS plane has nothing to reconcile and
	// the child boots cleanly without one — only a present-and-unreconciled
	// pin blocks the exec.
	if shipPinPresent(cfg) {
		if !bootRefreshRepinFn(cfg, stderr) {
			fmt.Fprintf(stderr, "[loop] boot-refresh: NOT exec'ing — the pin above stayed unreconciled, so the child would halt as tampered\n")
			return res
		}
	}

	// Write the marker (atomic per repo convention) BEFORE exec; if exec
	// fails, remove it — the old binary continues and the next launch should
	// judge staleness fresh, not inherit this attempt's target.
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
