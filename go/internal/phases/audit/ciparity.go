package audit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// ciparity.go — the audit phase's "CI-parity" deterministic gates. Each runs a
// whole-repo CI command (the EXACT one .github/workflows/go.yml runs) against
// THIS cycle's worktree, so a cycle can never ship green-locally / red-in-CI —
// the recurring "per-cycle proof ≠ repo-wide CI gate" disease that broke main
// via import cycles (go vet ./...), unregistered/over-ceiling env flags (-tags
// acs acs-durable), and unnamed exports (apicover -enforce).
//
// These are wired ONLY in NewDefaultWithStageCompact (production); New(Config{})
// leaves them nil so the audit package's own `go test` never recursively forks
// the go toolchain. They run in the phase-runner process (not the sandboxed
// auditor LLM), so the subprocess is unrestricted.
//
// Contract (matches gofmtCheckDefault): returns ([]offenders, nil) → FAIL when
// the CI command reports failures; (nil, err) → WARN (fail-open) when the gate
// itself cannot run (no toolchain / no module); (nil, nil) → clean.

const (
	goVetTimeout           = 4 * time.Minute
	acsDurableTimeout      = 8 * time.Minute
	integrationTierTimeout = 15 * time.Minute
)

// apicoverTimeout bounds the WHOLE apicover gate — the forked toolchain
// pre-steps AND the in-process apicover.Run measurement (which threads this
// ctx to its per-file AST walks; apicover-inprocess-ctx-timeout). A var, not a
// const, for the same reason as runCmd below: tests shrink it to force the
// ctx-interruption path without an 8-minute wait.
var apicoverTimeout = 8 * time.Minute

// runCmd is the subprocess runner the CI-parity gates use. It is a package var
// so tests can inject a fake runner and exercise the exit-code mapping + the
// apicover pipeline without forking the real go toolchain.
var runCmd sysexec.RunFunc = sysexec.DefaultRunner

// moduleDirForReq resolves the cycle's go/ module dir (where the builder's code
// lives), preferring the worktree. Empty → no-op signal ("").
func moduleDirForReq(req core.PhaseRequest) string {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	if root == "" {
		return ""
	}
	dir := codequality.ModuleDir(root)
	// Require a real go module (go.mod present). ModuleDir falls back to `root`
	// itself when there is no go/ subdir, so an IsDir check alone would run the
	// gate in a non-module directory — go vet then fails "go.mod not found",
	// a false offender. A synthetic/incomplete test worktree has no go.mod.
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return ""
	}
	return dir
}

// runCIGate runs one CI command in the cycle's go/ dir and maps the result to
// the hook contract via the EXIT CODE (see the Capture note in the body): an
// exec-start failure (binary not found, context cancelled) → error → fail-open
// WARN; ANY non-zero exit → offenders → FAIL (a synthesized line covers the
// rare no-output case); exit 0 → clean.
func runCIGate(req core.PhaseRequest, label string, timeout time.Duration, name string, args ...string) ([]string, error) {
	dir := moduleDirForReq(req)
	if dir == "" {
		return nil, nil // no go module in the worktree → nothing to check
	}
	run := runCmd // capture once at entry (consistent with apicoverEnforceChangedDefault)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// Capture (NOT CombinedOutput): DefaultRunner maps a non-zero process EXIT to
	// (code, nil), reserving err for unrecoverable start failures. So only the
	// exit code distinguishes "the tool ran and found problems" (code != 0 →
	// FAIL) from "the gate could not run" (err != nil → fail-open WARN). Capture
	// returns stdout AND stderr — go vet writes its diagnostics to stderr.
	out, errOut, code, err := sysexec.Capture(ctx, run, dir, name, args...)
	if err != nil {
		return nil, fmt.Errorf("%s gate could not run: %w", label, err) // fail-open → WARN
	}
	if code == 0 {
		return nil, nil // clean
	}
	combined := strings.TrimSpace(out + "\n" + errOut)
	if combined == "" {
		combined = fmt.Sprintf("%s exited %d (no output)", name, code)
	}
	return offenderLines(combined), nil // ran + non-zero exit → FAIL
}

// offenderLines extracts the most informative tail of a failing command's
// output (the actual FAIL/error lines) so the diagnostic is legible, bounded so
// a runaway log cannot bloat the verdict.
func offenderLines(out string) []string {
	all := strings.Split(out, "\n")
	var keep []string
	for _, ln := range all {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if strings.Contains(ln, "FAIL") || strings.Contains(ln, "error") ||
			strings.Contains(ln, "import cycle") || strings.Contains(ln, "UNCOVERED") ||
			strings.Contains(ln, "cannot") || strings.HasPrefix(ln, "--- FAIL") {
			keep = append(keep, ln)
		}
	}
	if len(keep) == 0 { // no recognizable marker — fall back to the last few lines
		start := len(all) - 6
		if start < 0 {
			start = 0
		}
		for _, ln := range all[start:] {
			if ln = strings.TrimSpace(ln); ln != "" {
				keep = append(keep, ln)
			}
		}
	}
	if len(keep) > 12 {
		keep = keep[len(keep)-12:]
	}
	return keep
}

// cycleTouchedGo reports whether this cycle has a build handoff naming >=1
// changed Go package — the signal that this worktree is a REAL cycle build (a
// synthetic test fixture or a docs-only cycle has none). The whole-repo gates
// (go vet, acs-durable) run only then, so they never fire against an incomplete
// module (e.g. a unit-test worktree with a bare go/ dir but no go.mod / repo
// structure).
func cycleTouchedGo(req core.PhaseRequest) bool {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	pkgs, _ := changedPackagesForAudit(root, req.Cycle)
	return len(pkgs) > 0
}

// goVetCheckDefault runs `go vet ./...` (CI go.yml "vet + fmt" step / `make
// lint`) over the whole worktree module — catches import cycles and other
// vet-level defects a scoped build misses. No-op unless the cycle built Go.
func goVetCheckDefault(req core.PhaseRequest) ([]string, error) {
	if !cycleTouchedGo(req) {
		return nil, nil
	}
	return runCIGate(req, "go vet ./...", goVetTimeout, "go", "vet", "./...")
}

// acsDurableCheckDefault runs the durable ACS regression suite with -tags acs
// (CI ci.yml acs-durable gate / `make test-acs-durable`) — catches flagregistry
// / flag-ceiling / skills-drift regressions invisible without the acs build tag.
// No-op unless the cycle built Go.
func acsDurableCheckDefault(req core.PhaseRequest) ([]string, error) {
	if !cycleTouchedGo(req) {
		return nil, nil
	}
	return runCIGate(req, "acs-durable (-tags acs)", acsDurableTimeout,
		"go", "test", "-count=1", "-tags", "acs", "./acs/regression/...")
}

// integrationTierCheckDefault runs the `-tags integration` test tier (go.yml's
// "test … incl. integration tier" step: `go test -tags integration $(go list
// ./... | grep -v /acs/)`) against the cycle worktree. It closes the parity
// hole one tier above go vet: TestFleetSoak went CI-red under a green per-cycle
// audit because ciparity never built the integration tier. Faithful to CI, it
// enumerates the module packages, drops acs/ (per-cycle ACS evals read runtime
// artifacts absent here), then runs the tier; any non-zero exit → offenders →
// FAIL. No-op unless the cycle built Go. -race IS included (CI runs it, so a
// genuine data race in a touched package must fail the gate, not just CI); only
// -cover is dropped — the coverage number is a CI-only concern (ADR-0069).
func integrationTierCheckDefault(req core.PhaseRequest) ([]string, error) {
	if !cycleTouchedGo(req) {
		return nil, nil
	}
	dir := moduleDirForReq(req)
	if dir == "" {
		return nil, nil // no go module in the worktree → nothing to check
	}
	ctx, cancel := context.WithTimeout(context.Background(), integrationTierTimeout)
	defer cancel()
	run := runCmd
	// go.yml lists packages WITHOUT the integration tag, then filters acs/.
	listOut, err := sysexec.Output(ctx, run, dir, "go", "list", "./...")
	if err != nil {
		return nil, fmt.Errorf("integration-tier gate: go list: %w", err) // fail-open → WARN
	}
	var pkgs []string
	for _, p := range strings.Fields(listOut) {
		if strings.Contains(p, "/acs/") {
			continue
		}
		pkgs = append(pkgs, p)
	}
	if len(pkgs) == 0 {
		return nil, nil
	}
	args := append([]string{"test", "-race", "-count=1", "-tags", "integration"}, pkgs...)
	out, errOut, code, cerr := sysexec.Capture(ctx, run, dir, "go", args...)
	if cerr != nil {
		return nil, fmt.Errorf("integration-tier gate could not run: %w", cerr) // fail-open → WARN
	}
	if code == 0 {
		return nil, nil // clean
	}
	return offenderLines(strings.TrimSpace(out + "\n" + errOut)), nil // ran + non-zero → FAIL
}

// apicoverEnforceChangedDefault runs `apicover -enforce` (CI go.yml "api-coverage
// enforce" step) over the enforced packages this cycle actually touched — the
// AST-level UNCOVERED (unnamed-export) check that repeatedly broke main. Scoped
// to the touched∩enforced set (O(change)); a no-op when the cycle touched no
// enforced package. FALSE-GREEN (coverage-dependent) is left to CI, matching the
// acs/regression/apicover completeness/correctness split.
func apicoverEnforceChangedDefault(req core.PhaseRequest) ([]string, error) {
	dir := moduleDirForReq(req)
	if dir == "" {
		return nil, nil
	}
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	changed, derivable := changedPackagesForAudit(root, req.Cycle)
	enforceBytes, err := os.ReadFile(filepath.Join(dir, ".apicover-enforce"))
	if err != nil {
		return nil, nil // no enforce list → nothing to enforce
	}
	if !derivable {
		// Underivable changed-set on a cycle WITH an enforce list: git failed
		// (no repo, bad baseRef, fleet .git/index.lock race), so we cannot prove
		// the touched∩enforced set is empty. FAIL loud (err==nil) instead of the
		// silent (nil,nil) no-op that shipped an uncovered export (cycle-581 D1).
		return []string{"changed-package set is underivable this cycle (git diff failed) — apicover -enforce gate cannot verify coverage; treat as FAIL, do not ship"}, nil
	}
	touched := ciparity.IntersectEnforced(changed, enforceBytes)
	if len(touched) == 0 {
		return nil, nil // cycle touched no enforced package
	}

	ctx, cancel := context.WithTimeout(context.Background(), apicoverTimeout)
	defer cancel()
	run := runCmd

	// Scoped coverage profile over the touched packages (apicover reads a
	// func-coverage file), then the enforce gate IN-PROCESS over just those dirs
	// — the same pipeline as go.yml, scoped, but folded into the evolve binary
	// (one-binary S1): no runtime `go build -o bin/apicover`. The scratch cover
	// files still live under the worktree's bin/, which we ensure exists (the
	// deleted build used to create it as a side effect).
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return nil, fmt.Errorf("apicover gate: ensure bin dir: %w", err)
	}
	covPath := filepath.Join(binDir, "ciparity-cover.txt")
	defer func() { _ = os.Remove(covPath) }() // scratch profile — don't accumulate on a persistent worktree
	// Tag-parity: build the scoped coverage args through the ciparity SSOT so
	// the gate measures the SAME (tagged) coverage number CI does — an untagged
	// run under-reports a tag-gated package by up to 43 points (R1).
	testArgs := ciparity.CoverageTestArgs(covPath, touched)
	if _, err := sysexec.Output(ctx, run, dir, "go", testArgs...); err != nil {
		return nil, fmt.Errorf("apicover gate: scoped coverage run: %w", err)
	}
	funcPath := covPath + ".func.txt"
	defer func() { _ = os.Remove(funcPath) }()
	funcOut, err := sysexec.Output(ctx, run, dir, "go", "tool", "cover", "-func="+covPath)
	if err != nil {
		return nil, fmt.Errorf("apicover gate: cover -func: %w", err)
	}
	if werr := os.WriteFile(funcPath, []byte(funcOut+"\n"), 0o644); werr != nil {
		return nil, fmt.Errorf("apicover gate: write func cover: %w", werr)
	}
	dirsOut, err := sysexec.Output(ctx, run, dir, "go", append([]string{"list", "-e", "-f", "{{.Dir}}"}, touched...)...)
	if err != nil {
		return nil, fmt.Errorf("apicover gate: go list: %w", err)
	}
	dirs := strings.Fields(dirsOut)
	if len(dirs) == 0 {
		return nil, nil
	}
	// In-process enforce gate — the folded apicover.Run, not a bin/apicover
	// subprocess. Exit-code contract: 0 clean; 1 offenders → FAIL; 2 (with a
	// non-nil error) a measurement failure → also FAIL. In-process there is NO
	// exec-start failure mode (the process always "runs"), so a measurement
	// error is a real finding about the touched code — an unparseable enforced
	// package — not the fail-open infra WARN a subprocess exit-2 warranted.
	// Folding it into the offender report keeps the FAIL the old bin/apicover
	// exit-2 produced (cf. the underivable-changed-set hard-FAIL, cycle-581 D1).
	// The gate ctx bounds the measurement itself (apicover-inprocess-ctx-timeout):
	// pre-ctx, a wedged AST walk escaped apicoverTimeout entirely.
	var report bytes.Buffer
	code, runErr := apicover.Run(ctx, apicover.Config{Enforce: true, CoverPath: funcPath, Dirs: dirs}, &report)
	if code == 0 && runErr == nil {
		return nil, nil // clean
	}
	// A ctx-deadline/cancel interruption is INFRA weather, not a finding about
	// the touched code — surface it as an error so this gate fails OPEN (WARN),
	// exactly like the sibling ctx-bounded exec steps above. Real measurement
	// errors (unparseable package) stay in the offender report → FAIL.
	if runErr != nil && (errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runErr, context.Canceled)) {
		return nil, fmt.Errorf("apicover gate: measurement interrupted: %w", runErr)
	}
	detail := strings.TrimSpace(report.String())
	if runErr != nil {
		detail = strings.TrimSpace(detail + "\napicover -enforce measurement error: " + runErr.Error())
	}
	return offenderLines(detail), nil // offenders or measurement error → FAIL
}

// apicoverNewPackageGraduationDefault flags changed go/internal/<pkg> packages
// that are NEW this cycle and absent from .apicover-enforce — the blind spot
// apicoverEnforceChangedDefault's IntersectEnforced silently drops (a package
// new this cycle cannot yet be in the enforce list, so the touched∩enforced
// scoping never inspects it). This is the deterministic, fail-fast half of the
// recurring warnship_apicover_ci_gap: each ungraduated package must gain an
// .apicover-enforce entry + an apicover_named_test.go before audit can PASS.
// Mirrors apicoverEnforceChangedDefault's own resolution (worktree dir, changed
// packages, enforce list); a no-op (nil,nil) when there is no module, no enforce
// list, or nothing ungraduated. go/cmd/... changes are never flagged (out of
// apicover's scope).
func apicoverNewPackageGraduationDefault(req core.PhaseRequest) ([]string, error) {
	dir := moduleDirForReq(req)
	if dir == "" {
		return nil, nil
	}
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	changed, derivable := changedPackagesForAudit(root, req.Cycle)
	enforceBytes, err := os.ReadFile(filepath.Join(dir, ".apicover-enforce"))
	if err != nil {
		return nil, nil // no enforce list → nothing to graduate against
	}
	if !derivable {
		// Same fail-loud reasoning as apicoverEnforceChangedDefault: an
		// underivable changed-set means we cannot prove no new package is
		// ungraduated, so FAIL loud rather than silently no-op (cycle-581 D2).
		return []string{"changed-package set is underivable this cycle (git diff failed) — apicover graduation gate cannot verify new packages; treat as FAIL, do not ship"}, nil
	}
	ungraduated := ciparity.NewUngraduatedPackages(changed, enforceBytes)
	if len(ungraduated) == 0 {
		return nil, nil
	}
	offenders := make([]string, 0, len(ungraduated))
	for _, pkg := range ungraduated {
		offenders = append(offenders, fmt.Sprintf("%s: new package absent from go/.apicover-enforce — add it + an apicover_named_test.go", pkg))
	}
	return offenders, nil
}

// changedPackagesForAudit locates this cycle's changed-package set and reports
// whether it is derivable. It prefers the build handoff when present (same
// locator the EGPS suite uses; a handoff yielding >=1 pkg is derivable), then
// falls back to a deterministic git derivation (changedpkgs.FromGitChecked vs
// HEAD). The handoff has been extinct since ~cycle 215, so the git fallback is
// what keeps the apicover gate live. The derivable flag closes the last
// fail-open hole: previously the git fallback returned nil identically whether
// the tree was git-clean (nothing changed) or the set was underivable (git
// failed), letting an underivable cycle ship with a silent PASS (cycle-581
// D1/D2, standing memory warnship_apicover_ci_gap).
func changedPackagesForAudit(projectRoot string, cycle int) ([]string, bool) {
	if projectRoot == "" {
		return nil, false
	}
	dir := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	for _, name := range []string{"handoff-build.json", "handoff-builder.json"} {
		if pkgs := changedpkgs.ChangedPackages(filepath.Join(dir, name)); len(pkgs) > 0 {
			return pkgs, true // handoff present and non-empty → derivable
		}
	}
	return changedpkgs.FromGitChecked(projectRoot, "HEAD")
}
