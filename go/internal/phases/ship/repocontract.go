package ship

// repocontract.go — the ship-time repo-contract scanner pack (2026-08-05).
//
// Lane ships push directly to main (per-lane landing), so a landing that
// breaks a REPO-WIDE guard suite reds main until a console fix lands. Four
// live incidents in one week, each an operator CI-email storm: the router
// Digest injection (cycle-1250), the phase-catalog metadata stub (1262), the
// tracked profile stubs (v22.13.0 release red), and the incident-postmortem
// spec rework (1313). Per-cycle changed-scope testing structurally cannot
// catch them — the guard suites scan repo-wide state (on-disk catalogs,
// tracked profiles, rendering parity) that a config-only diff never selects.
//
// The pack runs the four guard packages in the lane worktree BEFORE the ship
// binds/pushes. They are existing deterministic tests with FP≈0 by
// construction: if one fails here, main's next run fails identically. A RED
// pack fails the ship closed with the dedicated CodeRepoContractGate
// (mirroring CodeManifestGate, cycle-1064) so the lane FAILs honestly in
// place instead of redding main. Dial: policy.json gates.repo_contract_gate
// ("enforce" default — see policy.go for the shadow-first deviation
// rationale; "off" disables).
//
// cycle-1409 — exit-code classification + forensic persistence. The original
// pack ran a bare `cmd.Run()` and wrapped ANY non-nil error as a contract RED,
// with output teed nowhere. A build-cache contention / module-fetch flake /
// OOM kill was therefore indistinguishable from a genuinely red guard suite,
// and left no artifact to disprove it with: that false RED blocked three
// audit-green ships (cycles 1402/1403/1405; the preserved worktree e0638346
// re-ran 4/4 GREEN against the identical tree, as did baseline cba017c5).
// Now the pack runs `go test -json`, classifies the events, and:
//   - a genuine test/build failure stays CodeRepoContractGate, un-retried, and
//     NAMES the failing tests in the message so ship-error.json carries them;
//   - anything else is retried exactly once, and if still unclassifiable is
//     returned as CodeRepoContractInfra — a distinct, re-dispatchable code;
//   - every run, green or red, tees the scanner output to the run dir's
//     ship-repocontract-scan.log (the green baseline is half the diagnosis).

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"go/build"
	"go/build/constraint"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// repoContractPackages are the repo-wide guard suites whose breakage turned
// main red. Kept to the incident-proven set deliberately: every addition
// costs every ship wall-time and must carry the same FP≈0 property.
var repoContractPackages = []string{
	"./internal/phasespec/...",
	"./internal/profiles/...",
	"./internal/phasecoherence/...",
	"./internal/routingtest/...",
}

// scanLogName is the run-dir artifact every scanner-pack run is teed to —
// green runs included. cycle-1403's RED was undiagnosable precisely because
// no artifact of either the red run OR the green baseline survived. Kept
// unexported: nothing outside this package consumes the name today.
const scanLogName = "ship-repocontract-scan.log"

// packOutcome is one classified scanner-pack run. failedTests is non-empty
// ONLY when the pack itself said "your code is broken" — a `go test -json`
// fail event carrying a test name, or a build/setup failure. An err with an
// EMPTY failedTests is the ambiguous case: the toolchain exited nonzero
// without any guard suite reporting a violation.
type packOutcome struct {
	failedTests []string
	err         error
}

func (o packOutcome) green() bool   { return o.err == nil }
func (o packOutcome) realRed() bool { return o.err != nil && len(o.failedTests) > 0 }

// repoContractTestFn is the seam for the pack execution (package var, mirrors
// the runner seams elsewhere in this package's tests). Production runs
// `go test -json` in the lane worktree's module dir.
var repoContractTestFn = defaultRepoContractTest

func defaultRepoContractTest(ctx context.Context, moduleDir string, out io.Writer) packOutcome {
	return runRepoContractPackages(ctx, moduleDir, out, repoContractPackages)
}

func runRepoContractPackages(ctx context.Context, moduleDir string, out io.Writer, packages []string) packOutcome {
	return runRepoContractPackagesWithTags(ctx, moduleDir, out, packages, nil)
}

func runRepoContractPackagesWithTags(ctx context.Context, moduleDir string, out io.Writer, packages, tags []string) packOutcome {
	args := []string{"test", "-json", "-count=1"}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}
	args = append(args, packages...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return packOutcome{err: fmt.Errorf("go test stdout pipe: %w", err)}
	}
	cmd.Stderr = out // toolchain-level chatter is not a JSON event stream
	if err := cmd.Start(); err != nil {
		return packOutcome{err: fmt.Errorf("go test start: %w", err)}
	}
	// Drain to completion BEFORE Wait: an undrained pipe deadlocks the child.
	failed := classifyPackEvents(stdout, out)
	return packOutcome{failedTests: failed, err: cmd.Wait()}
}

// classifyPackEvents streams a `go test -json` event feed, teeing the
// human-readable Output text to tee (that is what lands in the scan log) and
// collecting the names of genuinely failing tests.
//
// The classification is deliberately conservative in ONE direction: a
// package-level fail with no test name is recorded only when the output marks
// a build/setup failure. A compile break is a real contract RED; an
// unexplained nonzero exit is NOT, and must fall through to the retry path
// rather than block an audit-green ship.
//
// Lines that are not JSON objects are teed verbatim and skipped rather than
// aborting the scan — a stray non-event line must not blind the classifier to
// the real failures after it.
func classifyPackEvents(r io.Reader, tee io.Writer) []string {
	var failed []string
	// A single compile break surfaces TWICE — once as a `build-fail` action
	// (which carries no package on go1.26) and once as the `FAIL pkg [build
	// failed]` output line — so build failures are collected per package and
	// appended after the loop rather than inline. Reporting one broken package
	// as two failures would make the ship error read as a wider RED than it is.
	buildFailPkgs := map[string]bool{}
	unattributedBuildFail := false
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // guard suites emit long lines
	for sc.Scan() {
		line := sc.Bytes()
		var ev struct {
			Action  string `json:"Action"`
			Package string `json:"Package"`
			Test    string `json:"Test"`
			Output  string `json:"Output"`
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			writeTee(tee, string(line)+"\n")
			continue
		}
		writeTee(tee, ev.Output)
		switch {
		case ev.Action == "fail" && ev.Test != "":
			failed = append(failed, ev.Package+"."+ev.Test)
		case ev.Action == "build-fail",
			strings.Contains(ev.Output, "[build failed]"),
			strings.Contains(ev.Output, "[setup failed]"):
			if ev.Package == "" {
				unattributedBuildFail = true
				continue
			}
			buildFailPkgs[ev.Package] = true
		}
	}
	if err := sc.Err(); err != nil {
		writeTee(tee, fmt.Sprintf("[ship] repo-contract gate: event stream read error: %v\n", err))
	}
	pkgs := make([]string, 0, len(buildFailPkgs))
	for pkg := range buildFailPkgs {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs) // deterministic ship-error message across runs
	for _, pkg := range pkgs {
		failed = append(failed, pkg+" [build failed]")
	}
	// A build failure the toolchain never attributed to a package is still a
	// real RED — it must never fall through to the infra/retry path — so it is
	// recorded when no attributed one covers it.
	if unattributedBuildFail && len(pkgs) == 0 {
		failed = append(failed, "[build failed] (package unattributed)")
	}
	return failed
}

func writeTee(tee io.Writer, s string) {
	if tee == nil || s == "" {
		return
	}
	_, _ = io.WriteString(tee, s)
}

// runRepoContractGate executes the scanner pack per the resolved dial.
// Returns nil when the dial is off/empty-off, the pack is green, or an
// unclassifiable first failure cleared on the single retry. A genuine RED
// returns CodeRepoContractGate naming the failing tests; a twice-ambiguous
// failure returns the distinct CodeRepoContractInfra.
//
// workspace is the run dir (`req.Workspace`); the scanner output is teed to
// <workspace>/ship-repocontract-scan.log. Empty workspace degrades to
// stderr-only diagnostics — a missing run dir must never block a ship.
func runRepoContractGate(ctx context.Context, gate, projectRoot, workspace string, stderr io.Writer) error {
	if gate != "enforce" {
		if gate != "" && gate != "off" {
			fmt.Fprintf(stderr, "[ship] repo-contract gate: unknown stage %q — treating as enforce (a typo must not silently disable a red-main guard)\n", gate)
		} else {
			return nil
		}
	}
	moduleDir := filepath.Join(projectRoot, "go")
	out := stderr
	if scan := openScanLog(workspace, stderr); scan != nil {
		// Close error deliberately dropped: the scan log is best-effort
		// forensics; a close failure must never turn a green pack red.
		defer func() { _ = scan.Close() }()
		out = io.MultiWriter(stderr, scan)
	}
	// Header first, so the artifact is non-empty and self-identifying even on
	// a green run — the green baseline is what disproves a false RED.
	fmt.Fprintf(out, "[ship] repo-contract scanner pack: go test -json -count=1 %s (module %s)\n",
		strings.Join(repoContractPackages, " "), moduleDir)

	if err := runClassifiedPack(ctx, out, workspace, "scanner pack", func() packOutcome {
		return repoContractTestFn(ctx, moduleDir, out)
	}); err != nil {
		return err
	}

	groups, excluded, discoveryErr := addedTestPackageGroups(projectRoot)
	if discoveryErr != nil {
		fmt.Fprintf(out, "[ship] repo-contract added-test backstop: discovery unavailable (%v) — skipped; no added tests were scanned\n", discoveryErr)
		return nil
	}
	for _, path := range excluded {
		fmt.Fprintf(out, "[ship] repo-contract gate: EXCLUDED %s (requires_tmux or another build constraint unavailable on this host; backstop required)\n", path)
	}
	for _, group := range groups {
		fmt.Fprintf(out, "[ship] repo-contract added-test backstop: go test -json -count=1")
		if len(group.tags) > 0 {
			fmt.Fprintf(out, " -tags %s", strings.Join(group.tags, ","))
		}
		fmt.Fprintf(out, " %s\n", strings.Join(group.packages, " "))
		if err := runClassifiedPack(ctx, out, workspace, "added-test backstop", func() packOutcome {
			return runRepoContractPackagesWithTags(ctx, moduleDir, out, group.packages, group.tags)
		}); err != nil {
			return err
		}
	}
	return nil
}

func runClassifiedPack(ctx context.Context, out io.Writer, workspace, name string, run func() packOutcome) error {
	first := run()
	switch {
	case first.green():
		return nil
	case first.realRed():
		return contractRed(name, first)
	}
	fmt.Fprintf(out, "[ship] repo-contract %s exited nonzero with NO test-level failure (%v) — retrying once before classing it infra (cycle-1402/1403/1405 false-RED class)\n", name, first.err)
	second := run()
	switch {
	case second.green():
		fmt.Fprintf(out, "[ship] repo-contract gate: retry GREEN — first failure was infra noise, ship proceeds\n")
		return nil
	case second.realRed():
		return contractRed(name, second)
	}
	return shiperr.NewShipError(shiperr.CodeRepoContractInfra, shiperr.ShipClassPrecondition, shiperr.StageAtomicShip,
		fmt.Sprintf("repo-contract %s exited nonzero TWICE with no test-level failure (attempt 1: %v; attempt 2: %v) — INFRA fault, not a contract violation; safe to re-dispatch. Scanner output: %s",
			name, first.err, second.err, scanLogHint(workspace)))
}

type addedTestGroup struct {
	tags     []string
	packages []string
}

func addedTestPackages(projectRoot string) (packages, excluded []string) {
	groups, excluded, _ := addedTestPackageGroups(projectRoot)
	for _, group := range groups {
		packages = append(packages, group.packages...)
	}
	sort.Strings(packages)
	return packages, excluded
}

func addedTestPackageGroups(projectRoot string) (groups []addedTestGroup, excluded []string, retErr error) {
	cmd := exec.Command("git", "-C", projectRoot, "diff", "--cached", "--name-only", "--diff-filter=A")
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("git diff --cached: %w", err)
	}
	packagesByTags := map[string]map[string]bool{}
	tagsByKey := map[string][]string{}
	for _, path := range strings.Fields(string(output)) {
		if !strings.HasPrefix(path, "go/") || !strings.HasSuffix(path, "_test.go") {
			continue
		}
		file := filepath.Join(projectRoot, path)
		tags, runnable, matchErr := addedTestBuildTags(file)
		if matchErr != nil {
			return nil, nil, fmt.Errorf("inspect %s: %w", path, matchErr)
		}
		if !runnable {
			excluded = append(excluded, path)
			continue
		}
		pkg := "./" + filepath.ToSlash(filepath.Dir(strings.TrimPrefix(path, "go/")))
		key := strings.Join(tags, ",")
		if packagesByTags[key] == nil {
			packagesByTags[key] = map[string]bool{}
			tagsByKey[key] = tags
		}
		packagesByTags[key][pkg] = true
	}
	keys := make([]string, 0, len(packagesByTags))
	for key := range packagesByTags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := addedTestGroup{tags: tagsByKey[key]}
		for pkg := range packagesByTags[key] {
			group.packages = append(group.packages, pkg)
		}
		sort.Strings(group.packages)
		groups = append(groups, group)
	}
	sort.Strings(excluded)
	return groups, excluded, nil
}

func addedTestBuildTags(path string) ([]string, bool, error) {
	dir, name := filepath.Dir(path), filepath.Base(path)
	match, err := build.Default.MatchFile(dir, name)
	if err != nil || match {
		return nil, match, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()

	var expr constraint.Expr
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if constraint.IsGoBuild(line) {
			expr, err = constraint.Parse(line)
			break
		}
		if line != "" && !strings.HasPrefix(line, "//") {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return nil, false, err
	}
	if err != nil || expr == nil {
		return nil, false, err
	}
	tagSet := map[string]bool{}
	collectBuildTags(expr, tagSet)
	// minimal: exhaustive tag selection is capped at 12 tags; upgrade to a
	// constraint solver if real added tests exceed that ceiling.
	if tagSet["requires_tmux"] || len(tagSet) > 12 {
		return nil, false, nil
	}
	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	for mask := 1; mask < 1<<len(tags); mask++ {
		candidate := make([]string, 0, len(tags))
		for i, tag := range tags {
			if mask&(1<<i) != 0 {
				candidate = append(candidate, tag)
			}
		}
		ctx := build.Default
		ctx.BuildTags = candidate
		if match, matchErr := ctx.MatchFile(dir, name); matchErr != nil {
			return nil, false, matchErr
		} else if match {
			return candidate, true, nil
		}
	}
	return nil, false, nil
}

func collectBuildTags(expr constraint.Expr, tags map[string]bool) {
	switch x := expr.(type) {
	case *constraint.TagExpr:
		tags[x.Tag] = true
	case *constraint.NotExpr:
		collectBuildTags(x.X, tags)
	case *constraint.AndExpr:
		collectBuildTags(x.X, tags)
		collectBuildTags(x.Y, tags)
	case *constraint.OrExpr:
		collectBuildTags(x.X, tags)
		collectBuildTags(x.Y, tags)
	}
}

// contractRed builds the genuine-violation ship error, naming the parsed
// failing tests so ship-error.json carries them directly instead of the bare
// "exit status 1" that made cycle-1402/1403 undiagnosable.
func contractRed(packName string, o packOutcome) error {
	detail := "added-test backstop"
	if packName == "scanner pack" {
		detail = "fixed scanner pack (phasespec, profiles, phasecoherence, routingtest)"
	}
	return shiperr.NewShipError(shiperr.CodeRepoContractGate, shiperr.ShipClassPrecondition, shiperr.StageAtomicShip,
		fmt.Sprintf("repo-contract %s RED in the lane worktree (%v) — failing: %s — pushing would red main; land the green fix or use an explicit t.Skip for an intentionally red-first reproducer",
			detail, o.err, strings.Join(o.failedTests, ", ")))
}

// openScanLog opens the run-dir scan log, truncating any prior attempt's file.
// Returns nil (never an error) when there is no workspace or the file cannot
// be opened: the log is diagnostics, and losing diagnostics must never fail a
// ship that would otherwise succeed.
func openScanLog(workspace string, stderr io.Writer) *os.File {
	if workspace == "" {
		return nil
	}
	path := filepath.Join(workspace, scanLogName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintf(stderr, "[ship] repo-contract gate: scan log %s unavailable (%v) — continuing with stderr-only diagnostics\n", path, err)
		return nil
	}
	return f
}

func scanLogHint(workspace string) string {
	if workspace == "" {
		return "stderr only (no run workspace)"
	}
	return filepath.Join(workspace, scanLogName)
}
