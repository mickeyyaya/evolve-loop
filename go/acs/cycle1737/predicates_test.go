//go:build acs

package cycle1737

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	loopPkg = "./cmd/evolve"
	corePkg = "./internal/core"

	gcRootGrader         = "TestC1735_001_GCManifestDirIsBatchOwnedNotCycleWorkspace"
	preBatchSiteGrader   = "TestC1735_002_PreBatchGCHookTargetsBatchOwnedDir"
	nextRunDirGrader     = "TestC1735_003_NextCycleWorkspaceHasNoGCManifestsAfterPreBatchGC"
	guardArchiveGrader   = "TestC1735_004_ArchivePollutedWorkspaceStillArchivesGCManifestOnlyDir"
	guardLaneScopeGrader = "TestC1735_005_ArchivePollutedWorkspaceIgnoresLaneScopeOnly"
	stageDirsGrader      = "TestC1735_006_PreBatchGCManifestSurvivesBatchEndGC"
	batchEndSiteGrader   = "TestRunLoopBatch_GCHookFiresAfterFinalizeAtBatchEnd"
	guardPreservesGrader = "TestArchivePollutedWorkspace_NonEmptyDirRenamed"
	noCycleSinkProbe     = "TestC1737ProbeNoCycleBatchEndSink"

	offendersRel = "go/internal/sizeratchet/offenders.json"

	preFixBase     = "c9093d5c9cf9b92a80ea8aeac2e7bb68fc98bcbc"
	explainDocGlob = "docs/explain/builds/cycle-1737-*.md"
)

var preFixLoopSources = []string{
	"cmd/evolve/cmd_loop.go",
	"cmd/evolve/cmd_loop_batch.go",
	"cmd/evolve/cmd_loop_outcome.go",
	"cmd/evolve/cmd_loop_prebatch.go",
}

type mutant struct {
	src, anchor, replacement string
}

var (
	preBatchRevert = mutant{
		src:         "cmd/evolve/cmd_loop_prebatch.go",
		anchor:      `gcHookFn(cfg, filepath.Join(gcManifestDir(cfg.EvolveDir), "pre-batch"), stderr)`,
		replacement: `gcHookFn(cfg, filepath.Clean(func() string { last, _ := readLastCycleNumber(context.Background(), deps.Storage); return cycleWorkspace(cfg.ProjectRoot, last+1) }()), stderr)`,
	}
	batchEndRevert = mutant{
		src:         "cmd/evolve/cmd_loop_batch.go",
		anchor:      `gcHookFn(cfg, filepath.Join(gcManifestDir(cfg.EvolveDir), "batch-end"), stderr)`,
		replacement: `gcHookFn(cfg, filepath.Clean(cycleWorkspace(cfg.ProjectRoot, lastCycleIn(*lr)+1)), stderr)`,
	}
	guardAllowlistsGCManifests = mutant{
		src:         "internal/core/workspace_guard.go",
		anchor:      `if e.Name() != LaneScopeFile {`,
		replacement: `if e.Name() != LaneScopeFile && e.Name() != "gc-shadow-manifest.json" && e.Name() != "workspace-gc-manifest.json" {`,
	}
)

func goModuleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx := context.Background()
	if d, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, d)
		t.Cleanup(cancel)
	}
	return ctx
}

func overlayOf(t *testing.T, mutants ...mutant) string {
	t.Helper()
	dir := t.TempDir()
	replace := make(map[string]string, len(mutants))
	for i, m := range mutants {
		orig := filepath.Join(goModuleDir(t), filepath.FromSlash(m.src))
		src, err := os.ReadFile(orig)
		if err != nil {
			t.Fatalf("read %s: %v", m.src, err)
		}
		if n := strings.Count(string(src), m.anchor); n != 1 {
			t.Fatalf("mutant anchor %q occurs %d time(s) in %s, want exactly 1", m.anchor, n, m.src)
		}
		mutated := filepath.Join(dir, filepath.Base(m.src)+"."+string(rune('a'+i))+".go")
		if err := os.WriteFile(mutated, []byte(strings.Replace(string(src), m.anchor, m.replacement, 1)), 0o644); err != nil {
			t.Fatalf("write mutant of %s: %v", m.src, err)
		}
		replace[orig] = mutated
	}
	return writeOverlay(t, dir, replace)
}

var guardSources = []string{
	"go/internal/core/workspace_guard.go",
	"go/internal/core/cyclerun.go",
}

func faithfulPreFixOverlay(t *testing.T, extra map[string]string) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	dir := t.TempDir()
	replace := make(map[string]string, len(preFixLoopSources))
	for _, src := range preFixLoopSources {
		body, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", preFixBase+":go/"+src)
		if code != 0 {
			t.Fatalf("git show %s:go/%s failed (exit=%d): %v\n%s", preFixBase, src, code, err, stderr)
		}
		if src == "cmd/evolve/cmd_loop_outcome.go" {
			body += "\nfunc gcManifestDir(evolveDir string) string { return filepath.Join(evolveDir, \"gc\") }\n"
		}
		restored := filepath.Join(dir, filepath.Base(src))
		if err := os.WriteFile(restored, []byte(body), 0o644); err != nil {
			t.Fatalf("write pre-fix %s: %v", src, err)
		}
		replace[filepath.Join(goModuleDir(t), filepath.FromSlash(src))] = restored
	}
	for dst, src := range extra {
		replace[dst] = src
	}
	return writeOverlay(t, dir, replace)
}

func writeOverlay(t *testing.T, dir string, replace map[string]string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	path := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	return path
}

type goTestRun struct {
	out  string
	code int
}

func runGraders(t *testing.T, overlay, pkg string, names ...string) goTestRun {
	t.Helper()
	cmd := exec.CommandContext(testContext(t), "go", "test", "-count=1", "-v", "-overlay="+overlay,
		"-run", "^("+strings.Join(names, "|")+")$", pkg)
	cmd.Dir = goModuleDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return goTestRun{out: string(out)}
	case errors.As(err, &exitErr):
		return goTestRun{out: string(out), code: exitErr.ExitCode()}
	default:
		t.Fatalf("%s: %v\n%s", cmd, err, out)
		return goTestRun{}
	}
}

func reported(run goTestRun, verdict, name string) bool {
	return regexp.MustCompile(`(?m)^--- ` + verdict + `: ` + regexp.QuoteMeta(name) + ` \(`).MatchString(run.out)
}

func requirePass(t *testing.T, run goTestRun, names ...string) {
	t.Helper()
	failed := run.code != 0
	for _, name := range names {
		if !reported(run, "PASS", name) {
			failed = true
			t.Errorf("RED: %s did not report --- PASS", name)
		}
	}
	if failed {
		t.Errorf("RED: grader run exited %d:\n%s", run.code, run.out)
	}
}

func requireKilled(t *testing.T, label string, run goTestRun, names ...string) {
	t.Helper()
	failed := false
	for _, name := range names {
		if !reported(run, "FAIL", name) {
			failed = true
			t.Errorf("RED: %s did not fail under mutant %q — the grader does not discriminate the bug", name, label)
		}
	}
	if failed {
		t.Logf("mutant %q output:\n%s", label, run.out)
	}
}

func TestC1737_001_BatchGCSweepsPublishUnderEvolveGCStageDirsNeverRunsCycleN(t *testing.T) {
	requirePass(t, runGraders(t, "", loopPkg, gcRootGrader, preBatchSiteGrader, batchEndSiteGrader, stageDirsGrader),
		gcRootGrader, preBatchSiteGrader, batchEndSiteGrader, stageDirsGrader)

	requireKilled(t, "pre-batch sweep targets runs/cycle-(last+1)",
		runGraders(t, overlayOf(t, preBatchRevert), loopPkg, preBatchSiteGrader), preBatchSiteGrader)
	requireKilled(t, "batch-end sweep targets runs/cycle-(last+1)",
		runGraders(t, overlayOf(t, batchEndRevert), loopPkg, batchEndSiteGrader), batchEndSiteGrader)
}

func TestC1737_002_CycleAfterPreBatchGCFindsNoGCManifestInItsRunDir(t *testing.T) {
	requirePass(t, runGraders(t, "", loopPkg, nextRunDirGrader), nextRunDirGrader)

	requireKilled(t, "pre-batch sweep targets runs/cycle-(last+1)",
		runGraders(t, overlayOf(t, preBatchRevert), loopPkg, nextRunDirGrader), nextRunDirGrader)
}

func TestC1737_003_WorkspaceGuardStillArchivesAnyNonLaneScopeFile(t *testing.T) {
	requirePass(t, runGraders(t, "", corePkg, guardArchiveGrader, guardLaneScopeGrader),
		guardArchiveGrader, guardLaneScopeGrader)

	requireKilled(t, "guard allowlists GC manifest filenames",
		runGraders(t, overlayOf(t, guardAllowlistsGCManifests), corePkg, guardArchiveGrader), guardArchiveGrader)
}

func TestC1737_004_ModuleWideSizeRatchetHoldsWithoutEditingOffenders(t *testing.T) {
	spans, err := sizeratchet.Walk(goModuleDir(t))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	offenders, err := sizeratchet.LoadOffenders(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(offendersRel)))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(spans, offenders); err != nil {
		t.Errorf("RED: %v", err)
	}

	root := acsassert.RepoRoot(t)
	base, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "main")
	if code != 0 {
		t.Fatalf("git merge-base HEAD main failed (exit=%d): %v\n%s", code, err, stderr)
	}
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", strings.TrimSpace(base), "--", offendersRel)
	if code != 0 {
		t.Fatalf("git diff %s -- %s failed (exit=%d): %v\n%s", strings.TrimSpace(base), offendersRel, code, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("RED: this lane edits %s; an allowance is a ceiling and only the wave-boundary tighten writes the file:\n%s", offendersRel, diff)
	}
}

type preFixTruth struct {
	failing           []string
	batchEndSinkCycle string
	revertSinkCycle   string
	preBatchSinkCycle string
}

var (
	batchEndSinkRE = regexp.MustCompile(`batch-end gcHookFn workspace = "[^"]*/\.evolve/runs/cycle-(\d+)"`)
	preBatchSinkRE = regexp.MustCompile(`pre-batch gcHookFn workspace = "[^"]*/\.evolve/runs/cycle-(\d+)"`)
	noCycleSinkRE  = regexp.MustCompile(`no-cycle batch-end sink: runs/cycle-(\d+)`)
)

func observePreFixBase(t *testing.T) preFixTruth {
	t.Helper()
	run := runGraders(t, faithfulPreFixOverlay(t, nil), loopPkg,
		gcRootGrader, preBatchSiteGrader, nextRunDirGrader, stageDirsGrader, batchEndSiteGrader)
	var truth preFixTruth
	for _, g := range []string{gcRootGrader, preBatchSiteGrader, nextRunDirGrader, stageDirsGrader, batchEndSiteGrader} {
		switch {
		case reported(run, "FAIL", g):
			truth.failing = append(truth.failing, g)
		case !reported(run, "PASS", g):
			t.Fatalf("%s did not run on the faithful pre-fix base:\n%s", g, run.out)
		}
	}
	if len(truth.failing) == 0 {
		t.Fatalf("no grader fails on the faithful pre-fix base, so the overlay does not restore the bug:\n%s", run.out)
	}
	truth.batchEndSinkCycle = batchEndSinkCycle(t, "faithful pre-fix base", run)
	if m := preBatchSinkRE.FindStringSubmatch(run.out); m != nil {
		truth.preBatchSinkCycle = m[1]
	}
	truth.revertSinkCycle = batchEndSinkCycle(t, "batchEndRevert",
		runGraders(t, overlayOf(t, batchEndRevert), loopPkg, batchEndSiteGrader))
	return truth
}

func batchEndSinkCycle(t *testing.T, label string, run goTestRun) string {
	t.Helper()
	m := batchEndSinkRE.FindStringSubmatch(run.out)
	if m == nil {
		t.Fatalf("%s: the batch-end grader reported no runs/cycle-N sink:\n%s", label, run.out)
	}
	return m[1]
}

func readExplanationDoc(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(explainDocGlob)))
	if err != nil || len(matches) != 1 {
		t.Fatalf("want exactly one %s, got %v (err=%v)", explainDocGlob, matches, err)
	}
	rel, err := filepath.Rel(root, matches[0])
	if err != nil {
		t.Fatalf("rel path of %s: %v", matches[0], err)
	}
	if _, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", filepath.ToSlash(rel)); code != 0 {
		t.Errorf("RED: %s is untracked, so it would not ship (exit=%d): %v\n%s", rel, code, err, stderr)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(raw)
}

var (
	sentenceEndRE     = regexp.MustCompile(`[.!?](\s+|$)`)
	graderIDRE        = regexp.MustCompile(`^TestC\d+_\d+`)
	bothSitesRE       = regexp.MustCompile(`(?i)\bboth\b|pre-batch and (the )?batch-end`)
	nextCycleSinkRE   = regexp.MustCompile(`(?i)cycle-<next>|<next cycle>|last\s*\+\s*1|next cycle's (own )?(fresh )?run dir|not started yet|has not started`)
	batchEndOwnSinkRE = regexp.MustCompile(`(?i)batchEndGCCycle|last-run|own last`)
	syntheticRE       = regexp.MustCompile(`(?i)synthetic`)
	revertContextRE   = regexp.MustCompile(`(?i)revert|put back|pre-fix|before the fix|restor|\bR1\b|mutant`)
	greenClaimRE      = regexp.MustCompile(`(?i)\bgreen\b|\bpass(es|ed|ing)?\b`)
	failClaimRE       = regexp.MustCompile(`(?i)\bfail(s|ed|ing)?\b`)
)

func docSentences(doc string) []string {
	var blocks, cur []string
	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, strings.Join(cur, " "))
			cur = nil
		}
	}
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		isHeading := strings.HasPrefix(trimmed, "#")
		if trimmed == "" || isHeading || strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "|") {
			flush()
		}
		if trimmed != "" {
			cur = append(cur, trimmed)
		}
		if isHeading {
			flush()
		}
	}
	flush()
	var sentences []string
	for _, b := range blocks {
		for _, s := range sentenceEndRE.Split(b, -1) {
			if s = strings.TrimSpace(s); s != "" {
				sentences = append(sentences, s)
			}
		}
	}
	return sentences
}

func docSection(doc, heading string) string {
	_, after, ok := strings.Cut(doc, "\n## "+heading+"\n")
	if !ok {
		return ""
	}
	if end := strings.Index(after, "\n## "); end >= 0 {
		return after[:end]
	}
	return after
}

func explanationDocFindings(doc string, truth preFixTruth) []string {
	sentences := docSentences(doc)
	var findings []string
	if truth.batchEndSinkCycle != truth.revertSinkCycle {
		for _, s := range sentences {
			if bothSitesRE.MatchString(s) && nextCycleSinkRE.MatchString(s) && !batchEndOwnSinkRE.MatchString(s) && !syntheticRE.MatchString(s) {
				findings = append(findings, fmt.Sprintf("says both GC call sites targeted the next cycle's run dir, but the pre-fix batch-end sweep targeted runs/cycle-%s (batchEndGCCycle, the batch's own last-run cycle), not runs/cycle-%s: %q",
					truth.batchEndSinkCycle, truth.revertSinkCycle, s))
			}
		}
		if !batchEndOwnSinkRE.MatchString(docSection(doc, "Summary") + docSection(doc, "Rationale")) {
			findings = append(findings, "Summary and Rationale never state the batch-end site's pre-fix target (batchEndGCCycle, the batch's own last-run cycle)")
		}
	}
	for _, g := range truth.failing {
		findings = append(findings, graderClaimFindings(sentences, g)...)
	}
	return findings
}

func graderClaimFindings(sentences []string, grader string) []string {
	id := grader
	if m := graderIDRE.FindString(grader); m != "" {
		id = m
	}
	var findings []string
	reportsFail := false
	for _, s := range sentences {
		switch {
		case !strings.Contains(s, id):
		case failClaimRE.MatchString(s):
			reportsFail = true
		case greenClaimRE.MatchString(s) && revertContextRE.MatchString(s) && !syntheticRE.MatchString(s):
			findings = append(findings, fmt.Sprintf("says %s stays green under a revert not labeled synthetic, but it FAILs on the faithful pre-fix base: %q", id, s))
		}
	}
	if !reportsFail {
		findings = append(findings, fmt.Sprintf("never reports that %s FAILs on the faithful pre-fix base", id))
	}
	return findings
}

func TestC1737_005_ExplanationDocMatchesFaithfulPreFixBase(t *testing.T) {
	truth := observePreFixBase(t)
	t.Logf("faithful pre-fix base %s: failing=%v, batch-end sink runs/cycle-%s, batchEndRevert sink runs/cycle-%s",
		preFixBase[:8], truth.failing, truth.batchEndSinkCycle, truth.revertSinkCycle)
	for _, f := range explanationDocFindings(readExplanationDoc(t), truth) {
		t.Errorf("RED: explanation document %s", f)
	}
}

type overwriteTruth struct {
	preBatchSinkCycle  string
	batchEndSinkCycle  string
	noCycleSinkCycle   string
	guardArchivesFirst bool
}

func noCycleProbeSource(startNext string) string {
	return `package main

import "testing"

func ` + noCycleSinkProbe + `(t *testing.T) {
	t.Logf("no-cycle batch-end sink: runs/cycle-%d", batchEndGCCycle(loopResult{}, ` + startNext + `))
}
`
}

func observeOverwriteTruth(t *testing.T) overwriteTruth {
	t.Helper()
	pre := observePreFixBase(t)
	if pre.preBatchSinkCycle == "" {
		t.Fatalf("the faithful pre-fix base reported no runs/cycle-N pre-batch sink, so the overwrite truth is unobservable")
	}
	truth := overwriteTruth{preBatchSinkCycle: pre.preBatchSinkCycle, batchEndSinkCycle: pre.batchEndSinkCycle}

	root := acsassert.RepoRoot(t)
	diff, stderr, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root, "diff", preFixBase, "--"}, guardSources...)...)
	if code != 0 {
		t.Fatalf("git diff %s -- %v failed (exit=%d): %v\n%s", preFixBase, guardSources, code, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Fatalf("the workspace guard changed since %s, so running it here does not show what the pre-fix loop did:\n%s", preFixBase[:8], diff)
	}
	guard := runGraders(t, "", corePkg, guardArchiveGrader, guardPreservesGrader)
	truth.guardArchivesFirst = true
	for _, g := range []string{guardArchiveGrader, guardPreservesGrader} {
		switch {
		case reported(guard, "FAIL", g):
			truth.guardArchivesFirst = false
		case !reported(guard, "PASS", g):
			t.Fatalf("%s did not run:\n%s", g, guard.out)
		}
	}

	probe := filepath.Join(t.TempDir(), "no_cycle_probe_test.go")
	if err := os.WriteFile(probe, []byte(noCycleProbeSource(truth.preBatchSinkCycle)), 0o644); err != nil {
		t.Fatalf("write no-cycle probe: %v", err)
	}
	probeAt := filepath.Join(goModuleDir(t), "cmd", "evolve", "zz_c1737_no_cycle_probe_test.go")
	run := runGraders(t, faithfulPreFixOverlay(t, map[string]string{probeAt: probe}), loopPkg, noCycleSinkProbe)
	m := noCycleSinkRE.FindStringSubmatch(run.out)
	if run.code != 0 || m == nil {
		t.Fatalf("the no-cycle probe did not report a sink on the faithful pre-fix base (exit=%d):\n%s", run.code, run.out)
	}
	truth.noCycleSinkCycle = m[1]
	return truth
}

var (
	overwriteRE      = regexp.MustCompile(`(?i)\boverwr\w*|\bclobber\w*`)
	preBatchRE       = regexp.MustCompile(`(?i)\bpre-batch\b`)
	fixtureScopeRE   = regexp.MustCompile(`(?i)\bfixture\b|guard-less|without the guard`)
	negatedRE        = regexp.MustCompile(`(?i)\b(no|never|nothing|not|cannot|without)\b[^,;]{0,40}\boverwr|\boverwr\w*\s+nothing\b`)
	newLayoutRE      = regexp.MustCompile(`\.evolve/gc\b`)
	ranCyclesRE      = regexp.MustCompile(`(?i)\bran (a |one |a single |its |any |\d+ )?(single )?cycles?\b|\b(single|one)[- ]cycle\b`)
	ranNoCyclesRE    = regexp.MustCompile(`(?i)\bran no cycles?\b|\bno cycles? (ran|were run)\b|\bran none\b|\bzero cycles?\b`)
	clauseSplitterRE = regexp.MustCompile(`;\s*`)
)

func docClauses(doc string) []string {
	var clauses []string
	for _, s := range docSentences(doc) {
		for _, c := range clauseSplitterRE.Split(s, -1) {
			if c = strings.TrimSpace(c); c != "" {
				clauses = append(clauses, c)
			}
		}
	}
	return clauses
}

func overwriteClaimFindings(doc string, truth overwriteTruth) []string {
	sharedInLoop := truth.preBatchSinkCycle == truth.batchEndSinkCycle
	var findings []string
	if sharedInLoop && truth.guardArchivesFirst {
		for _, c := range docClauses(doc) {
			switch {
			case !overwriteRE.MatchString(c) || !preBatchRE.MatchString(c):
			case fixtureScopeRE.MatchString(c), negatedRE.MatchString(c), newLayoutRE.MatchString(c):
			case ranNoCyclesRE.MatchString(c) && !ranCyclesRE.MatchString(c) && truth.noCycleSinkCycle == truth.preBatchSinkCycle:
			default:
				findings = append(findings, fmt.Sprintf("says the batch-end manifest overwrote the pre-batch one in a batch that ran cycles, but the workspace guard archived runs/cycle-%s (pre-batch manifests kept in runs/cycle-%s.polluted-<ts>) before the batch-end sweep wrote there; that overwrite shows only in the guard-less test fixture: %q",
					truth.preBatchSinkCycle, truth.preBatchSinkCycle, c))
			}
		}
	}
	if truth.noCycleSinkCycle == truth.preBatchSinkCycle {
		scoped := false
		for _, c := range docClauses(docSection(doc, "Summary") + "\n\n" + docSection(doc, "Rationale")) {
			if overwriteRE.MatchString(c) && preBatchRE.MatchString(c) && ranNoCyclesRE.MatchString(c) {
				scoped = true
			}
		}
		if !scoped {
			findings = append(findings, fmt.Sprintf("Summary and Rationale never state where the pre-fix overwrite did happen: a batch that ran no cycles sent its batch-end manifest to runs/cycle-%s, the pre-batch sink, with no guard in between",
				truth.noCycleSinkCycle))
		}
	}
	return findings
}

func TestC1737_006_ExplanationDocScopesPreFixOverwriteToGuardlessPaths(t *testing.T) {
	truth := observeOverwriteTruth(t)
	t.Logf("faithful pre-fix base %s: pre-batch sink runs/cycle-%s, batch-end sink runs/cycle-%s, no-cycle batch-end sink runs/cycle-%s, guard archives first=%v",
		preFixBase[:8], truth.preBatchSinkCycle, truth.batchEndSinkCycle, truth.noCycleSinkCycle, truth.guardArchivesFirst)
	for _, f := range overwriteClaimFindings(readExplanationDoc(t), truth) {
		t.Errorf("RED: explanation document %s", f)
	}
}
