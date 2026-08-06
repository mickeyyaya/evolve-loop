// cmd_loop_chain.go — the outer batch-chaining loop (cycle 1075, inbox item
// loop-batch-chaining; standing operator directive of 2026-07-11: lanes keep
// running until the inbox is empty without an operator relaunching
// `evolve loop` at every batch boundary).
//
// Design: runLoopBatch stays the single-batch dispatcher it has always been.
// This file adds a thin loop AROUND it that, at each boundary, decides whether
// another batch may start. It deliberately owns no cycle-level logic — the
// quota wall in particular is NOT re-derived here: the batch already maps
// core.ErrAllFamiliesExhausted (core.allFamiliesQuotaExhausted, the all-85
// attempt sequence) onto the resumable rc=5 QUOTA-PAUSE contract, and the
// chain simply refuses to relaunch into it and defers with the checkpoint's
// reset-time hint.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

// chainBrakeFile is the operator brake: `touch .evolve/loop-stop` and the
// chain stops at the next boundary (the in-flight batch is never interrupted —
// use SIGINT for that, which the batch already checkpoints).
const chainBrakeFile = "loop-stop"

// runLoopBatchFn is the test seam for the chain loop: tests substitute a
// scripted batch so the boundary decisions can be exercised without running
// real cycles. nil-free by construction — production is the real batch.
var runLoopBatchFn = runLoopBatch

// chainBatchRecord is one chained batch's boundary observation. FleetCount is
// recorded per batch because fleet width is a hard operator commitment
// (ten_lane_concurrency_standing): a chain that silently narrows lanes from
// batch N to N+1 looks healthy in every other signal.
type chainBatchRecord struct {
	Batch        int `json:"batch"`
	RC           int `json:"rc"`
	FleetCount   int `json:"fleet_count"`
	InboxPending int `json:"inbox_pending"`
}

// chainResult is the machine-readable chain summary, emitted to stdout after
// the per-batch loopResult documents.
type chainResult struct {
	ChainMode  bool               `json:"chain_mode"`
	MaxBatches int                `json:"max_batches"`
	Batches    []chainBatchRecord `json:"batches"`
	StopReason string             `json:"chain_stop_reason"`
	// BoundaryRefresh surfaces the LAST boundary-refresh-log.jsonl entry when
	// this chain's stop is "chain_boundary_refresh_reexec" — nil-when-clean
	// (spineFailOpenRollup shape), omitted on every ordinary stop so a
	// dossier/summary consumer does not need a separate grep of the JSONL
	// audit trail to see which commits a refresh moved between.
	BoundaryRefresh *chainBoundaryRefreshLogEntry `json:"boundary_refresh,omitempty"`
}

// loadChainConfig loads .evolve/policy.json and returns the resolved chain
// configuration. Absent or malformed policy falls back to built-in defaults
// (chaining off, positive compiled cap), mirroring loadWorkflowConfig.
func loadChainConfig(evolveDir string) policy.ChainConfig {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		return policy.Policy{}.ChainConfig()
	}
	return pol.ChainConfig()
}

// inboxPendingCount counts unclaimed inbox items — the `*.json` files directly
// under .evolve/inbox that actually PARSE as an inbox item. Lifecycle
// subdirectories (processing/, processed/, consumed/, quarantine/, …) and
// non-json files are not pending work and stay invisible. A root-level `*.json`
// that is not an item (truncated, 0-byte, a top-level array, no `id`) is
// returned by NAME in skipped rather than counted: counting it would pin
// pending>0 permanently and burn the chain to max_batches consuming nothing,
// and swallowing it would hide a real item lost to a typo. A MISSING inbox is
// legitimately zero pending; any other read error is returned so the caller
// stops loudly rather than chaining on a guess.
//
// The skip list is RETURNED, not printed, so this stays a pure function and the
// operator diagnostic lives at the call site (runLoopChain).
func inboxPendingCount(evolveDir string) (int, []string, error) {
	dir := filepath.Join(evolveDir, "inbox")
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, nil
		}
		return 0, nil, fmt.Errorf("read inbox: %w", err)
	}
	n := 0
	var skipped []string
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if isInboxItemFile(filepath.Join(dir, e.Name())) {
			n++
			continue
		}
		skipped = append(skipped, e.Name())
	}
	return n, skipped, nil
}

// isInboxItemFile reports whether a root-level `*.json` is a real inbox item: a
// JSON OBJECT carrying a non-empty `id` (the field every inbox consumer keys
// on). Deliberately shallow — the chain only needs "is this pending work", not
// full schema validation, and a stricter check here would silently drop items
// the real consumers accept.
func isInboxItemFile(path string) bool {
	buf, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var doc map[string]any
	if err := json.Unmarshal(buf, &doc); err != nil {
		return false
	}
	id, _ := doc["id"].(string)
	return strings.TrimSpace(id) != ""
}

// chainBrakeEngaged reports whether the operator dropped the `.evolve/loop-stop`
// brake file.
func chainBrakeEngaged(evolveDir string) bool {
	_, err := os.Stat(filepath.Join(evolveDir, chainBrakeFile))
	return err == nil
}

// --- Boundary binary refresh (cycle 1314, inbox item auto-refresh-binary-at-
// boundary): a chained loop can run for hours across many boundaries while
// fixes land on main mid-chain (e.g. the sentinel tail-anchor fix at
// cycle-1301 64f8620e sat inert across cycles 1302-1309 because nothing
// rebuilds the RUNNING binary between boundaries). Every seam below is a
// package var so tests can script every branch without a real rebuild/re-exec.

// chainBoundaryAheadFn is the test seam for the "is my binary stale" check.
var chainBoundaryAheadFn = defaultChainBoundaryAhead

// defaultChainBoundaryAhead reports whether HEAD carries commits beyond
// runningCommit — reusing the exact ancestor-check idiom runResetSHA already
// uses (`git merge-base --is-ancestor <commit> HEAD`), inverted: ahead=true
// means runningCommit is a STRICT ancestor of HEAD (HEAD has moved past it).
// An empty runningCommit (unstamped dev binary) is a quiet no-op. Any
// git/repo failure returns a non-nil error — the caller treats that as "skip
// this boundary's refresh", never halt the chain.
//
// runningCommit is version.Commit(), which the Makefile stamps as a 12-char
// SHORT commit (`git rev-parse --short=12 HEAD`, go/Makefile:16) while `git
// rev-parse HEAD` below returns the FULL 40-char SHA. Comparing the two
// strings directly can never match even when the binary IS current, and
// `git merge-base --is-ancestor` treats equal commits as ancestors too (it
// is not a strict check) — so an up-to-date binary fell through to
// ahead=true. Resolve runningCommit to its full SHA first so the equality
// check actually catches the common "already current" case.
func defaultChainBoundaryAhead(projectRoot, runningCommit string) (ahead bool, err error) {
	if runningCommit == "" {
		return false, nil
	}
	runningOut, err := exec.Command("git", "-C", projectRoot, "rev-parse", runningCommit).Output()
	if err != nil {
		return false, fmt.Errorf("chain boundary ahead-check: resolve running commit %q: %w", runningCommit, err)
	}
	running := strings.TrimSpace(string(runningOut))

	headOut, err := exec.Command("git", "-C", projectRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return false, fmt.Errorf("chain boundary ahead-check: resolve HEAD: %w", err)
	}
	head := strings.TrimSpace(string(headOut))
	if head == running {
		return false, nil
	}
	if err := exec.Command("git", "-C", projectRoot, "merge-base", "--is-ancestor", running, "HEAD").Run(); err != nil {
		return false, fmt.Errorf("chain boundary ahead-check: %q is not a verifiable ancestor of HEAD: %w", runningCommit, err)
	}
	return true, nil
}

// chainRunningCommitFn resolves the running binary's build commit. Production
// is version.Commit(); a seam so the loop breaker's "same commit twice"
// behaviour is deterministic under test (a real re-exec is not reproducible in
// a test process).
var chainRunningCommitFn = version.Commit

// chainBoundaryRepinProvenanceFn resolves the build commit + the provenance
// predicate authorizing the boundary re-pin. Production =
// defaultChainBoundaryRepinProvenance.
var chainBoundaryRepinProvenanceFn = defaultChainBoundaryRepinProvenance

// defaultChainBoundaryRepinProvenance mirrors core.defaultPostBuildRepinProvenance
// (the SAME shape the boot healer and the post-build re-pin already use): the
// running binary's build commit, plus a closure asserting that commit is an
// ancestor of HEAD (`git merge-base --is-ancestor`). An empty commit is
// unverifiable and returns false — a stripped/tampered binary can never
// self-authorize a boundary re-pin, and there is no sentinel substitute for it
// (cycle-1320 d9d245d4: the dead `repinCommit = "boundary-refresh"` fallback
// would have laundered an unverifiable commit past a real provenance gate).
//
// Cycle-1320 handed RepinShipSHA `func(string) bool { return true }`, stamping
// Authorized="provenance" on a pin nothing had verified — the ADR-0072
// forged-verdict class. "The rebuild just succeeded, so it must be HEAD" is not
// a verification: the rebuild's success says nothing about what the binary was
// built from once the tree is dirty, the rebuild is a no-op, or the source is
// tampered.
func defaultChainBoundaryRepinProvenance(projectRoot string) (string, phaseintegrity.ProvenanceVerified) {
	return chainRunningCommitFn(), func(c string) bool {
		if c == "" {
			return false
		}
		return exec.Command("git", "-C", projectRoot, "merge-base", "--is-ancestor", c, "HEAD").Run() == nil
	}
}

// chainReExecTargetFn resolves the executable the boundary refresh re-execs
// into. Production = defaultChainReExecTarget.
var chainReExecTargetFn = defaultChainReExecTarget

// defaultChainReExecTarget returns the REBUILT <projectRoot>/go/bin/evolve —
// the artifact defaultChainRebuild just wrote — never exec.LookPath(os.Args[0]),
// which resolves to the STALE running image the refresh exists to escape
// (cycle-1320 ddb8f717). An absent or non-executable target is an error, so the
// boundary degrades to "no refresh" rather than exec'ing an unknown path.
func defaultChainReExecTarget(projectRoot string) (string, error) {
	target := filepath.Join(projectRoot, "go", "bin", "evolve")
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("rebuilt binary %s: %w", target, err)
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("rebuilt binary %s is not executable (mode %s)", target, info.Mode())
	}
	return target, nil
}

// chainBoundaryRefreshAttemptFile is the on-disk re-exec loop breaker. A
// refresh records the running build commit that triggered it; a LATER attempt
// carrying that SAME commit means the previous re-exec came back on a binary
// that had not moved, so it is refused (WARN, refreshed=false) and the chain
// degrades to running batches on the current binary.
//
// On disk because a re-exec destroys any in-process counter — and because the
// refresh check precedes chainStartDecision, an unbroken loop runs ZERO batches
// forever (cycle-1320 de8b9e49 / df20cf48: a bricked chain, the exact inversion
// of the fail-open contract).
var chainBoundaryRefreshAttemptFile = "boundary-refresh-attempt.json"

// chainBoundaryRefreshAttempt is the marker's wire shape.
type chainBoundaryRefreshAttempt struct {
	RunningCommit string `json:"running_commit"`
	Batch         int    `json:"batch"`
	Timestamp     string `json:"timestamp"`
}

// chainBoundaryRefreshAlreadyAttempted reports whether a refresh has already
// been performed for runningCommit. An absent/unreadable marker means "no prior
// attempt" — the breaker must never be the thing that stops a legitimate first
// refresh.
func chainBoundaryRefreshAlreadyAttempted(evolveDir, runningCommit string) bool {
	if runningCommit == "" {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(evolveDir, chainBoundaryRefreshAttemptFile))
	if err != nil {
		return false
	}
	var rec chainBoundaryRefreshAttempt
	if err := json.Unmarshal(raw, &rec); err != nil {
		return false
	}
	return rec.RunningCommit == runningCommit
}

// recordChainBoundaryRefreshAttempt persists the marker for runningCommit,
// arming the breaker against the next boundary.
func recordChainBoundaryRefreshAttempt(evolveDir, runningCommit string, batch int) error {
	buf, err := json.Marshal(chainBoundaryRefreshAttempt{
		RunningCommit: runningCommit,
		Batch:         batch,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("marshal boundary-refresh attempt marker: %w", err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, chainBoundaryRefreshAttemptFile), buf, 0o644); err != nil {
		return fmt.Errorf("write boundary-refresh attempt marker: %w", err)
	}
	return nil
}

// chainRebuildFn is the test seam over the sanctioned rebuild recipe.
var chainRebuildFn = defaultChainRebuild

// defaultChainRebuild runs `make -C go build` (runtime-reference.md) from
// projectRoot so the on-disk binary catches up to HEAD.
func defaultChainRebuild(projectRoot string) error {
	cmd := exec.Command("make", "-C", "go", "build")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("make -C go build: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// chainReExecArgvFn resolves the argv to re-exec — deliberately just os.Args,
// so runLoopChain's signature (and every frozen call site) stays untouched.
var chainReExecArgvFn = func() []string { return os.Args }

// chainReExecFn is the seam over syscall.Exec so tests can assert the re-exec
// was invoked with the right argv without replacing the test process.
var chainReExecFn = defaultChainReExec

// defaultChainReExec replaces the current process image with argv0/argv/envv.
// A successful call never returns; an error means the exec syscall itself
// failed to launch (e.g. argv0 not executable).
func defaultChainReExec(argv0 string, argv, envv []string) error {
	return syscall.Exec(argv0, argv, envv)
}

// chainBoundaryRefreshLogFile is the additive, distinguishable audit trail
// for an automatic boundary refresh — separate from state.json's own
// two-value Authorized enum (phaseintegrity.RepinShipSHA is protected surface
// and stays unchanged), so a ledger read can tell an automatic boundary repin
// apart from a manual `evolve reset-sha`.
var chainBoundaryRefreshLogFile = "boundary-refresh-log.jsonl"

// chainBoundaryRefreshLogEntry is one JSONL line appended to
// chainBoundaryRefreshLogFile.
type chainBoundaryRefreshLogEntry struct {
	Batch           int    `json:"batch"`
	AuthorizedClass string `json:"authorized_class"`
	Timestamp       string `json:"timestamp"`
	OldSHA          string `json:"old_sha,omitempty"`
	NewSHA          string `json:"new_sha,omitempty"`
}

// appendChainBoundaryRefreshLog appends one audit record. Best-effort: a
// failure here must not itself halt the chain (the pin already moved).
func appendChainBoundaryRefreshLog(evolveDir string, batch int, res phaseintegrity.RepinResult) error {
	entry := chainBoundaryRefreshLogEntry{
		Batch:           batch,
		AuthorizedClass: "boundary-refresh",
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		OldSHA:          res.OldSHA,
		NewSHA:          res.NewSHA,
	}
	buf, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal boundary-refresh log entry: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(evolveDir, chainBoundaryRefreshLogFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open boundary-refresh log: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(buf, '\n')); err != nil {
		return fmt.Errorf("write boundary-refresh log: %w", err)
	}
	return nil
}

// lastChainBoundaryRefreshLogEntry reads chainBoundaryRefreshLogFile under
// evolveDir and returns the LAST (most-recently-appended) entry. Best-effort,
// mirroring spineFailOpenRollup's nil-when-clean shape: a missing file, an
// empty file, or a read/parse error all resolve to (nil, nil) — surfacing a
// refresh event into the summary must never itself become a new failure mode
// for an already-successful boundary refresh.
func lastChainBoundaryRefreshLogEntry(evolveDir string) (*chainBoundaryRefreshLogEntry, error) {
	raw, err := os.ReadFile(filepath.Join(evolveDir, chainBoundaryRefreshLogFile))
	if err != nil {
		return nil, nil
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var entry chainBoundaryRefreshLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, nil
		}
		return &entry, nil
	}
	return nil, nil
}

// chainBoundaryFleetLaneFn is the test seam for "is a sibling fleet lane
// active" — checked AFTER chainBoundaryAheadFn confirms staleness and BEFORE
// chainBoundaryRefreshAlreadyAttempted/chainRebuildFn are ever reached, so an
// active sibling lane refuses the boundary heal before either rebuild or
// exec (mirrors the boot-time healer's own guard order, the stranded salvage
// commit cycle-42824668-1360/e057d1b3's design for the OLDER boot-only
// healer). An error from the check is unverifiable safety state and must be
// treated exactly like laneActive=true.
var chainBoundaryFleetLaneFn = defaultChainBoundaryFleetLaneActive

// defaultChainBoundaryFleetLaneActive wraps gc.Discover — the SAME
// lease-aware run-dir scan the retention engine (L3.2) already owns and
// adversarially hardens — rather than re-implementing an independent
// .lease reader (never_duplicate_centralize_via_design_patterns). It reports
// true iff any discovered run dir is Live AND that dir is NOT this same
// process's own lease.
//
// Self-exclusion is REQUIRED, not optional (cycle-1364 D1, CRITICAL): the
// original design assumed "this lane's own history is already terminal" by
// the time maybeRefreshChainBoundary runs, but runlease's freshness window
// (runlease.DefaultTTL, 10 minutes) and the WRITE-ORDERING INVARIANT (a
// scheduler releases a lease only by letting it age out, never by deleting
// it — internal/runlease doc comment) mean THIS lane's own just-completed
// run dir keeps reporting Live for up to 10 more minutes. Without
// self-exclusion, gc.Discover always finds at least one Live dir — this
// lane's own — so defaultChainBoundaryFleetLaneActive reports active=true
// unconditionally and the boundary binary auto-refresh this function exists
// to provide never fires (probe evidence: active=true with the caller's own
// .lease as the only "sibling").
//
// The exclusion key is runlease.Lease.OwnerPID, not the run-dir path: the
// whole chain (every batch between re-execs) runs in one process, and every
// lease this process writes carries os.Getpid() (internal/core/
// runlease_hook.go). A DIFFERENT lane is, by construction, a different OS
// process, so a lease whose OwnerPID does not match ours is unambiguously a
// sibling — self-exclusion is safe (never masks a real sibling) because a
// sibling can never share our pid. A dir that is Live via gc's OTHER
// liveness source (dir == current workspace, no fresh lease at all) has no
// OwnerPID to compare and is deliberately NOT excluded: cfg.EvolveDir can be
// the shared plane runs/ dir in fleet mode, so that signal is not provably
// this process's own the way a pid-matched lease is — falling through to
// active=true there keeps the existing fail-safe posture (refuse the
// rebuild when unverifiable) rather than risking a false self-exclusion.
func defaultChainBoundaryFleetLaneActive(cfg loopConfig) (active bool, err error) {
	dirs, err := gc.Discover(cfg.EvolveDir, gc.DiscoverOptions{})
	if err != nil {
		return false, fmt.Errorf("chain boundary fleet-lane discovery: %w", err)
	}
	selfPID := os.Getpid()
	for _, d := range dirs {
		if !d.Live {
			continue
		}
		if lease, ok, lerr := runlease.Read(d.Path); lerr == nil && ok && lease.OwnerPID == selfPID {
			// This process's own lease (or own current-workspace dir with no
			// lease yet) — not a sibling. Keep scanning; a real sibling may
			// still be discovered.
			continue
		}
		return true, nil
	}
	return false, nil
}

// maybeRefreshChainBoundary runs the ahead-check -> rebuild -> repin ->
// audit-log -> re-exec sequence for boundary `batch`. It is called ONLY from
// runLoopChain between chainStartDecision deciding to continue and
// runLoopBatchFn — the loop body is single-threaded per boundary, so this
// call-site placement is what satisfies "refuse entirely while a batch is
// mid-flight"; no separate lock is needed. Every failure at every stage
// (ahead-check error, rebuild error, repin error, re-exec error) degrades to
// refreshed=false, logged to stderr, and the CURRENT binary keeps running the
// chain — never halts.
func maybeRefreshChainBoundary(cfg loopConfig, batch int, stderr io.Writer) (refreshed bool) {
	runningCommit := chainRunningCommitFn()
	ahead, err := chainBoundaryAheadFn(cfg.ProjectRoot, runningCommit)
	if err != nil {
		fmt.Fprintf(stderr, "[chain] boundary-refresh: ahead-check failed (%v) — skipping refresh, continuing on the current binary\n", err)
		return false
	}
	if !ahead {
		return false
	}

	// Fleet-lane guard BEFORE the rebuild-attempt breaker: a sibling lane
	// holding a live run lease means this rebuild would overwrite the shared
	// go/bin/evolve binary out from under it — the standing rule "NEVER
	// rebuild the plane binary mid-batch" (project memory
	// stale_binary_false_fail). An error from the check is unverifiable
	// safety state, not proof the plane is idle, so it refuses exactly like
	// an active lane — fail-safe for the rebuild while the chain itself still
	// degrades to "no refresh" rather than halting (fail-open for the chain).
	if active, lerr := chainBoundaryFleetLaneFn(cfg); lerr != nil {
		fmt.Fprintf(stderr, "[chain] boundary-refresh: WARN fleet lane check unverifiable (%v) — cannot prove the plane is idle, refusing to rebuild the shared binary; continuing on the current binary\n", lerr)
		return false
	} else if active {
		fmt.Fprintf(stderr, "[chain] boundary-refresh: a sibling fleet lane is active (fresh run lease) — refusing to rebuild the plane binary mid-batch; continuing on the current binary\n")
		return false
	}

	// Loop breaker BEFORE the rebuild: a second attempt carrying the same build
	// commit means the previous re-exec came back on a binary that had not
	// moved. Refuse so the chain runs batches on the current binary instead of
	// livelocking at zero batches executed.
	if chainBoundaryRefreshAlreadyAttempted(cfg.EvolveDir, runningCommit) {
		fmt.Fprintf(stderr, "[chain] boundary-refresh: REFUSED — a refresh was already performed for build commit %.12s and the running binary still reports it; the rebuild did not move the binary. Continuing on the current binary rather than re-execing again (see .evolve/%s)\n",
			runningCommit, chainBoundaryRefreshAttemptFile)
		return false
	}

	fmt.Fprintf(stderr, "[chain] boundary-refresh: HEAD has advanced past the running binary's build commit (%.12s) — rebuilding\n", runningCommit)
	if err := chainRebuildFn(cfg.ProjectRoot); err != nil {
		fmt.Fprintf(stderr, "[chain] boundary-refresh: rebuild failed (%v) — skipping refresh, continuing on the current binary\n", err)
		return false
	}

	// Resolve the re-exec target BEFORE the re-pin: if there is nothing safe to
	// come back on, the pin must stay where it is.
	target, err := chainReExecTargetFn(cfg.ProjectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "[chain] boundary-refresh: no re-exec target (%v) — skipping refresh, continuing on the current binary\n", err)
		return false
	}

	// The re-pin goes through phaseintegrity.RepinIfDrifted — the SAME shared
	// detect-drift + provenance-gate + re-pin path core.repinShipSHAAfterBuild
	// and the boot healer use — so this cycle deletes the second, hand-rolled
	// re-pin path instead of maintaining two that can diverge. It hashes the
	// REBUILT binary at `target` (cycle-1320 d7542cf6 pinned selfsha.Running(),
	// the stale running image) and it consults a REAL provenance closure
	// (cycle-1320 dd8a9d64/dcaf44e4 passed an always-true stub). Unverified
	// provenance ⇒ RepinIfDrifted refuses with an error and leaves the pin
	// untouched; that refusal degrades this boundary to "no refresh".
	statePath := filepath.Join(cfg.EvolveDir, "state.json")
	repinCommit, prov := chainBoundaryRepinProvenanceFn(cfg.ProjectRoot)
	res, err := phaseintegrity.RepinIfDrifted(statePath, target, repinCommit, "", prov)
	if err != nil {
		fmt.Fprintf(stderr, "[chain] boundary-refresh: repin refused (%v) — skipping refresh, continuing on the current binary\n", err)
		return false
	}

	if err := appendChainBoundaryRefreshLog(cfg.EvolveDir, batch, res); err != nil {
		fmt.Fprintf(stderr, "[chain] boundary-refresh: WARN: audit log write failed (%v) — pin already moved\n", err)
	}

	argv := chainReExecArgvFn()
	if len(argv) == 0 {
		fmt.Fprintf(stderr, "[chain] boundary-refresh: empty re-exec argv — skipping refresh\n")
		return false
	}

	// Arm the breaker BEFORE the exec: a successful syscall.Exec never returns,
	// so anything written after it never happens.
	if err := recordChainBoundaryRefreshAttempt(cfg.EvolveDir, runningCommit, batch); err != nil {
		fmt.Fprintf(stderr, "[chain] boundary-refresh: cannot arm the re-exec loop breaker (%v) — skipping refresh rather than risking an unbounded re-exec loop\n", err)
		return false
	}

	fmt.Fprintf(stderr, "[chain] boundary-refresh: re-pinned %.12s -> %.12s — re-executing %s to pick up the rebuilt binary\n", res.OldSHA, res.NewSHA, target)
	if err := chainReExecFn(target, argv, os.Environ()); err != nil {
		fmt.Fprintf(stderr, "[chain] boundary-refresh: re-exec failed (%v) — continuing on the current binary\n", err)
		return false
	}
	return true
}

// chainStartDecision decides whether batch n (0-based) may start, and names
// the reason when it may not. Pure, so the precedence between the three
// pre-batch stop conditions is testable without running a batch: the operator
// brake outranks everything (it is an explicit instruction), then a drained
// inbox AFTER at least one batch (the success exit), then the runaway cap.
//
// The `n > 0` scope on the drained-inbox exit is the min-one-batch guarantee:
// a drained inbox is a CONTINUE condition ("we finished the work"), not a START
// condition ("there was never any work"). Without it, opting into chaining is
// silently WEAKER than the pre-chain contract, where `evolve loop` always ran
// one batch — a chain launched against an already-drained inbox returned rc=0
// having run zero cycles. The allowance is deliberately narrow: it is outranked
// by the brake (an explicit operator instruction is never overridden by a
// contract default) and it never widens the cap, which stays an exact ceiling
// (a non-positive cap still runs nothing — no cap+1).
func chainStartDecision(n, maxBatches, inboxPending int, brake bool) (reason string, stop bool) {
	switch {
	case brake:
		return "chain_operator_brake", true
	case inboxPending == 0 && n > 0:
		return "chain_inbox_empty", true
	case n >= maxBatches:
		return "chain_max_batches", true
	}
	return "", false
}

// chainContinueDecision maps a finished batch's exit code onto the chain's
// next move. rc 0 (clean) and rc 3 (batch completed with absorbed
// recoverable/verdict failures) are both "the batch ran to completion" — the
// queue is never halted for them (never_stop_queue_inject_inbox). rc 5 is the
// QUOTA-PAUSE contract the batch derives from core.allFamiliesQuotaExhausted:
// relaunching would only burn the next batch into the same drained families,
// so the chain defers with the checkpoint intact. Every other code is a fatal
// batch outcome (preflight, unfinished cycle, ADR-0072 system-failure halt,
// signal) and propagates unchanged.
func chainContinueDecision(rc int) (reason string, exit int, stop bool) {
	switch rc {
	case 0, 3:
		return "", rc, false
	case 5:
		return "chain_quota_defer", 5, true
	default:
		return "chain_batch_error", rc, true
	}
}

// runLoopChain drives runLoopBatch until a boundary condition stops it. The
// same loopConfig value is handed to EVERY batch — no per-batch re-derivation
// — so fleet width and every other resolved setting are preserved across the
// chain by construction.
func runLoopChain(cfg loopConfig, cc policy.ChainConfig, stdin io.Reader, stdout, stderr io.Writer) int {
	res := chainResult{ChainMode: true, MaxBatches: cc.MaxBatches}
	exit := 0

	for n := 0; ; n++ {
		pending, skipped, err := inboxPendingCount(cfg.EvolveDir)
		if err != nil {
			fmt.Fprintf(stderr, "[chain] cannot read the inbox (%v) — stopping the chain rather than looping blind\n", err)
			res.StopReason = "chain_inbox_unreadable"
			exit = 2
			break
		}
		// Fail loudly: a root-level *.json that is not an item is usually a real
		// todo lost to a typo. Named BEFORE the stop decision so the operator
		// sees it even on the boundary that ends the chain.
		for _, name := range skipped {
			fmt.Fprintf(stderr, "[chain] skipping .evolve/inbox/%s — not a valid inbox item (no parseable object with an `id`); it is NOT counted as pending work\n", name)
		}
		// Precedence (scout Hypothesis 1): operator brake > refresh-needed >
		// drained-inbox > max-batches. `brake` is resolved exactly ONCE here —
		// the single source of truth threaded through both branches below —
		// because the brake must gate the SIDE-EFFECTING boundary-refresh call
		// (rebuild/re-exec) before it ever runs, which chainStartDecision's
		// pure, no-side-effects signature cannot do on its own (it can't run
		// maybeRefreshChainBoundary from inside a pure decision function). The
		// early return below is therefore the ONLY place brake=true is ever
		// acted on in production; the same resolved value (never a fresh
		// chainBrakeEngaged() call, never a hardcoded literal) is passed to
		// chainStartDecision purely so its documented four-branch contract
		// (exercised directly by TestChainStartDecision) reflects the real
		// precedence even though brake is always false by the time control
		// reaches it here (an engaged brake already broke the loop above) —
		// one resolution, two consumers, never a second decision.
		brake := chainBrakeEngaged(cfg.EvolveDir)
		if brake {
			res.StopReason = "chain_operator_brake"
			fmt.Fprintf(stderr, "[chain] stopping after %d batch(es): %s (inbox pending=%d, cap=%d)\n",
				len(res.Batches), res.StopReason, pending, cc.MaxBatches)
			break
		}

		if maybeRefreshChainBoundary(cfg, n+1, stderr) {
			res.StopReason = "chain_boundary_refresh_reexec"
			if entry, err := lastChainBoundaryRefreshLogEntry(cfg.EvolveDir); err == nil {
				res.BoundaryRefresh = entry
			}
			fmt.Fprintf(stderr, "[chain] stopping after %d batch(es): %s — re-exec is terminal, the new process resumes the chain\n",
				len(res.Batches), res.StopReason)
			break
		}

		if reason, stop := chainStartDecision(n, cc.MaxBatches, pending, brake); stop {
			res.StopReason = reason
			fmt.Fprintf(stderr, "[chain] stopping after %d batch(es): %s (inbox pending=%d, cap=%d)\n",
				len(res.Batches), reason, pending, cc.MaxBatches)
			break
		}

		// Width is read (not re-resolved into cfg) purely to record it: the
		// batch resolves its own fleet block, so an operator widening mid-chain
		// still takes effect — what must never happen is the CHAIN narrowing it.
		width := loadFleetConfig(cfg.EvolveDir).Count
		fmt.Fprintf(stderr, "[chain] batch %d/%d starting — inbox pending=%d, fleet lanes=%d\n",
			n+1, cc.MaxBatches, pending, width)

		rc := runLoopBatchFn(cfg, stdin, stdout, stderr)
		res.Batches = append(res.Batches, chainBatchRecord{Batch: n + 1, RC: rc, FleetCount: width, InboxPending: pending})

		reason, code, stop := chainContinueDecision(rc)
		if !stop {
			continue
		}
		res.StopReason, exit = reason, code
		if reason == "chain_quota_defer" {
			emitChainQuotaDefer(cfg, n+1, stderr)
		} else {
			fmt.Fprintf(stderr, "[chain] batch %d exited rc=%d — stopping the chain (%s)\n", n+1, rc, reason)
		}
		break
	}

	buf, _ := json.MarshalIndent(res, "", "  ")
	fmt.Fprintln(stdout, string(buf))
	return exit
}

// emitChainQuotaDefer prints the deferral notice for a quota-walled batch,
// including the checkpoint's reset-time hint when one was written. The point
// of the chain reading it here is the negative behaviour: it does NOT start
// another batch into families that are already drained.
func emitChainQuotaDefer(cfg loopConfig, batch int, stderr io.Writer) {
	if qp, ok := detectQuotaPause(cfg.EvolveDir); ok {
		fmt.Fprintf(stderr, "[chain] batch %d hit the quota wall (cycle=%d wake-at=%s source=%s) — DEFERRING, not relaunching\n",
			batch, qp.Cycle, qp.WakeAt, qp.Source)
	} else {
		fmt.Fprintf(stderr, "[chain] batch %d hit the quota wall (no checkpoint block on disk) — DEFERRING, not relaunching\n", batch)
	}
	fmt.Fprintln(stderr, "[chain]   the checkpoint is intact; resume when quota resets: evolve loop --resume")
}
