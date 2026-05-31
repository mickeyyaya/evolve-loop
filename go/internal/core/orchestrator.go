package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/backfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// CycleRequest is the operator-facing input to RunCycle.
type CycleRequest struct {
	ProjectRoot string
	GoalHash    string
	Budget      BudgetEnvelope
	// Env is propagated to every PhaseRequest.Env that runs in this
	// cycle. Phases consult it for CLI/model selection
	// (EVOLVE_CLI, EVOLVE_<PHASE>_MODEL, …). The orchestrator copies the
	// map so post-RunCycle operator mutation does not affect in-flight
	// or completed runs.
	Env map[string]string
	// Context seeds the PhaseRequest.Context every phase receives. Ship
	// requires Context["commit_message"]; Scout reads
	// Context["strategy"]. Copied like Env.
	Context map[string]string
}

// CycleResult summarises what RunCycle did.
type CycleResult struct {
	Cycle        int
	FinalVerdict string
	PhasesRun    []Phase
	// RetroDecision is the failure-adapter's verdict on the retro branch,
	// populated only when retro ran. Format: "<action>: <reason>".
	RetroDecision string
}

// Orchestrator drives one cycle through the state machine, calling a
// PhaseRunner per phase and appending ledger entries. It is pure: all
// I/O is delegated to the injected Storage and Ledger ports.
//
// This is the Phase 1 skeleton — guards, observer, budget enforcement
// land in Phase 2.
type Orchestrator struct {
	storage Storage
	ledger  Ledger
	runners map[Phase]PhaseRunner
	sm      *StateMachine
	now     func() time.Time
	// gitHEAD returns the current git HEAD SHA. Called once at cycle
	// start and once before finalizing the verdict so the orchestrator
	// can detect whether anything got committed during the cycle (e.g.
	// when the build phase invokes `evolve ship --class manual` inline).
	// Errors are swallowed and treated as "no movement detected" — the
	// outcome calculator falls back to SKIPPED_UNKNOWN.
	gitHEAD func() (string, error)

	// gitDirtyPaths returns the set of modified tracked paths in the main
	// repo's working directory (`git diff --name-only HEAD` in repoRoot).
	// Workstream B's tree-diff guard snapshots this before each source-
	// writing phase and compares after — any newly-dirty MAIN-tree path is a
	// leak that escaped the sandbox (each git worktree is a separate working
	// dir, so its writes don't show up here). Injected for tests.
	gitDirtyPaths func(ctx context.Context, repoRoot string) ([]string, error)

	// worktree provisions/cleans the per-cycle source worktree (ADR-0027).
	// Default gitWorktree (real git); injected in tests via
	// WithWorktreeProvisioner so RunCycle runs without touching real git.
	worktree WorktreeProvisioner

	// cfg + strategy drive dynamic phase routing ("model proposes, kernel
	// disposes"). The zero value (Stage:Off, StaticPreset) reproduces the
	// legacy static-state-machine behavior byte-for-byte: routing is
	// computed only when the composition root opts in via WithRouting with
	// a non-Off stage. The orchestrator never reads a routing flag itself —
	// config.Load (the composition root) is the sole env/file reader.
	cfg      config.RoutingConfig
	strategy router.RoutingStrategy

	// planner produces the upfront whole-cycle plan (ADR-0024 §2). Optional:
	// nil ⇒ no advisor plan ⇒ the kernel floor falls back to the configurable
	// never-skip spine (fail-safe to static). Consulted once at cycle start,
	// only at Stage>=Advisory; its output is clamped to the integrity floor
	// before being threaded into every routing decision.
	planner router.Planner

	// catalog is the merged phase catalog (built-in + user overlays). It lets
	// the orchestrator accept and run user-defined phases on the dynamic-routing
	// path WITHOUT hardcoding them in the Phase enum / state machine. Empty (the
	// default) ⇒ only built-in phases exist ⇒ byte-identical legacy behavior.
	catalog phasespec.Catalog

	// reviewer adjudicates a finished phase's deliverable before the cycle
	// advances (Workstream E2). Nil ⇒ noopReviewer default ⇒ every non-error,
	// non-SKIPPED verdict is recorded as a success (pre-E2 behavior). Set via
	// WithReviewer; the deterministic default + future LLM reviewer
	// implementations share the DeliverableReviewer interface.
	reviewer DeliverableReviewer

	// observer is the per-phase stall detector (cycle-122 Fix 3 / ADR-0030).
	// Start is called once before each runner.Run; the returned cancel runs
	// once after. Nil ⇒ noopObserver default ⇒ byte-identical to the pre-
	// ADR-0030 cycle. Set via WithObserver; cmd_cycle.go wires the real
	// implementation when EVOLVE_OBSERVER_AUTOSPAWN != "0" (default 1).
	observer Observer
}

// Option customizes an Orchestrator at construction (functional-options DI).
// Absent any option, the orchestrator runs in legacy Stage:Off mode.
type Option func(*Orchestrator)

// WithRouting injects the loaded routing config + the strategy selected once
// at the composition root. A nil strategy is ignored so the StaticPreset
// default stands; the orchestrator depends only on the RoutingStrategy
// interface, never on a mode conditional.
func WithRouting(cfg config.RoutingConfig, strategy router.RoutingStrategy) Option {
	return func(o *Orchestrator) {
		o.cfg = cfg
		if strategy != nil {
			o.strategy = strategy
		}
	}
}

// WithPlanner injects the whole-cycle phase planner (ADR-0024 §2 hybrid
// cadence). A nil planner is ignored so the no-plan default stands; the
// orchestrator consults it only at Stage>=Advisory and always clamps its
// output to the integrity floor — "model proposes, kernel disposes".
func WithPlanner(p router.Planner) Option {
	return func(o *Orchestrator) {
		if p != nil {
			o.planner = p
		}
	}
}

// WithCatalog injects the merged phase catalog so the orchestrator can accept
// and run user-defined (non-built-in) phases on the dynamic-routing path. The
// empty default keeps behavior byte-identical to the built-in-only pipeline.
func WithCatalog(cat phasespec.Catalog) Option {
	return func(o *Orchestrator) { o.catalog = cat }
}

// WithWorktreeProvisioner injects a worktree provisioner. Tests pass a fake to
// avoid real git; nil is ignored so the gitWorktree default stands.
func WithWorktreeProvisioner(p WorktreeProvisioner) Option {
	return func(o *Orchestrator) {
		if p != nil {
			o.worktree = p
		}
	}
}

// WithObserver injects a per-phase stall detector (cycle-122 Fix 3 / ADR-0030).
// The orchestrator calls observer.Start(...) before each phase's runner.Run
// and the returned cancel after — running a background watcher that emits
// stall_no_output events to the workspace when the subagent's stdout-log
// stops growing. A nil observer (default) keeps the noopObserver default,
// which is byte-identical to the pre-ADR-0030 cycle.
//
// cmd_cycle.go wires the real implementation via
// observer.NewCoreAdapter when EVOLVE_OBSERVER_AUTOSPAWN != "0" (default 1).
func WithObserver(o Observer) Option {
	return func(orch *Orchestrator) {
		if o != nil {
			orch.observer = o
		}
	}
}

// WithReviewer injects a per-phase deliverable reviewer (Workstream E2). The
// orchestrator calls reviewer.Review(...) after each phase's runner.Run returns
// a non-error, non-SKIPPED verdict, BEFORE the ledger append or
// CompletedPhases++. Approve=false aborts the cycle with the reviewer's Reason
// (no retry budget yet — that's a follow-up; see the WS-E plan). A nil reviewer
// keeps the noopReviewer default, which is byte-identical to the pre-E2 cycle.
func WithReviewer(r DeliverableReviewer) Option {
	return func(o *Orchestrator) {
		if r != nil {
			o.reviewer = r
		}
	}
}

// NewOrchestrator wires the orchestrator with its dependencies. Routing stays
// off unless a WithRouting option supplies an enabled-stage config.
func NewOrchestrator(storage Storage, ledger Ledger, runners map[Phase]PhaseRunner, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		storage:       storage,
		ledger:        ledger,
		runners:       runners,
		sm:            NewStateMachine(),
		now:           time.Now,
		gitHEAD:       defaultGitHEAD,
		gitDirtyPaths: defaultGitDirtyPaths,
		worktree:      gitWorktree{},
		strategy:      router.StaticPreset{},
		reviewer:      noopReviewer{}, // WS-E2: byte-identical default until WithReviewer is used
		observer:      noopObserver{}, // cycle-122 Fix 3 / ADR-0030: byte-identical default until WithObserver is used
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// archivePollutedWorkspace renames <workspace>/ to
// <workspace>.polluted-<UTCnano>/ when it exists and is non-empty.
// Returns nil for the empty-or-missing case (the cycle just runs in a
// fresh directory). Returns the underlying error only when stat/rename
// actually fails. Tests inject a deterministic clock via now.
func archivePollutedWorkspace(workspace string, now func() time.Time) error {
	info, err := os.Stat(workspace)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat workspace: %w", err)
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return fmt.Errorf("readdir workspace: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	stamp := now().UTC().Format("20060102T150405.000000000")
	archived := workspace + ".polluted-" + stamp
	if err := os.Rename(workspace, archived); err != nil {
		return fmt.Errorf("rename to %s: %w", archived, err)
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] archived polluted workspace: %s -> %s (%d files)\n",
		workspace, archived, len(entries))
	return nil
}

// defaultGitHEAD runs `git rev-parse HEAD` in cwd.
// Returns empty string on error AND emits a one-line WARN to stderr so
// operators see the degraded-mode signal that yields SKIPPED_UNKNOWN.
// finalizeOutcome treats equal strings as no movement.
func defaultGitHEAD() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN git HEAD probe failed (cycle outcome labels degraded): %v\n", err)
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// recordAuditBinding writes the rich auditor ledger entry that ship's
// audit-binding (verify.go findLatestAudit / verifyAuditBinding) requires:
// role=auditor, kind=agent_subprocess, with git_head + tree_state_sha +
// artifact_path/sha256. Without it the Go orchestrator recorded audit only as
// kind:phase (no binding fields), so ship fell back to an ancient bash-era
// auditor entry and every cycle failed AUDIT_BINDING_HEAD_MOVED (root cause,
// 2026-05-29). tree_state_sha is sha256(`git diff HEAD`) — byte-identical to
// ship's computeTreeStateSHA so the bind matches. Best-effort: a failure WARNs
// and is swallowed; ship then fails loudly on the missing/stale binding rather
// than shipping unbound.
func (o *Orchestrator) recordAuditBinding(ctx context.Context, cycle int, projectRoot, workspace, worktree, verdict string) {
	head, _, err := gitCapture(ctx, projectRoot, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding: git rev-parse HEAD failed: %v (ship will refuse to bind)\n", err)
		return
	}
	// Worktree CHANGES tree: stage everything (respects .gitignore) and write a
	// tree object = exactly the tree ship will commit. This is what the auditor
	// SHOULD bind (it audited the worktree's working changes); its persona binds
	// HEAD^{tree} = the unchanged base, which can never equal the changes-commit
	// tree → INTEGRITY_TREE_DRIFT every cycle (cycle-152). Ship prefers this
	// over the auditor's comment. Best-effort: empty ⇒ ship falls back to the
	// auditor's value. No commit is made (write-tree only); ship re-stages anyway.
	worktreeTree := ""
	if worktree != "" {
		if _, _, aerr := gitCapture(ctx, worktree, "add", "-A"); aerr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding: git add -A in worktree failed: %v — WorktreeTreeSHA empty, ship falls back to the auditor comment\n", aerr)
		} else if wt, code, werr := gitCapture(ctx, worktree, "write-tree"); werr == nil && code == 0 {
			worktreeTree = strings.TrimSpace(wt)
		} else {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding: git write-tree in worktree failed (rc=%d): %v\n", code, werr)
		}
	}
	// `git diff HEAD` returns exit 1 when differences exist — not an error;
	// only exit >1 (e.g. 128) is fatal. Match computeTreeStateSHA semantics.
	diff, code, err := gitCapture(ctx, projectRoot, "diff", "HEAD")
	if err != nil || code > 1 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding: git diff HEAD failed (rc=%d): %v\n", code, err)
		return
	}
	treeSum := sha256.Sum256([]byte(diff))
	artPath := filepath.Join(workspace, "audit-report.md")
	artBytes, err := os.ReadFile(artPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding: read %s: %v\n", artPath, err)
		return
	}
	artSum := sha256.Sum256(artBytes)
	// exit_code mirrors the Unix-convention auditor signal ship tolerates (0|1):
	// 0 = clean PASS, 1 = findings (WARN). Ship's binding accepts both; this
	// keeps the ledger semantically accurate for operators reading it.
	exitCode := 0
	if verdict == VerdictWARN {
		exitCode = 1
	}
	if err := o.ledger.Append(ctx, LedgerEntry{
		TS:              o.now().UTC().Format(time.RFC3339),
		Cycle:           cycle,
		Role:            "auditor",
		Kind:            "agent_subprocess",
		ExitCode:        exitCode,
		GitHEAD:         strings.TrimSpace(head),
		TreeStateSHA:    hex.EncodeToString(treeSum[:]),
		WorktreeTreeSHA: worktreeTree,
		ArtifactPath:    artPath,
		ArtifactSHA256:  hex.EncodeToString(artSum[:]),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding ledger append: %v\n", err)
	}
}

// normalizeWorktreeToBase soft-resets the worktree to baseSHA so any commits a
// builder made during the build phase become PENDING changes again. The builder
// is instructed to `git add -A && git commit -m "… [worktree-build]"`
// (agents/evolve-builder.md:235) for crash-safety, but the auditor
// (agents/evolve-auditor.md:57: "Run `git diff HEAD`") and the orchestrator's
// audit-binding (recordAuditBinding: sha256(`git diff HEAD`)) both inspect the
// PENDING diff — which is empty after a commit. agy/Gemini followed the commit
// instruction literally and every cycle's work was discarded as "tree lacks the
// files". Resetting --soft to the cycle base re-exposes the work to `git diff
// HEAD` without changing the auditor prompt or the security binding. See
// docs/incidents/cycle-156-builder-commit-vs-audit-pending-diff.md (Option C).
//
// Best-effort: any failure WARNs and leaves the worktree untouched (audit then
// inspects whatever state exists); it NEVER aborts the cycle. No-op when HEAD is
// already at baseSHA (the builder left changes uncommitted — the historical
// Claude-builder path), so opting in is byte-identical for non-committing builders.
func normalizeWorktreeToBase(ctx context.Context, worktree, baseSHA string) {
	if worktree == "" || baseSHA == "" {
		return
	}
	head, _, err := gitCapture(ctx, worktree, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree-normalize: rev-parse HEAD failed: %v (audit inspects worktree as-is)\n", err)
		return
	}
	if strings.TrimSpace(head) == baseSHA {
		return // builder left changes uncommitted — nothing to normalize
	}
	if _, code, rerr := gitCapture(ctx, worktree, "reset", "--soft", baseSHA); rerr != nil || code != 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree-normalize: git reset --soft %s failed (rc=%d): %v (audit inspects committed state as-is)\n", baseSHA, code, rerr)
		return
	}
	short := baseSHA
	if len(short) > 12 {
		short = short[:12]
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] worktree-normalize: soft-reset builder commits to base %s — changes now pending for audit\n", short)
}

// porcelainDirtySet returns the set of paths `git status --porcelain` reports
// dirty in dir — tracked-modified AND untracked. Captured for the main tree at
// cycle start so recoverBuildLeak only touches paths the BUILD introduced, never
// the operator's pre-existing uncommitted work. (The tree-diff guard's
// `git diff --name-only HEAD` baseline is tracked-only and misses untracked, so
// it can't serve this purpose — see the cycle-160 incident.)
func porcelainDirtySet(ctx context.Context, dir string) map[string]bool {
	set := map[string]bool{}
	// -uall lists every untracked FILE individually (never a bare directory), so
	// recoverBuildLeak relocates at file granularity — no dir-rename ENOTEMPTY in
	// a real worktree, and the baseline is file-exact.
	out, code, err := gitCapture(ctx, dir, "status", "--porcelain", "-uall")
	if err != nil || code != 0 {
		return set
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		set[porcelainPath(line)] = true
	}
	return set
}

// porcelainPath extracts the path from a `git status --porcelain` line. Lines are
// "XY <path>"; a rename/copy is "XY <old> -> <new>" (take the new path). Quotes
// (paths with special chars) are trimmed best-effort.
func porcelainPath(line string) string {
	p := strings.TrimSpace(line[3:])
	if i := strings.Index(p, " -> "); i >= 0 {
		p = p[i+4:]
	}
	return strings.Trim(p, "\"")
}

// recoverBuildLeak relocates build-phase writes that escaped into the main tree
// back into the worktree, then restores the main tree — the self-heal for the
// cycle-160 incident (Option A). Non-Claude builders (agy/codex in tmux) are not
// bound by the Claude-only role-gate, and the OS sandbox is off on nested-macOS,
// so they can write to project_root instead of the worktree. Rather than abort
// the cycle, move the build's output to where audit/ship expect it.
//
// baseline (file-granular via `git status --porcelain -uall`) = paths already
// dirty in projectRoot before the build (operator / pre-existing work) — never
// touched. For each NEW dirty path:
//   - untracked ('?')                 → os.Rename(projectRoot/p → worktree/p);
//     the relocated paths (and ONLY those) are then `git add --`'d in the worktree
//     so the auditor's `git diff HEAD` sees them without sweeping in unrelated
//     worktree content (same visibility reason as normalizeWorktreeToBase).
//   - rebuilt release binary (buildArtifacts: go/evolve, go/bin/evolve) → always
//     discard (git checkout HEAD -- p); never relocate, or the cycle would commit
//     binary drift (cycle-153). go/evolve is re-committed only by the release pipeline.
//   - modified tracked ('M') → real builder work edited in the MAIN tree (cycle-162:
//     orchestrator.go). If the worktree has NOT independently touched p (its copy is
//     at HEAD) → relocate the leaked content into the worktree (preserve the work) +
//     stage it. If the worktree diverged for p → discard the main leak (worktree is
//     authoritative).
//   - added/deleted tracked ('A'/'D') → git checkout HEAD -- p (discards staged AND
//     unstaged; plain `git checkout -- p` would no-op a staged-only change).
//   - rename/copy/other → not safe to auto-recover → return false.
//
// Returns true iff every NEW leak was handled and the main tree is clean of them;
// the caller continues. On false the caller ABORTS the cycle — the tree-diff
// guard only backstops tracked leaks, so an unrecovered (esp. untracked) leak
// must not be allowed to slip past into audit. "Couldn't determine" cases degrade
// to true (let the guard be the backstop). Best-effort + loud WARNs throughout.
func recoverBuildLeak(ctx context.Context, projectRoot, worktree string, baseline map[string]bool) bool {
	if worktree == "" {
		return true // no worktree to relocate into → degrade (caller guards this anyway)
	}
	// -uall lists untracked FILES individually (never a bare dir), so each leaked
	// path is a file: os.Rename has no dir-collision and is overwrite-safe.
	out, code, err := gitCapture(ctx, projectRoot, "status", "--porcelain", "-uall")
	if err != nil || code != 0 {
		// Can't determine leaks → DEGRADE to the tree-diff guard (return true, do
		// not abort). false is reserved for a leak we detected but could not safely
		// recover; "couldn't even check" is not that.
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: git status failed (rc=%d): %v — degrading to tree-diff guard\n", code, err)
		return true
	}
	var relocated []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		p := porcelainPath(line)
		if p == "" || baseline[p] {
			continue
		}
		// Skip the orchestrator's own runtime state and directory entries — never
		// build output, and not safe to relocate as a file:
		//   - .evolve/ : cycle-state, ledger, runs, guards.log, and the cycle's OWN
		//     worktree live here. Match it at ANY path depth — guard hooks run with
		//     cwd set to subdirectories and write nested `<subdir>/.evolve/guards.log`
		//     (cycle-176 / issue #11): top-level-only matching missed those, so
		//     recoverBuildLeak relocated them and the gitignored `git add` failed →
		//     batch abort. Both top-level (`.evolve/...`) and nested (`.../.evolve/...`)
		//     are skipped.
		//   - trailing '/' : a nested worktree/submodule that `-uall` reports as a bare
		//     directory (it won't recurse into another working tree). moveFile cannot
		//     move a directory — this is the cycle-1 worktree dir that aborted the cycle
		//     (415a9a7 regression caught by the e2e ship-path tests).
		if p == ".evolve" || strings.HasPrefix(p, ".evolve/") || strings.Contains(p, "/.evolve/") || strings.HasSuffix(p, "/") {
			continue
		}
		xy := line[:2]
		switch {
		case strings.Contains(xy, "?"): // untracked file → relocate into the worktree
			src := filepath.Join(projectRoot, p)
			dst := filepath.Join(worktree, p)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: mkdir for %s: %v\n", p, err)
				return false
			}
			if err := moveFile(src, dst); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: relocate %s: %v\n", p, err)
				return false
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: relocated leaked %s into worktree\n", p)
			relocated = append(relocated, p)
		case buildArtifacts[p]: // rebuilt release binary leaked → always discard
			// go/evolve is the marketplace-tracked binary, re-committed ONLY by the
			// release pipeline (releasepipeline.go) and reset to HEAD by the ship phase
			// (ship/gitops.go). A mid-cycle rebuild leaked into the main tree must be
			// discarded, never relocated into the worktree — relocating would commit
			// binary drift (the cycle-153 hazard). Discard regardless of status code.
			if err := discardMainLeak(ctx, projectRoot, p); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: %v\n", err)
				return false
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: discarded leaked rebuilt artifact %s\n", p)
		case strings.Contains(xy, "M"): // modified tracked file (exists at HEAD)
			// A non-Claude builder may edit an EXISTING tracked source file in the
			// MAIN tree instead of its worktree (cycle-162: orchestrator.go). That is
			// real builder work — preserve it by relocating the leaked content into the
			// worktree, but ONLY when the worktree has not independently modified the
			// same file: a divergent worktree edit is authoritative, and overlaying
			// would clobber it, so discard the main leak in that case.
			if worktreeCleanForPath(ctx, worktree, p) {
				if err := relocateTrackedEdit(ctx, projectRoot, worktree, p); err != nil {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: relocate tracked edit %s: %v\n", p, err)
					return false
				}
				fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: relocated leaked tracked edit %s into worktree\n", p)
				relocated = append(relocated, p)
			} else {
				if err := discardMainLeak(ctx, projectRoot, p); err != nil {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: %v\n", err)
					return false
				}
				fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: discarded leaked main-tree change %s (worktree diverged)\n", p)
			}
		case strings.ContainsAny(xy, "AD"): // added-not-at-HEAD / deleted tracked → discard (rare; conservative)
			if err := discardMainLeak(ctx, projectRoot, p); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: %v\n", err)
				return false
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: discarded leaked main-tree change %s\n", p)
		default: // rename/copy/unknown — not safe to auto-recover
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: unrecoverable leak status %q for %s (falling through to abort)\n", xy, p)
			return false
		}
	}
	if len(relocated) > 0 {
		// Stage ONLY the relocated files (not `git add -A`, which would also stage
		// unrelated worktree content and pollute the auditor's `git diff HEAD`),
		// so the relocated work is visible to audit + the binding — the same
		// visibility reason as normalizeWorktreeToBase. Use -f: a relocated path may
		// be gitignored in the WORKTREE (a builder that edited .gitignore, or a leak
		// that only main's status surfaced) — a plain `git add` exits 1 on an ignored
		// path and would abort the whole batch (cycle-176 / issue #11). The path is a
		// real leak we deliberately moved here for audit, so force-stage it.
		args := append([]string{"add", "-f", "--"}, relocated...)
		if _, c, e := gitCapture(ctx, worktree, args...); e != nil || c != 0 {
			// Fail loudly + return false: the files were physically relocated but
			// are NOT staged, so the auditor's `git diff HEAD` would not see them.
			// Returning false lets the tree-diff guard below abort cleanly rather
			// than ship a half-recovered, audit-invisible state.
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: git add of relocated paths failed (rc=%d): %v — aborting recovery\n", c, e)
			return false
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: %d leaked path(s) relocated into worktree; main tree restored\n", len(relocated))
	}
	return true
}

// moveFile relocates src→dst, falling back to copy+remove when os.Rename fails.
// os.Rename returns EXDEV when src and dst are on different filesystems — which
// happens when the worktree base resolves to a different volume than projectRoot
// (EVOLVE_WORKTREE_BASE / TMPDIR on another mount). Used by recoverBuildLeak, which
// operates at file granularity (-uall).
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// copyFile writes src's contents to dst (creating dst's parent dir), preserving src's
// file mode. Shared by moveFile's cross-filesystem fallback and relocateTrackedEdit.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if fi, serr := os.Stat(src); serr == nil {
		mode = fi.Mode()
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}

// relocateTrackedEdit moves a tracked-file edit that leaked into projectRoot back into
// the worktree: it copies the leaked content over the worktree's copy of p, then
// restores p in the main tree to HEAD (discarding the leak). The caller stages p in the
// worktree afterward (batched with the other relocated paths) so the auditor's
// `git diff HEAD` sees it. Only called when the worktree's copy of p is at HEAD, so the
// overlay never clobbers independent in-worktree work.
func relocateTrackedEdit(ctx context.Context, projectRoot, worktree, p string) error {
	if err := copyFile(filepath.Join(projectRoot, p), filepath.Join(worktree, p)); err != nil {
		return fmt.Errorf("relocate content of %s: %w", p, err)
	}
	return discardMainLeak(ctx, projectRoot, p) // restore main to HEAD; the worktree now holds the edit
}

// discardMainLeak restores p in projectRoot to HEAD, dropping a leaked change. `git
// checkout HEAD -- p` resets BOTH index and working tree, so it discards a staged-only
// ("M "/"A "/"D ") leak too (plain `git checkout -- p` no-ops a staged-only change).
func discardMainLeak(ctx context.Context, projectRoot, p string) error {
	// gitCapture returns a non-zero exit as (c != 0, e == nil); e is non-nil only on a
	// launch failure. Branch so we never wrap a nil error with %w (which would render
	// "<nil>" and break errors.Is/As on the result).
	_, c, e := gitCapture(ctx, projectRoot, "checkout", "HEAD", "--", p)
	if e != nil {
		return fmt.Errorf("git checkout HEAD -- %s: %w", p, e)
	}
	if c != 0 {
		return fmt.Errorf("git checkout HEAD -- %s: exit %d", p, c)
	}
	return nil
}

// worktreeCleanForPath reports whether the worktree's copy of p is unmodified from HEAD,
// so overlaying a relocated edit won't clobber independent in-worktree work. `git diff
// --quiet HEAD -- p` exits 0 (clean) / 1 (differs); any launch error is treated as
// "not clean" so the caller falls back to the conservative discard path.
func worktreeCleanForPath(ctx context.Context, worktree, p string) bool {
	_, c, e := gitCapture(ctx, worktree, "diff", "--quiet", "HEAD", "--", p)
	return e == nil && c == 0
}

// buildArtifacts are tracked build outputs a builder may rebuild into the main tree.
// go/evolve is the marketplace-tracked binary, re-committed ONLY by the release pipeline
// (releasepipeline.go) and reset to HEAD by the ship phase (ship/gitops.go); a mid-cycle
// rebuild leaked here must be DISCARDED, never relocated into the worktree (relocating
// would commit binary drift — cycle-153). go/bin/evolve is gitignored and normally never
// appears in `git status`, but is listed defensively.
var buildArtifacts = map[string]bool{
	"go/evolve":     true,
	"go/bin/evolve": true,
}

// gitCapture runs `git -C dir <args...>` and returns (stdout, exitCode, err).
// A non-zero exit is returned as exitCode with nil err (the caller decides
// whether it's fatal — e.g. `git diff HEAD` exit 1 means "differences", not a
// failure). Only a failure to launch git returns a non-nil err.
func gitCapture(ctx context.Context, dir string, args ...string) (string, int, error) {
	var buf strings.Builder
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr // surface git diagnostics (fatal: not a repo, …) for triage
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return buf.String(), ee.ExitCode(), nil
		}
		return "", -1, err
	}
	return buf.String(), 0, nil
}

// defaultGitDirtyPaths runs `git diff --name-only HEAD` in repoRoot and
// returns the list of dirty tracked paths (one per line). Workstream B's
// tree-diff guard uses this as a before/after snapshot — any path that
// becomes dirty during a source-writing phase is a leak that escaped the
// sandbox (each worktree is a separate working dir, so worktree writes don't
// appear here). Errors propagate so the guard can degrade to "snapshot
// missed" rather than misreport leaks.
func defaultGitDirtyPaths(ctx context.Context, repoRoot string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff", "--name-only", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only HEAD: %w", err)
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// finalizeOutcome translates SKIPPED into a more specific CycleOutcome label
// using HEAD movement and retro text as signals. PASS/FAIL/WARN pass through.
func (o *Orchestrator) finalizeOutcome(lastPhaseVerdict, retroDecision, preHEAD, postHEAD string) string {
	if lastPhaseVerdict != VerdictSKIPPED {
		return lastPhaseVerdict
	}
	// HEAD moved → something shipped inline (build calling `evolve ship --class manual`).
	if preHEAD != "" && postHEAD != "" && preHEAD != postHEAD {
		return CycleOutcomeShippedViaBuild
	}
	if strings.Contains(retroDecision, "would-have-blocked") {
		return CycleOutcomeSkippedAuditAdvisory
	}
	return CycleOutcomeSkippedUnknown
}

// phaseMaxAttempts bounds per-phase retries on a recoverable bridge
// ArtifactTimeout (Fix D). 2 = one relaunch after the first timeout; a
// deterministic timeout still aborts the cycle after the cap.
const phaseMaxAttempts = 2

func resolvePhaseMaxAttempts(env map[string]string) int {
	if env == nil {
		return phaseMaxAttempts
	}
	val := env["EVOLVE_PHASE_MAX_ATTEMPTS"]
	if val == "" {
		return phaseMaxAttempts
	}
	num, err := strconv.Atoi(val)
	if err != nil {
		return phaseMaxAttempts
	}
	if num > 5 {
		return 5
	}
	if num < 1 {
		return phaseMaxAttempts
	}
	return num
}

func resolveRetryBackoffBase(env map[string]string) int {
	if env == nil {
		return 5
	}
	val := env["EVOLVE_RETRY_BACKOFF_BASE_S"]
	if val == "" {
		return 5
	}
	num, err := strconv.Atoi(val)
	if err != nil {
		return 5
	}
	if num < 0 {
		return 0
	}
	return num
}

func executeRetryBackoff(attempt int, env map[string]string) {
	base := resolveRetryBackoffBase(env)
	if base <= 0 {
		return
	}
	nextAttempt := attempt + 1
	if nextAttempt < 2 {
		return
	}
	sleepSecs := base * (1 << (nextAttempt - 2))
	limitSecs := base
	if limitSecs < 30 {
		limitSecs = 30
	}
	if sleepSecs > limitSecs {
		sleepSecs = limitSecs
	}
	if sleepSecs > 0 {
		time.Sleep(time.Duration(sleepSecs) * time.Second)
	}
}


func isTransientBridgeError(err error) bool {
	return errors.Is(err, ErrTransientBridgeFailure)
}

func bridgeExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrArtifactTimeout) {
		return 81
	}
	errStr := err.Error()
	const target = "bridge: launch exit="
	idx := strings.Index(errStr, target)
	if idx != -1 {
		start := idx + len(target)
		end := start
		for end < len(errStr) && errStr[end] >= '0' && errStr[end] <= '9' {
			end++
		}
		if end > start {
			code, _ := strconv.Atoi(errStr[start:end])
			return code
		}
	}
	return 0
}

// maxRecoveryDepth bounds advisor-driven ship-error recovery per cycle
// (Component #5/#7). Ship is a pure executor: a structured ShipError is
// resolved by routing to a recovery phase (re-audit / retry-ship / debugger),
// not by aborting. This caps ship→recover→ship so a persistent blocker cannot
// loop forever; on exhaustion the orchestrator aborts loud with the accumulated
// ShipError. A safety invariant, not a flag (the outer safety<32 loop backstops).
const maxRecoveryDepth = 2

// phaseTimingEntry records per-phase latency + outcome for phase-timing.json.
type phaseTimingEntry struct {
	Phase        string  `json:"phase"`
	DurationMS   int64   `json:"duration_ms"`
	Verdict      string  `json:"verdict"`
	CostUSD      float64 `json:"cost_usd"`
	AttemptCount int     `json:"attempt_count"`
}

// phaseFailureDiag is the structured diagnostic written to <phase>-failure-diag.json
// when a mandatory phase aborts after exhausting all retry attempts.
type phaseFailureDiag struct {
	Phase        string `json:"phase"`
	Cycle        int    `json:"cycle"`
	ErrorMessage string `json:"error_message"`
	ExitCode     int    `json:"exit_code"`
	AttemptCount int    `json:"attempt_count"`
	Timestamp    string `json:"timestamp"`
}

// writePhaseFailureDiag writes a structured diagnostic file to
// <workspace>/<phase>-failure-diag.json. Best-effort: failures are logged to
// stderr but never mask the original error.
func writePhaseFailureDiag(workspace, phase string, cycle int, phaseErr error, attempts int, now func() time.Time) {
	exitCode := 1
	var exitErr *exec.ExitError
	if errors.Is(phaseErr, ErrArtifactTimeout) {
		exitCode = 81
	} else if errors.As(phaseErr, &exitErr) {
		exitCode = exitErr.ExitCode()
	}
	diag := phaseFailureDiag{
		Phase:        phase,
		Cycle:        cycle,
		ErrorMessage: phaseErr.Error(),
		ExitCode:     exitCode,
		AttemptCount: attempts,
		Timestamp:    now().UTC().Format(time.RFC3339),
	}
	data, merr := json.Marshal(diag)
	if merr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-diag marshal: %v\n", merr)
		return
	}
	path := filepath.Join(workspace, phase+"-failure-diag.json")
	tmp := path + ".tmp"
	if werr := os.WriteFile(tmp, data, 0o644); werr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-diag write: %v\n", werr)
		return
	}
	if rerr := os.Rename(tmp, path); rerr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-diag rename: %v\n", rerr)
	}
}

// RunCycle drives one cycle from PhaseStart to PhaseEnd, returning a
// summary of what ran. The lock is acquired up front and released on
// every exit path. State is updated incrementally so a crash leaves an
// inspectable trail in .evolve/.
func (o *Orchestrator) RunCycle(ctx context.Context, req CycleRequest) (CycleResult, error) {
	release, err := o.storage.AcquireLock(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("acquire lock: %w", err)
	}
	defer func() { _ = release() }()

	state, err := o.storage.ReadState(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("read state: %w", err)
	}
	cycle := state.LastCycleNumber + 1

	startedAt := o.now().UTC().Format(time.RFC3339)
	// IntentRequired is the gate for the start→intent vs start→scout
	// edge. Source priority: explicit Context["intent_required"]=="true"
	// from the caller > env EVOLVE_REQUIRE_INTENT=="1" > false. This
	// mirrors the bash dispatcher's check at run-cycle.sh:build_context.
	intentRequired := req.Context["intent_required"] == "true" ||
		req.Env["EVOLVE_REQUIRE_INTENT"] == "1"
	cs := CycleState{
		CycleID:        cycle,
		Phase:          string(PhaseStart),
		StartedAt:      startedAt,
		PhaseStartedAt: startedAt,
		WorkspacePath:  fmt.Sprintf("%s/.evolve/runs/cycle-%d", req.ProjectRoot, cycle),
		IntentRequired: intentRequired,
	}
	// Guard against workspace pollution from a prior killed attempt at
	// the same cycle number. If `<workspace>/` exists and has files,
	// rename to `<workspace>.polluted-<UTCnano>/` BEFORE any phase runs.
	// Without this, leftover scout-report.md / build-report.md from the
	// killed attempt cause Scout to short-circuit (read pre-existing
	// artifacts in seconds instead of redoing discovery) and steer
	// downstream phases via the OLD task selection.
	// Source incident: cycle-108 meta-loop attempts 1-4 (2026-05-26).
	// Opt-out via EVOLVE_DISABLE_WORKSPACE_GUARD=1 — used by tests that
	// pre-seed workspace files to simulate phase state.
	if req.Env["EVOLVE_DISABLE_WORKSPACE_GUARD"] != "1" && os.Getenv("EVOLVE_DISABLE_WORKSPACE_GUARD") != "1" {
		if err := archivePollutedWorkspace(cs.WorkspacePath, o.now); err != nil {
			// Best-effort: WARN but don't block the cycle; the failure
			// mode it prevents is bad-data steering, not safety.
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN workspace archive failed: %v\n", err)
		}
	}
	// Provision the per-cycle source worktree (ADR-0027): tdd/build write code
	// here, isolated from the live tree. cs.ActiveWorktree gates source writes
	// in the role-gate and drives worktree-aware ship. Best-effort — on failure
	// the source phases are denied by the role-gate (loud, not silent). Cleaned
	// up on cycle exit (after ship has merged the worktree→main).
	// worktreeBaseSHA is the worktree HEAD at creation == the cycle base. After
	// the build phase we soft-reset to it so a committing builder's work becomes
	// pending again (see normalizeWorktreeToBase + the cycle-156 incident).
	var worktreeBaseSHA string
	// Full main-tree dirty baseline (tracked + untracked) captured BEFORE any
	// phase runs. recoverBuildLeak (cycle-160 / Option A) subtracts it so it only
	// relocates paths the build introduced, never the operator's pre-existing work.
	mainDirtyBaseline := porcelainDirtySet(ctx, req.ProjectRoot)
	if wtPath, werr := o.worktree.Create(req.ProjectRoot, cycle); werr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree provisioning failed (source phases will be blocked): %v\n", werr)
	} else {
		cs.ActiveWorktree = wtPath
		if base, _, berr := gitCapture(ctx, wtPath, "rev-parse", "HEAD"); berr == nil {
			worktreeBaseSHA = strings.TrimSpace(base)
		} else {
			// Fail loudly: an empty base disables the cycle-156 normalize, so a
			// committing builder's work would again be discarded by the audit —
			// the exact symptom Option C fixes. WARN rather than abort (the
			// source phases still run; normalize just degrades to a no-op).
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree-normalize: rev-parse HEAD at worktree creation failed: %v (build-commit normalize disabled this cycle)\n", berr)
		}
		defer func() { _ = o.worktree.Cleanup(req.ProjectRoot, wtPath) }()
	}
	if err := o.storage.WriteCycleState(ctx, cs); err != nil {
		return CycleResult{}, fmt.Errorf("init cycle-state: %w", err)
	}

	// One snapshot per cycle — operator mutation post-call must not
	// retroactively change what phases saw.
	envSnap := make(map[string]string, len(req.Env))
	for k, v := range req.Env {
		envSnap[k] = v
	}
	ctxSnap := make(map[string]string, len(req.Context))
	for k, v := range req.Context {
		ctxSnap[k] = v
	}

	// PR 6 (cycle-135 followup): mint the cycle's challenge token here —
	// ONCE per cycle, at orchestrator start, BEFORE any phase runs. Surface
	// it to every phase via Context["challengeToken"] (scout's ComposePrompt
	// reads it at scout.go:64) AND persist it to <workspace>/challenge-
	// token.txt so the agent-templates.md PR 5 fallback source is populated.
	// Pre-PR-6, no Go code injected the token; scout invented its own
	// (cycle 134 audit C1: "no-token-manual-run-cycle-134"; cycle 135 audit
	// C1: scout minted `59576594e2e8d5c3` instead of using `5b96ecb69a0c848f`
	// from challenge-token.txt). The mint is the same 8-byte-hex shape as
	// bridge.defaultChallengeToken so post-cycle ledger entries are
	// indistinguishable from the bridge-minted ones used pre-cycle-135.
	if _, alreadySet := ctxSnap["challengeToken"]; !alreadySet {
		var tokBytes [8]byte
		if _, err := rand.Read(tokBytes[:]); err == nil {
			tok := hex.EncodeToString(tokBytes[:])
			ctxSnap["challengeToken"] = tok
			// Best-effort workspace write — phase agents per agent-templates.md
			// PR 5 read this as fallback source #2 when inputs.challengeToken
			// is empty. Failure is logged but not fatal (the Context path is
			// the primary route; phases that can't read the file just rely on
			// Context).
			_ = os.MkdirAll(cs.WorkspacePath, 0o755)
			if err := os.WriteFile(filepath.Join(cs.WorkspacePath, "challenge-token.txt"), []byte(tok+"\n"), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN challenge-token.txt write failed: %v (Context route still works)\n", err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN challenge token mint failed: %v (phase agents will fall back to their own protocol)\n", err)
		}
	}

	// Capture HEAD before any phase so finalizeOutcome can detect mid-cycle commits.
	preCycleHEAD, _ := o.gitHEAD()

	// Upfront whole-cycle plan (ADR-0024 §2). At Stage>=Advisory with a planner,
	// ask the advisor once which phases to run, CLAMP the answer to the integrity
	// floor (ship⇒build∧audit∧tdd), persist it, and thread the clamped plan into
	// every routing decision below. The clamp is the non-bypassable kernel floor:
	// it can only COMPLETE the ship-chain, never weaken it, so a hallucinated or
	// adversarial plan cannot reach ship without a real build+audit. Any
	// failure leaves clampedPlan nil ⇒ routing falls back to the configurable
	// never-skip spine (fail-safe to static). Below Advisory, no plan is computed.
	// This is the SINGLE gate for the upfront plan: Stage>=Advisory (the advisor
	// drives) AND Mode==DynamicLLM (static mode makes no LLM calls) AND a planner
	// is wired. The composition root passes WithPlanner unconditionally; the
	// Mode check lives here so the invariant ("LLM plan iff DynamicLLM+Advisory")
	// has one source of truth rather than two gates that could drift.
	var clampedPlan *router.PhasePlan
	if o.cfg.Stage >= config.StageAdvisory && o.cfg.Mode == config.ModeDynamicLLM && o.planner != nil {
		planIn := router.RouteInput{
			Current:         string(PhaseStart),
			Signals:         router.RoutingSignals{}, // no handoffs exist yet at cycle start
			Cfg:             o.cfg,
			BudgetRemaining: req.Budget.MaxUSD,
			Now:             o.now(),
			Workspace:       cs.WorkspacePath,
			ProjectRoot:     req.ProjectRoot,
			Cycle:           cycle,
			Env:             envSnap,
		}
		// ClampPlanToFloor's tddPinned reads planIn.Signals, empty here (no
		// handoffs yet) — cycle_size!="trivial" evaluates true, so tdd is pinned on
		// the conservative (more-mandatory) side at plan time.
		if raw, perr := o.planner.Plan(planIn); perr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase advisor Plan failed (degrading to static spine): %v\n", perr)
		} else if raw != nil {
			var clamps []router.Clamp
			clampedPlan, clamps = router.ClampPlanToFloor(planIn, raw)
			o.recordPhasePlan(ctx, cycle, cs, clampedPlan, clamps)
		}
	}

	result := CycleResult{Cycle: cycle, FinalVerdict: VerdictPASS}
	var phaseTimings []phaseTimingEntry
	// Deferred write of phase-timing.json runs even when RunCycle returns an
	// error so partial timing data is preserved for operator inspection.
	defer func() {
		if len(phaseTimings) == 0 {
			return
		}
		timingPath := filepath.Join(cs.WorkspacePath, "phase-timing.json")
		data, merr := json.Marshal(phaseTimings)
		if merr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-timing marshal: %v\n", merr)
			return
		}
		tmp := timingPath + ".tmp"
		if werr := os.WriteFile(tmp, data, 0o644); werr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-timing write: %v\n", werr)
			return
		}
		if rerr := os.Rename(tmp, timingPath); rerr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-timing rename: %v\n", rerr)
		}
	}()
	current := PhaseStart
	lastVerdict := VerdictPASS
	// scheduledNext, when non-empty, overrides the state machine for
	// the next iteration. Set by the retro branch to inject the
	// failure-adapter's decision.
	var scheduledNext Phase
	// routingSeq names the per-cycle routing-decision artifacts
	// (routing-decision-<seq>.json). Incremented only when routing runs.
	routingSeq := 0
	// recoveryDepth bounds advisor-driven ship-error recovery across the whole
	// cycle (maxRecoveryDepth). Persists across loop iterations.
	recoveryDepth := 0

	// Bounded loop guards against any transition-table cycle bug.
	for safety := 0; safety < 32; safety++ {
		var next Phase
		// fromSchedule marks an iteration whose `next` came from scheduledNext —
		// an authoritative injection by the retro branch, the ship-error recovery
		// seam, or the debugger decision. The dynamic-routing override
		// (enforceNext) must NOT second-guess such a transition, so it is gated on
		// !fromSchedule (generalizing the prior current!=PhaseRetro guard).
		fromSchedule := false
		switch {
		case scheduledNext != "":
			next = scheduledNext
			scheduledNext = ""
			fromSchedule = true
		case current == PhaseStart:
			// First edge is gated by intent_required, not by verdict.
			next = o.sm.NextFromStart(cs.IntentRequired)
		case !current.IsValid():
			// current is a user-defined phase (only reachable when dynamic
			// routing selected it). The static successor is simply the next
			// entry in the configured order; the routing block below refines it.
			next = o.nextInOrder(current)
		default:
			n, err := o.sm.Next(current, lastVerdict)
			if err != nil {
				return result, fmt.Errorf("transition from %s: %w", current, err)
			}
			next = n
		}

		// Dynamic routing (shadow → advisory → enforce). Stage:Off — the
		// default — leaves the static state machine fully in control: no
		// digest, no ledger entry, byte-identical to legacy. When enabled,
		// digest the completed handoffs, ask the Strategy for a decision,
		// record it forensically, and — at Advisory and above — let the clamped
		// decision override the static successor, re-validated against the
		// legality oracle (CanTransition) AND the artifact-backed spine gate
		// (SpineSatisfiedUpTo). Retro keeps its failure-adapter shim
		// (decideAfterRetro) while routing is bedded in, so routing never
		// overrides the retro branch. The configurable mandatory set
		// (cfg.Mandatory) decides which phases are never-skip; the integrity
		// floor (ClampPlanToFloor, applied to the upfront plan) decides what ship
		// still requires regardless of how small the operator makes that set.
		if o.cfg.Stage != config.StageOff {
			routingSeq++
			signals, _ := router.Digest(cs.WorkspacePath, cs.CompletedPhases)
			dec := o.strategy.Decide(router.RouteInput{
				Current:         string(current),
				Verdict:         lastVerdict,
				Signals:         signals,
				History:         entriesFromRecords(state.FailedAt),
				Cfg:             o.cfg,
				BudgetRemaining: req.Budget.MaxUSD,
				Completed:       cs.CompletedPhases,
				Strict:          envSnap["EVOLVE_STRICT_AUDIT"] == "1",
				Now:             o.now(),
				// Proposer context (DynamicLLM only; ignored by pure Route).
				Workspace:   cs.WorkspacePath,
				ProjectRoot: req.ProjectRoot,
				Cycle:       cycle,
				Env:         envSnap,
				// Clamped whole-cycle plan (Stage>=Advisory). nil below Advisory
				// or on planner failure ⇒ shouldRun runs the legacy trigger path.
				Plan: clampedPlan,
			})
			if o.cfg.Stage >= config.StageAdvisory && !fromSchedule {
				if forced, ok := o.enforceNext(current, next, signals, dec); ok {
					next = forced
				}
				// Full spine-integrity check on the SELECTED next (static OR
				// override). Fail-open: a missing mandatory-predecessor handoff
				// is a loud WARN recorded in the decision, never a block —
				// Digest is fail-open, so an absent artifact may be a read miss
				// rather than a real gap, and false-blocking a real cycle is
				// worse than surfacing the signal. The override path already
				// declines (blocks) divergent edges that fail this check; here
				// we additionally surface it for the trusted static edge.
				if next != PhaseEnd && !o.sm.SpineSatisfiedUpTo(next, signals, o.cfg) {
					dec.Clamps = append(dec.Clamps, router.Clamp{
						Rule:     "spine-unsatisfied-warn",
						Proposed: string(next),
						Forced:   string(next),
					})
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN spine not satisfied for next=%s (a mandatory predecessor's handoff artifact is missing); proceeding fail-open\n", next)
				}
			}
			o.recordRoutingDecision(ctx, cycle, cs, routingSeq, dec)
		}

		if next == PhaseEnd {
			break
		}

		runner, ok := o.runners[next]
		if !ok {
			return result, fmt.Errorf("%w: no runner registered for phase %s", ErrPhaseInvalid, next)
		}

		phaseStarted := o.now().UTC()
		cs.Phase = string(next)
		cs.PhaseStartedAt = phaseStarted.Format(time.RFC3339)
		cs.ActiveAgent = string(next)
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			return result, fmt.Errorf("write cycle-state pre-%s: %w", next, err)
		}

		// Phases that run with cwd=worktree: source writers (tdd/build) so their
		// code writes land in the isolated worktree the role-gate permits, AND the
		// audit phase (read-only) so its verification commands inspect the builder's
		// pending work there instead of the main tree (issue #9). Every other phase
		// writes only its artifact to the absolute workspace.
		phaseWorktree := ""
		if o.runsInWorktree(next) {
			phaseWorktree = cs.ActiveWorktree
		}
		// Workstream B: snapshot the main-tree dirty set BEFORE a source-
		// writing phase runs. After it runs we re-snapshot and compare —
		// any newly-dirty MAIN-tree path is a leak that escaped the bridge
		// sandbox (each git worktree is a separate working dir, so its
		// writes don't show up here). The treediff package owns the
		// snapshot/check + SnapshotMissed semantics; the orchestrator just
		// threads it through. Skipped entirely for non-worktree phases.
		var (
			treeGuard      *treediff.Guard
			beforeDirty    []string
			snapshotFailed bool
		)
		if phaseWorktree != "" && o.gitDirtyPaths != nil {
			treeGuard = treediff.New(o.gitDirtyPaths)
			snap, err := treeGuard.Snapshot(ctx, req.ProjectRoot)
			if err != nil {
				snapshotFailed = true
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff pre-phase snapshot failed for %s: %v (sandbox guard degraded; post-phase leak check skipped)\n", next, err)
			} else {
				beforeDirty = snap
			}
		}
		phaseReq := PhaseRequest{
			Cycle:         cycle,
			ProjectRoot:   req.ProjectRoot,
			Workspace:     cs.WorkspacePath,
			Worktree:      phaseWorktree,
			GoalHash:      req.GoalHash,
			Budget:        req.Budget,
			PreviousPhase: string(current),
			Env:           envSnap,
			Context:       ctxSnap,
		}
		// Cycle-122 Fix 3 / ADR-0030: attach the per-phase observer
		// goroutine BEFORE runner.Run and cancel it AFTER. noopObserver
		// (default when WithObserver wasn't used) is byte-identical to
		// the pre-fix cycle. Real implementations spawn a stall detector
		// that watches <workspace>/<agent>-stdout.log and emits stall
		// events to <workspace>/<agent>-observer-events.ndjson.
		// Self-heal (Fix D): a bridge ArtifactTimeout (exit=81) is the
		// recoverable "agent produced no artifact within the wait window" case
		// — a stalled launch where a fresh relaunch usually succeeds. Retry the
		// phase a bounded number of times on THAT sentinel only; every other
		// error (and exhaustion of the budget) aborts the cycle as before. A
		// deterministic timeout (e.g. a misconfigured agent) simply fails again
		// and aborts after the cap — at most one wasted retry. The observer is
		// (re)started per attempt so each launch is watched.
		var resp PhaseResponse
		var err error
		// shipRecovered marks that a ShipError was intercepted and routed to a
		// recovery phase instead of aborting; the post-loop guard then continues
		// the outer loop (skipping verdict/ledger handling for the failed ship).
		shipRecovered := false
		maxAttempts := resolvePhaseMaxAttempts(phaseReq.Env)
		var attemptCount int
		for attempt := 1; ; attempt++ {
			attemptCount = attempt
			obsCancel := o.observer.Start(ctx, string(next), phaseReq)
			resp, err = runner.Run(ctx, phaseReq)
			if obsCancel != nil {
				obsCancel()
			}
			if err == nil && IsVerdict(resp.Verdict) {
				break
			}
			if err != nil {
				if attempt >= maxAttempts || (!errors.Is(err, ErrArtifactTimeout) && !isTransientBridgeError(err)) {
					// Backfill: when exhaustion is specifically due to ErrArtifactTimeout,
					// try to reconstruct the artifact from stdout.clean.txt before aborting.
					// Enabled by default (EVOLVE_BACKFILL_ENABLED != 0).
					if attempt >= maxAttempts && errors.Is(err, ErrArtifactTimeout) &&
						envSnap["EVOLVE_BACKFILL_ENABLED"] != "0" && os.Getenv("EVOLVE_BACKFILL_ENABLED") != "0" {
						artifactPath := backfillArtifactPath(cs.WorkspacePath, string(next))
						if ok, berr := backfill.TryExtract(cs.WorkspacePath, string(next), artifactPath, 200); berr != nil {
							fmt.Fprintf(os.Stderr, "[orchestrator] WARN backfill %s: %v\n", next, berr)
						} else if ok {
							fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s: ErrArtifactTimeout exhausted; backfilled artifact from stdout.clean.txt; proceeding with WARN verdict\n", next)
							resp = PhaseResponse{Phase: string(next), Verdict: VerdictWARN, ArtifactsDir: cs.WorkspacePath}
							err = nil
							if lerr := o.ledger.Append(ctx, LedgerEntry{
								TS:       o.now().UTC().Format(time.RFC3339),
								Cycle:    cycle,
								Role:     string(next),
								Kind:     "backfill",
								ExitCode: 81,
							}); lerr != nil {
								fmt.Fprintf(os.Stderr, "[orchestrator] WARN backfill ledger append: %v\n", lerr)
							}
							break
						}
					}
					// Ship-error recovery seam (Component #7): ship is a pure
					// executor — a structured ShipError is resolved by the advisor's
					// recovery chain (Strategy + CoR), not by aborting the cycle. The
					// resolver records the error, picks the recovery phase
					// (re-audit / retry-ship / debugger), and bounds the depth.
					// Integrity breaches, an illegal edge, or exhausted depth return
					// (_, false) and fall through to the loud abort below.
					if se, ok := AsShipError(err); ok {
						if rec, recovering := o.recoverFromShipError(ctx, cycle, cs, se, recoveryDepth); recovering {
							ctxSnap["ship_error_code"] = string(se.Code)
							ctxSnap["ship_error_class"] = string(se.Class)
							ctxSnap["ship_error_stage"] = string(se.Stage)
							ctxSnap["ship_error_debug"] = se.DebugString()
							recoveryDepth++
							scheduledNext = rec
							current = PhaseShip // ship ran (and failed); keep forensics accurate
							shipRecovered = true
							break
						}
					}
					writePhaseFailureDiag(cs.WorkspacePath, string(next), cycle, err, attempt, o.now)
					return result, fmt.Errorf("phase %s: %w", next, err)
				}
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s attempt %d/%d hit a transient bridge error or timeout; relaunching (self-heal)\n", next, attempt, maxAttempts)
				// Emit structured audit trail for the self-heal retry.
				if lerr := o.ledger.Append(ctx, LedgerEntry{
					TS:       o.now().UTC().Format(time.RFC3339),
					Cycle:    cycle,
					Role:     string(next),
					Kind:     "phase_retry",
					ExitCode: bridgeExitCode(err),
				}); lerr != nil {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_retry ledger append: %v\n", lerr)
				}
				executeRetryBackoff(attempt, phaseReq.Env)
				continue
			}
			if err == nil && !IsVerdict(resp.Verdict) {
				if attempt >= maxAttempts {
					writePhaseFailureDiag(cs.WorkspacePath, string(next), cycle, fmt.Errorf("phase %s returned non-canonical verdict %q", next, resp.Verdict), attempt, o.now)
					return result, fmt.Errorf("phase %s returned non-canonical verdict %q", next, resp.Verdict)
				}
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s attempt %d/%d returned non-canonical verdict %q; relaunching\n", next, attempt, maxAttempts, resp.Verdict)
				// Emit structured audit trail for the self-heal retry.
				if lerr := o.ledger.Append(ctx, LedgerEntry{
					TS:       o.now().UTC().Format(time.RFC3339),
					Cycle:    cycle,
					Role:     string(next),
					Kind:     "phase_retry",
					ExitCode: 0,
				}); lerr != nil {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_retry ledger append: %v\n", lerr)
				}
				executeRetryBackoff(attempt, phaseReq.Env)
				continue
			}
		}
		if shipRecovered {
			continue // run the recovery phase (scheduledNext) next iteration
		}

		// Workstream E2: per-phase deliverable review gate. Runs ONLY for
		// non-SKIPPED verdicts (a SKIPPED phase produced no deliverable to
		// review) and BEFORE the tree-diff guard + ledger append, so a reject
		// aborts the cycle without recording the phase as a success. The
		// default reviewer is noopReviewer (every phase approved) so opt-out
		// is byte-identical to pre-E2. Retry/N is a follow-up — today reject
		// = abort.
		if o.reviewer != nil && resp.Verdict != VerdictSKIPPED {
			rin := ReviewInput{
				Phase:       string(next),
				Response:    resp,
				Workspace:   cs.WorkspacePath,
				Worktree:    phaseWorktree,
				ProjectRoot: req.ProjectRoot,
			}
			rr := o.reviewer.Review(ctx, rin)
			if !rr.Approve {
				return result, fmt.Errorf("review gate: phase %q deliverable rejected: %s", next, rr.Reason)
			}
		}

		// Workstream B: post-phase tree-diff check. Runs BEFORE the ledger
		// append so a leak aborts the cycle without recording the phase as a
		// success. Snapshot failures (pre OR post) degrade silently — the
		// guard is belt-and-suspenders to the OS sandbox, so a transient git
		// read error must never cause a false abort.
		// Cycle-160 fix (Option A): a non-Claude builder (agy/codex in tmux) is
		// not bound by the Claude-only role-gate, and the OS sandbox is off on
		// nested-macOS, so it can write build output to the MAIN tree instead of
		// its worktree. Relocate any such leak into the worktree (staged, so audit
		// sees it) and restore main BEFORE the tree-diff guard runs. Runs
		// unconditionally after build (no-op when clean) because the guard's
		// `git diff --name-only HEAD` baseline is tracked-only and misses
		// pure-untracked leaks. On recovery FAILURE we abort explicitly — the
		// tree-diff guard only backstops tracked leaks, so a failed recovery
		// of an untracked leak must not slip past into audit.
		if next == PhaseBuild && cs.ActiveWorktree != "" {
			if !recoverBuildLeak(ctx, req.ProjectRoot, cs.ActiveWorktree, mainDirtyBaseline) {
				return result, fmt.Errorf("phase build: worktree-leak recovery failed (main tree left unsafe for audit)")
			}
		}

		if treeGuard != nil && !snapshotFailed {
			res := treeGuard.Check(ctx, req.ProjectRoot, beforeDirty)
			if res.SnapshotMissed {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff post-phase snapshot failed for %s (sandbox guard degraded; not aborting)\n", next)
			} else if !res.OK() {
				return result, res.Error(string(next), phaseWorktree)
			}
		}

		if err := o.ledger.Append(ctx, LedgerEntry{
			TS:       o.now().UTC().Format(time.RFC3339),
			Cycle:    cycle,
			Role:     string(next),
			Kind:     "phase",
			ExitCode: 0,
		}); err != nil {
			return result, fmt.Errorf("ledger append for %s: %w", next, err)
		}

		// Audit-binding (root-cause fix, 2026-05-29): ship's verifyAuditBinding
		// looks for the latest role=auditor kind=agent_subprocess ledger entry
		// carrying git_head + tree_state_sha + artifact SHA. The Go orchestrator
		// otherwise records audit only as kind:phase (no binding fields), so ship
		// fell back to an ancient bash-era entry and EVERY cycle failed
		// AUDIT_BINDING_HEAD_MOVED. Emit the rich binding entry after a shippable
		// audit so ship binds to THIS cycle.
		if next == PhaseAudit && (resp.Verdict == VerdictPASS || resp.Verdict == VerdictWARN) {
			o.recordAuditBinding(ctx, cycle, req.ProjectRoot, cs.WorkspacePath, cs.ActiveWorktree, resp.Verdict)
		}

		// Cycle-156 fix (Option C): a committing builder (e.g. agy/Gemini
		// following evolve-builder.md:235) leaves its work in a worktree
		// commit, but audit + binding inspect `git diff HEAD` (empty after a
		// commit). Soft-reset the build's commits to the cycle base so the
		// work is pending again before audit runs (next iteration). No-op for
		// non-committing builders. See the cycle-156 incident doc.
		if next == PhaseBuild && cs.ActiveWorktree != "" {
			normalizeWorktreeToBase(ctx, cs.ActiveWorktree, worktreeBaseSHA)
		}

		cs.CompletedPhases = append(cs.CompletedPhases, string(next))
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			return result, fmt.Errorf("write cycle-state post-%s: %w", next, err)
		}

		result.PhasesRun = append(result.PhasesRun, next)
		result.FinalVerdict = resp.Verdict
		phaseTimings = append(phaseTimings, phaseTimingEntry{
			Phase:        string(next),
			DurationMS:   resp.DurationMS,
			Verdict:      resp.Verdict,
			CostUSD:      resp.CostUSD,
			AttemptCount: attemptCount,
		})
		current = next
		lastVerdict = resp.Verdict

		// Retro is the one phase whose successor isn't verdict-driven;
		// the failure-adapter consults cycle history (state.FailedAt) and
		// the retro verdict to pick {ship | tdd | end}. Set scheduledNext
		// so the next loop iteration runs the chosen phase.
		if current == PhaseRetro {
			branch, extraEnv, reason := o.decideAfterRetro(resp.Verdict, state.FailedAt)
			for k, v := range extraEnv {
				envSnap[k] = v
			}
			result.RetroDecision = reason
			if branch == PhaseEnd {
				break
			}
			if !o.sm.CanTransition(PhaseRetro, branch) {
				return result, fmt.Errorf("retro→%s not allowed by state machine", branch)
			}
			scheduledNext = branch
		}

		// The debugger phase is decision-driven (RESHIP / RERUN_PHASE / BLOCK),
		// not verdict-driven — mirror the retro branch. The debugger runner
		// surfaces its decision on PhaseResponse.Signals; decideAfterDebugger
		// maps it to the next phase, which the next iteration runs via
		// scheduledNext (bypassing the routing override, like retro).
		if current == PhaseDebugger {
			branch := o.decideAfterDebugger(resp)
			o.recordDebuggerDecision(ctx, cycle, cs, resp)
			if branch == PhaseEnd {
				break
			}
			if !o.sm.CanTransition(PhaseDebugger, branch) {
				return result, fmt.Errorf("debugger→%s not allowed by state machine", branch)
			}
			scheduledNext = branch
		}
	}

	postCycleHEAD, _ := o.gitHEAD()
	result.FinalVerdict = o.finalizeOutcome(result.FinalVerdict, result.RetroDecision, preCycleHEAD, postCycleHEAD)

	// Notice the silent no-ship (Fix C): the cycle ran phases but ended without
	// HEAD advancing and without an audit-advisory "would-have-blocked" record —
	// i.e. work may have been produced and then discarded with the worktree
	// (cycle-148: a genuine PASS mis-graded FAIL routed audit→retro→end). The
	// outcome label alone is advisory and easily missed in a batch summary, so
	// surface it loudly here. Not an error — some cycles legitimately produce no
	// change — but always worth an operator's eyes.
	if result.FinalVerdict == CycleOutcomeSkippedUnknown {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d ended without shipping (%s): phases ran but HEAD did not advance and no audit-advisory block was recorded — any worktree changes were discarded. Inspect %s (audit-report.md verdict + acs-verdict.json red_count).\n", cycle, CycleOutcomeSkippedUnknown, cs.WorkspacePath)
	}

	state.LastCycleNumber = cycle
	if err := o.storage.WriteState(ctx, state); err != nil {
		return result, fmt.Errorf("write state: %w", err)
	}
	return result, nil
}

// decideAfterRetro consults the failure-adapter over cycle history
// (state.failedApproaches) to pick the post-retro branch.
//
// Mapping (retro verdict × failureadapter action → next phase):
//   - retro PASS               → ship   (retrospective recovered the cycle)
//   - retro FAIL/WARN + BLOCK-* → end    (cycle history forbids further work)
//   - retro FAIL/WARN + RETRY  → tdd    (retry from earlier phase w/ fallback env)
//   - retro FAIL/WARN + PROCEED → end   (no recovery, no block — exit cleanly)
//
// Returned reason is "<action>: <failureadapter reason>" for the
// CycleResult.RetroDecision audit field.
func (o *Orchestrator) decideAfterRetro(retroVerdict string, history []FailedRecord) (next Phase, extraEnv map[string]string, reason string) {
	// retro PASS → ship; no failureadapter consultation.
	if retroVerdict == VerdictPASS {
		return PhaseShip, nil, "retro-recovered: ship"
	}
	entries := entriesFromRecords(history)
	dec := failureadapter.Decide(entries, failureadapter.Options{Now: o.now()})
	switch dec.Action {
	case failureadapter.ActionRetryWithFallback:
		return PhaseTDD, dec.SetEnv, "retry-with-fallback: " + dec.Reason
	case failureadapter.ActionBlockCode, failureadapter.ActionBlockOperatorAction:
		return PhaseEnd, nil, string(dec.Action) + ": " + dec.Reason
	default: // ActionProceed
		return PhaseEnd, dec.SetEnv, "proceed: " + dec.Reason
	}
}

// recoverFromShipError resolves a ship-phase ShipError via the advisor's
// recovery chain (Strategy + Chain-of-Responsibility, Component #6/#7). Ship is
// a pure executor: it never rejects a cycle, it returns a structured error and
// the orchestrator decides what to do. This records the error for forensics,
// then asks the strategy's Recover() for the recovery phase. Returns
// (phase, true) to proceed with recovery, or ("", false) to abort the cycle:
//   - depth >= maxRecoveryDepth  → exhausted, abort loud
//   - recovery routes to end     → integrity breach / unmapped, abort loud
//   - illegal ship→cand edge     → defensive abort
//
// Recovery is structural (always available via StaticPreset.Recover) and so runs
// regardless of the dynamic-routing Stage — it is error handling, not routing.
func (o *Orchestrator) recoverFromShipError(ctx context.Context, cycle int, cs CycleState, se *ShipError, depth int) (Phase, bool) {
	o.recordShipError(ctx, cycle, cs, se)
	if depth >= maxRecoveryDepth {
		fmt.Fprintf(os.Stderr, "[orchestrator] ship recovery exhausted after %d attempt(s) (%s/%s); aborting\n", depth, se.Code, se.Class)
		return "", false
	}
	// Recovery is deterministic Chain-of-Responsibility (no LLM); both routing
	// strategies just delegate to the pure router.Recover, so call it directly.
	// This keeps recovery available even when no routing Strategy was wired
	// (e.g. Stage:Off) — error handling must not depend on routing being on.
	dec := router.Recover(router.RouteInput{
		Blocker: &router.Blocker{
			Code:  string(se.Code),
			Class: string(se.Class),
			Stage: string(se.Stage),
		},
	})
	cand := o.candidatePhase(dec.NextPhase)
	if cand == "" || cand == PhaseEnd {
		fmt.Fprintf(os.Stderr, "[orchestrator] ship error %s (%s) is unrecoverable (%s); aborting\n", se.Code, se.Class, dec.Reason)
		return "", false
	}
	if !o.sm.CanTransition(PhaseShip, cand) {
		fmt.Fprintf(os.Stderr, "[orchestrator] ship recovery proposed illegal edge ship→%s (%s); aborting\n", cand, dec.Reason)
		return "", false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] ship error %s (%s) → recovery routes to %s (%s)\n", se.Code, se.Class, cand, dec.Reason)
	return cand, true
}

// decideAfterDebugger maps the debugger phase's recovery decision (surfaced on
// PhaseResponse.Signals by the debugger runner) to the next phase, mirroring
// decideAfterRetro. RESHIP→ship; RERUN_PHASE→the named phase (defaulting to
// audit); BLOCK/empty/unknown→end. A malformed decision already safe-defaulted
// to BLOCK in the debugger's Classify, so this conservatively ends on anything
// not explicitly RESHIP/RERUN_PHASE.
func (o *Orchestrator) decideAfterDebugger(resp PhaseResponse) Phase {
	action, _ := resp.Signals["debugger.action"].(string)
	switch action {
	case "RESHIP":
		return PhaseShip
	case "RERUN_PHASE":
		// Clamp rerun targets to UPSTREAM phases (audit/build/tdd) — re-shipping
		// is the dedicated RESHIP action, so a "rerun_phase: ship" must not become
		// a reship that skips re-establishing the precondition. An unrecognized or
		// non-upstream target defaults to audit, the dominant binding-recovery
		// target. (Defense-in-depth: the loop's CanTransition gate independently
		// rejects illegal edges.)
		rerun, _ := resp.Signals["debugger.rerun_phase"].(string)
		switch o.candidatePhase(rerun) {
		case PhaseAudit:
			return PhaseAudit
		case PhaseBuild:
			return PhaseBuild
		case PhaseTDD:
			return PhaseTDD
		default:
			return PhaseAudit
		}
	default: // BLOCK, empty, unknown
		return PhaseEnd
	}
}

// recordShipError persists a ShipError to <workspace>/ship-error.json and
// appends a hash-bound ship_error ledger entry (Component #6 forensics). The
// tamper-evident trail lets the failure-adapter and operators see every
// auto-recovery. Best-effort: a marshal/write/append failure WARNs and is
// swallowed — forensics must never compound a ship failure into a cycle abort.
func (o *Orchestrator) recordShipError(ctx context.Context, cycle int, cs CycleState, se *ShipError) {
	ts := o.now().UTC().Format(time.RFC3339)
	artifactPath := filepath.Join(cs.WorkspacePath, "ship-error.json")
	sha := ""
	payload := map[string]string{
		"code":    string(se.Code),
		"class":   string(se.Class),
		"stage":   string(se.Stage),
		"message": se.Message,
		"debug":   se.DebugString(),
	}
	if buf, err := json.MarshalIndent(payload, "", "  "); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN ship-error marshal: %v\n", err)
		artifactPath = ""
	} else if err := os.MkdirAll(cs.WorkspacePath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN ship-error mkdir: %v\n", err)
		artifactPath = ""
	} else if err := os.WriteFile(artifactPath, buf, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN ship-error write: %v\n", err)
		artifactPath = ""
	} else {
		sum := sha256.Sum256(buf)
		sha = hex.EncodeToString(sum[:])
	}
	if err := o.ledger.Append(ctx, LedgerEntry{
		TS: ts, Cycle: cycle, Role: "ship", Kind: "ship_error",
		ExitCode: 1, ArtifactPath: artifactPath, ArtifactSHA256: sha,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN ship_error ledger append: %v\n", err)
	}
}

// recordDebuggerDecision appends a hash-bound debugger_decision ledger entry
// pointing at the debugger's debug-decision.json artifact (Component #6
// forensics). Best-effort: failures WARN and are swallowed.
func (o *Orchestrator) recordDebuggerDecision(ctx context.Context, cycle int, cs CycleState, _ PhaseResponse) {
	// The action + root_cause live in the debug-decision.json artifact; the
	// ledger entry binds its SHA so the decision is tamper-evident without
	// duplicating the payload into a field LedgerEntry does not have.
	artifactPath := filepath.Join(cs.WorkspacePath, "debug-decision.json")
	sha := ""
	if buf, err := os.ReadFile(artifactPath); err == nil {
		sum := sha256.Sum256(buf)
		sha = hex.EncodeToString(sum[:])
	} else {
		artifactPath = ""
	}
	if err := o.ledger.Append(ctx, LedgerEntry{
		TS: o.now().UTC().Format(time.RFC3339), Cycle: cycle, Role: "debugger",
		Kind: "debugger_decision", ExitCode: 0, ArtifactPath: artifactPath, ArtifactSHA256: sha,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN debugger_decision ledger append: %v\n", err)
	}
}

// recordRoutingDecision marshals the RouterDecision to
// <workspace>/routing-decision-<seq>.json and appends a hash-bound
// routing_decision ledger entry, plus one phase_skipped entry per declined
// optional phase (preserving the PSMAS resume/audit-binding contract).
//
// Best-effort: a marshal/write/append failure WARNs and is swallowed —
// routing forensics must never abort a cycle. Called only when Stage != Off,
// so the legacy path appends nothing new.
func (o *Orchestrator) recordRoutingDecision(ctx context.Context, cycle int, cs CycleState, seq int, dec router.RouterDecision) {
	ts := o.now().UTC().Format(time.RFC3339)
	artifactPath := filepath.Join(cs.WorkspacePath, fmt.Sprintf("routing-decision-%d.json", seq))
	sha := ""
	if buf, err := json.MarshalIndent(dec, "", "  "); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN routing-decision marshal: %v\n", err)
		artifactPath = ""
	} else if err := os.MkdirAll(cs.WorkspacePath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN routing-decision mkdir: %v\n", err)
		artifactPath = ""
	} else if err := os.WriteFile(artifactPath, buf, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN routing-decision write: %v\n", err)
		artifactPath = ""
	} else {
		sum := sha256.Sum256(buf)
		sha = hex.EncodeToString(sum[:])
	}

	if err := o.ledger.Append(ctx, LedgerEntry{
		TS: ts, Cycle: cycle, Role: "orchestrator", Kind: "routing_decision",
		ExitCode: 0, ArtifactPath: artifactPath, ArtifactSHA256: sha,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN routing_decision ledger append: %v\n", err)
	}
	for _, sp := range dec.SkipPhases {
		if err := o.ledger.Append(ctx, LedgerEntry{
			TS: ts, Cycle: cycle, Role: sp, Kind: "phase_skipped", ExitCode: 0,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_skipped ledger append: %v\n", err)
		}
	}
}

// recordPhasePlan persists the advisor's CLAMPED whole-cycle plan to
// <workspace>/phase-plan.json (a bare PhasePlanEntry array, symmetric with the
// advisor's wire format) and appends a hash-bound phase_plan ledger entry. Any
// integrity-floor clamps that fired are logged for operator visibility (rich
// per-clamp forensics land in a later slice). Best-effort: a marshal/write/
// append failure WARNs and is swallowed — plan forensics must never abort a
// cycle. Called once per cycle, only at Stage>=Advisory with a non-nil plan.
func (o *Orchestrator) recordPhasePlan(ctx context.Context, cycle int, cs CycleState, plan *router.PhasePlan, clamps []router.Clamp) {
	ts := o.now().UTC().Format(time.RFC3339)
	artifactPath := filepath.Join(cs.WorkspacePath, "phase-plan.json")
	sha := ""
	if buf, err := json.MarshalIndent(plan.Entries, "", "  "); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-plan marshal: %v\n", err)
		artifactPath = ""
	} else if err := os.MkdirAll(cs.WorkspacePath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-plan mkdir: %v\n", err)
		artifactPath = ""
	} else if err := os.WriteFile(artifactPath, buf, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-plan write: %v\n", err)
		artifactPath = ""
	} else {
		sum := sha256.Sum256(buf)
		sha = hex.EncodeToString(sum[:])
	}
	for _, c := range clamps {
		fmt.Fprintf(os.Stderr, "[orchestrator] integrity-floor clamp: %s (%s → %s)\n", c.Rule, c.Proposed, c.Forced)
	}
	if err := o.ledger.Append(ctx, LedgerEntry{
		TS: ts, Cycle: cycle, Role: "orchestrator", Kind: "phase_plan",
		ExitCode: 0, ArtifactPath: artifactPath, ArtifactSHA256: sha,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_plan ledger append: %v\n", err)
	}
}

// enforceNext maps the router's proposed NextPhase back to a core.Phase and
// returns it ONLY if it differs from the static successor AND survives both
// kernel gates: a legal edge (CanTransition) and the artifact-backed spine
// gate (SpineSatisfiedUpTo). Otherwise the static successor stands. This is
// the non-bypassable "kernel disposes" floor for Enforce mode — neither
// Strategy can reach Ship without a real PASS/WARN audit artifact.
func (o *Orchestrator) enforceNext(current, staticNext Phase, sig router.RoutingSignals, dec router.RouterDecision) (Phase, bool) {
	cand := o.candidatePhase(dec.NextPhase)
	if cand == "" || cand == staticNext {
		return staticNext, false
	}
	if !o.transitionLegal(current, cand) {
		return staticNext, false
	}
	if !o.sm.SpineSatisfiedUpTo(cand, sig, o.cfg) {
		return staticNext, false
	}
	return cand, true
}

// candidatePhase resolves a router-proposed phase string to a runnable Phase:
// a built-in (via phaseFromRouter) OR a user phase present in the catalog. An
// unknown string yields "" so enforceNext declines it.
func (o *Orchestrator) candidatePhase(s string) Phase {
	if p := phaseFromRouter(s); p != "" {
		return p
	}
	if _, ok := o.catalog.Get(s); ok {
		return Phase(s)
	}
	return ""
}

// transitionLegal is the kernel legality gate for a proposed edge. Built-in
// phases use the hardcoded state-machine graph. A user phase (optional,
// catalog-defined) is legal iff it makes forward progress in the configured
// order (cfg.Order) — the router only proposes the next runnable optional, and
// SpineSatisfiedUpTo independently guards the mandatory anchors, so an optional
// insertion between anchors cannot skip the spine or reach ship illegitimately.
func (o *Orchestrator) transitionLegal(from, cand Phase) bool {
	if from.IsValid() && cand.IsValid() {
		return o.sm.CanTransition(from, cand) // both built-in: hardcoded graph
	}
	// At least one endpoint is NOT a built-in phase — validate via order
	// forward-progress (both-built-in edges took the sm.CanTransition branch above).
	// A user-phase candidate must be optional (the floor). Leapfrogging a
	// mandatory anchor is independently blocked by SpineSatisfiedUpTo in the caller.
	if !cand.IsValid() {
		spec, ok := o.catalog.Get(string(cand))
		if !ok || !spec.Optional {
			return false
		}
	}
	ci, fi := orderIndex(o.cfg.Order, string(cand)), orderIndex(o.cfg.Order, string(from))
	return ci >= 0 && fi >= 0 && ci > fi
}

// nextInOrder returns the phase immediately following p in the configured
// order, or PhaseEnd when p is last/absent. Used to resume the normal sequence
// after a user phase runs. Assumes cfg.Order is the complete registry order
// (applyRegistry appends every registry phase), so a built-in successor is
// always present when a registry is loaded.
func (o *Orchestrator) nextInOrder(p Phase) Phase {
	i := orderIndex(o.cfg.Order, string(p))
	if i < 0 || i+1 >= len(o.cfg.Order) {
		return PhaseEnd
	}
	return Phase(o.cfg.Order[i+1])
}

// orderIndex returns the position of phase in order, or -1 if absent.
func orderIndex(order []string, phase string) int {
	for i, p := range order {
		if p == phase {
			return i
		}
	}
	return -1
}

// worktreePhase reports whether next writes source and so must run with
// cwd=worktree. Built-in tdd/build always do; a user phase does iff its spec
// sets writes_source. Method form (vs the free WorktreePhase) so it consults
// the injected catalog.
func (o *Orchestrator) worktreePhase(p Phase) bool {
	if WorktreePhase(p) {
		return true
	}
	if spec, ok := o.catalog.Get(string(p)); ok {
		return spec.WritesSource
	}
	return false
}

// runsInWorktree reports whether a phase's subprocess should run with cwd set to
// the cycle worktree. This is a SUPERSET of worktreePhase (source writers): the
// audit phase also runs there — read-only — so its verification commands
// (`git diff HEAD`, `go test`, `test -d`) inspect the builder's PENDING work in
// the worktree rather than the main tree. (Issue #9: a non-Claude auditor running
// a relative `cd go` from the project root inspected an empty main tree and
// false-FAILed work that was present in the worktree; the audit-binding at
// recordAuditBinding already uses cs.ActiveWorktree, so the auditor's cwd must
// match it.) Audit is deliberately NOT a worktreePhase — it gets cwd for
// inspection, never source-write permission (the role-gate keys off worktreePhase).
func (o *Orchestrator) runsInWorktree(p Phase) bool {
	return o.worktreePhase(p) || p == PhaseAudit
}

// phaseFromRouter denormalizes a router phase string back to a core.Phase.
// The router speaks canonical "retrospective"/"end"; core uses "retro"/
// PhaseEnd. An unknown string yields "" so enforceNext declines it.
func phaseFromRouter(s string) Phase {
	switch s {
	case "retrospective":
		return PhaseRetro
	case router.PhaseEnd: // "end" — same string as core.PhaseEnd
		return PhaseEnd
	}
	p := Phase(s)
	if !p.IsValid() {
		return ""
	}
	return p
}

// entriesFromRecords converts FailedRecord values into failureadapter.Entry.
// Inlined here (rather than exposed from failureadapter) to avoid a
// circular import between core and failureadapter.
func entriesFromRecords(records []FailedRecord) []failureadapter.Entry {
	out := make([]failureadapter.Entry, len(records))
	for i, r := range records {
		out[i] = failureadapter.Entry{
			TS:                r.TS,
			Cycle:             r.Cycle,
			Verdict:           r.Verdict,
			Classification:    failureadapter.Classification(r.Classification),
			RecordedAt:        r.RecordedAt,
			ExpiresAt:         r.ExpiresAt,
			AuditReportPath:   r.AuditReportPath,
			AuditReportSHA256: r.AuditReportSHA256,
			GitHead:           r.GitHead,
			TreeStateSHA:      r.TreeStateSHA,
			Defects:           r.Defects,
			Retrospected:      r.Retrospected,
			Summary:           r.Summary,
		}
	}
	return out
}

// backfillArtifactPath returns the absolute path to the backfilled artifact file.
func backfillArtifactPath(workspacePath, phase string) string {
	var filename string
	switch phase {
	case "tdd":
		filename = "test-report.md"
	case "intent":
		filename = "intent.md"
	default:
		filename = phase + "-report.md"
	}
	return filepath.Join(workspacePath, filename)
}
