//go:build acs

package cycle1679

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/build/constraint"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const alignmentREADME = "docs/research/deliverable-alignment-2026-08/README.md"

var allFour = []string{
	coherence.InvariantVerdictAgreement,
	coherence.InvariantTestCounts,
	coherence.InvariantReferencedPaths,
	coherence.InvariantPhaseOrder,
}

func writeAudit(t *testing.T, dir, verdict string, evidencePaths []string) {
	t.Helper()
	payload := map[string]any{"phase": "audit", "verdict": verdict, "schema_version": 1}
	if len(evidencePaths) > 0 {
		payload["schema_version"] = 2
		payload["failure"] = map[string]any{
			"class":          "code-audit-fail",
			"defects":        []string{"H1: cited artifact"},
			"evidence_paths": evidencePaths,
		}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal sentinel: %v", err)
	}
	writeRaw(t, dir, "audit-report.md",
		"# Audit Report\n\n## Verdict\n**"+verdict+"**\n<!-- evolve-verdict: "+string(b)+" -->\n")
}

func writeRaw(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func acsBody(t *testing.T, verdict string, total, green, red, skip int, results []string) string {
	t.Helper()
	rows := make([]map[string]any, 0, len(results))
	for _, r := range results {
		rows = append(rows, map[string]any{"name": "p", "result": r})
	}
	b, err := json.Marshal(map[string]any{
		"schema_version":  "1.0",
		"cycle":           1679,
		"predicate_suite": map[string]any{"this_cycle_count": len(results), "total": total},
		"results":         rows,
		"green_count":     green,
		"red_count":       red,
		"skip_count":      skip,
		"verdict":         verdict,
		"ship_eligible":   red == 0,
	})
	if err != nil {
		t.Fatalf("marshal acs-verdict: %v", err)
	}
	return string(b)
}

func writeTiming(t *testing.T, dir string, phases [][3]string) {
	t.Helper()
	rows := make([]map[string]any, 0, len(phases))
	for _, p := range phases {
		rows = append(rows, map[string]any{
			"phase": p[0], "duration_ms": 1000, "verdict": "PASS", "cost_usd": 0,
			"started_at": p[1], "ended_at": p[2], "attempt_count": 1,
		})
	}
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshal phase-timing: %v", err)
	}
	writeRaw(t, dir, "phase-timing.json", string(b))
}

func coherentWorkspace(t *testing.T) (workspace, worktree string) {
	t.Helper()
	workspace, worktree = t.TempDir(), t.TempDir()
	writeAudit(t, workspace, "PASS", []string{"acs-verdict.json"})
	writeRaw(t, workspace, "acs-verdict.json", acsBody(t, "PASS", 3, 2, 0, 1, []string{"green", "green", "skip"}))
	writeTiming(t, workspace, [][3]string{
		{"scout", "2026-09-14T03:41:45Z", "2026-09-14T03:45:45Z"},
		{"tdd", "2026-09-14T03:46:23Z", "2026-09-14T03:48:15Z"},
		{"build", "2026-09-14T03:48:15Z", "2026-09-14T03:59:00Z"},
		{"audit", "2026-09-14T04:00:00Z", "2026-09-14T04:10:00Z"},
	})
	return workspace, worktree
}

func find(t *testing.T, r coherence.InvariantReport, name string) coherence.Invariant {
	t.Helper()
	for _, inv := range r.Invariants {
		if inv.Name == name {
			return inv
		}
	}
	t.Fatalf("invariant %q absent from the report; got %+v", name, r.Invariants)
	return coherence.Invariant{}
}

func TestC1679_001_SuiteReportsAllFourInvariantsOnACoherentWorkspace(t *testing.T) {
	ws, wt := coherentWorkspace(t)
	got := coherence.CheckCrossArtifactInvariants(ws, wt)
	if !got.Advisory {
		t.Errorf("RED: the aggregate must declare itself advisory (inbox `fix`: ADVISORY until FP rate ~0)")
	}
	for _, name := range allFour {
		if inv := find(t, got, name); inv.Status != coherence.InvariantOK {
			t.Errorf("RED: %s = %q on a coherent workspace, want ok (evidence: %s)", name, inv.Status, inv.Evidence)
		}
	}
	if v := got.Violations(); len(v) != 0 {
		t.Errorf("RED: a coherent workspace reported %d violation(s): %+v", len(v), v)
	}
}

func TestC1679_002_EachInvariantClassIsIndependentlyFalsifiable(t *testing.T) {
	cases := []struct {
		name    string
		want    string
		corrupt func(t *testing.T, ws, wt string)
	}{
		{
			name: "verdict disagreement",
			want: coherence.InvariantVerdictAgreement,
			corrupt: func(t *testing.T, ws, wt string) {
				writeRaw(t, ws, "acs-verdict.json", acsBody(t, "FAIL", 3, 2, 0, 1, []string{"green", "green", "skip"}))
			},
		},
		{
			name: "claimed counts contradict the parsed results",
			want: coherence.InvariantTestCounts,
			corrupt: func(t *testing.T, ws, wt string) {
				writeRaw(t, ws, "acs-verdict.json", acsBody(t, "PASS", 3, 3, 0, 0, []string{"green", "green", "red"}))
			},
		},
		{
			name: "cited evidence path exists nowhere",
			want: coherence.InvariantReferencedPaths,
			corrupt: func(t *testing.T, ws, wt string) {
				writeAudit(t, ws, "PASS", []string{"no/such/artifact-that-never-existed.json"})
			},
		},
		{
			name: "phase chain runs backwards",
			want: coherence.InvariantPhaseOrder,
			corrupt: func(t *testing.T, ws, wt string) {
				writeTiming(t, ws, [][3]string{
					{"audit", "2026-09-14T03:41:45Z", "2026-09-14T03:45:45Z"},
					{"build", "2026-09-14T04:00:00Z", "2026-09-14T04:10:00Z"},
				})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws, wt := coherentWorkspace(t)
			tc.corrupt(t, ws, wt)
			got := coherence.CheckCrossArtifactInvariants(ws, wt)
			inv := find(t, got, tc.want)
			if inv.Status != coherence.InvariantViolated {
				t.Errorf("RED: %s = %q, want violated", tc.want, inv.Status)
			}
			if strings.TrimSpace(inv.Evidence) == "" {
				t.Errorf("RED: %s violated with no evidence — an unactionable finding", tc.want)
			}
			named := false
			for _, v := range got.Violations() {
				if v.Name == tc.want {
					named = true
				}
			}
			if !named {
				t.Errorf("RED: Violations() did not name %s; got %+v", tc.want, got.Violations())
			}
		})
	}
}

func TestC1679_003_AbsentArtifactsAreIndeterminateNeverViolated(t *testing.T) {
	empty, wt := t.TempDir(), t.TempDir()
	got := coherence.CheckCrossArtifactInvariants(empty, wt)
	if len(got.Invariants) != len(allFour) {
		t.Fatalf("RED: empty workspace reported %d invariants, want %d", len(got.Invariants), len(allFour))
	}
	for _, name := range allFour {
		if inv := find(t, got, name); inv.Status != coherence.InvariantIndeterminate {
			t.Errorf("RED: %s = %q on an empty workspace, want indeterminate", name, inv.Status)
		}
	}
	if v := got.Violations(); len(v) != 0 {
		t.Errorf("RED: absence produced %d violation(s) — a false-positive generator: %+v", len(v), v)
	}
}

// acs-predicate: config-check — the deliverable for this AC IS document content,
func TestC1679_004_ExperienceRecordLandedAndItsCitationsResolve(t *testing.T) {
	root := acsassert.RepoRoot(t)
	abs := filepath.Join(root, alignmentREADME)
	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("RED: cannot read %s: %v", alignmentREADME, err)
	}
	body := string(raw)

	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", alignmentREADME); code != 0 {
		t.Errorf("RED: %s is untracked — it would be dropped at ship (cycle-93)", alignmentREADME)
	}

	head := regexp.MustCompile(`(?m)^### 6\.\d+ .*[Cc]ross-artifact.*$`)
	loc := head.FindStringIndex(body)
	if loc == nil {
		t.Fatalf("RED: no '### 6.x' experience-record subsection for the cross-artifact invariant stack "+
			"(item rank 5) in %s — §6 carries 6.1-6.3 only, and the inbox record's `fix` field requires "+
			"this entry per §3.8", alignmentREADME)
	}

	rest := body[loc[1]:]
	if next := regexp.MustCompile(`(?m)^#{2,3} `).FindStringIndex(rest); next != nil {
		rest = rest[:next[0]]
	}
	for _, want := range []string{"Issue", "Gap", "Solution", "Measured"} {
		if !strings.Contains(rest, want) {
			t.Errorf("RED: the §6.x cross-artifact entry has no **%s.** paragraph — §6.1-6.3 shape not followed", want)
		}
	}

	cited := regexp.MustCompile(`go/(?:internal|cmd|acs|pkg)/[A-Za-z0-9_./-]+\.go`).FindAllString(rest, -1)
	if len(cited) == 0 {
		t.Errorf("RED: the §6.x entry cites no implementation file — an experience record with no Solution citation")
	}
	for _, rel := range cited {
		clean := strings.TrimRight(rel, ".,;:)")
		if _, err := os.Stat(filepath.Join(root, clean)); err != nil {
			t.Errorf("RED: the §6.x entry cites %s, which does not exist on disk — the doc fails the very "+
				"referenced-paths-exist invariant it documents", clean)
		}
	}
}

// acs-predicate: config-check — document content is the deliverable (see 004).
func TestC1679_005_PortfolioAndLayerRowsNoLongerMarkTheStackNewOrPartial(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, alignmentREADME))
	if err != nil {
		t.Fatalf("RED: cannot read %s: %v", alignmentREADME, err)
	}
	var portfolio, layer string
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case strings.HasPrefix(line, "| 5 | **Cross-artifact metamorphic invariant stack**"):
			portfolio = line
		case strings.HasPrefix(line, "| **L3 Verification**"):
			layer = line
		}
	}
	if portfolio == "" {
		t.Errorf("RED: the ranked-portfolio row for item rank 5 is gone from %s", alignmentREADME)
	} else if strings.Contains(portfolio, "NEW — filed 0.85") {
		t.Errorf("RED: the rank-5 portfolio row still reads '**NEW — filed 0.85**' while the stack is landed, " +
			"wired at go/internal/core/cyclerun.go:241 and green")
	}
	if layer == "" {
		t.Errorf("RED: the L3 Verification row is gone from %s", alignmentREADME)
	} else if strings.Contains(layer, "cross-artifact invariant stack partial") {
		t.Errorf("RED: the L3 Verification row still calls the stack 'partial (`coherence`)' — all four " +
			"invariants now live in internal/coherence and are recorded from internal/core")
	}
}

const (
	selfPkg = "./acs/cycle1679"

	gradingPredicate = "TestC1676_006_MaterializedEvalIsDurableAndBehavioral"
	gradedEvalPkg    = "./acs/cycle1676"
)

func goModRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func TestC1679_006_AddedEvalAndItsGradingPredicateAgreeInTheShippedTree(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goModRoot(t), "test", "-tags", "acs", "-count=1", "-v",
		"-run", "^"+gradingPredicate+"$", gradedEvalPkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", gradedEvalPkg, err, stderr)
	}
	out := stdout + stderr
	if !strings.Contains(out, "--- PASS: "+gradingPredicate) {
		t.Errorf("RED (audit H1): %s does not pass over %s in the shipped tree, so the "+
			"repo-contract added-test backstop blocks the push. Both the predicate and the eval it "+
			"grades are ADDED paths in this diff — this is the lane's own inconsistency, not inherited "+
			"breakage.\n\nThe frozen predicate is not the thing to change (never weaken a test to clear "+
			"a rejection): it wants >=4 `[code]` behavioural checks in the eval, and the eval carries "+
			"zero. Add the code-graded half — `### ACn: <criterion> [code]` sections, each followed by a "+
			"```bash fence whose command RUNS something and exits 0 — alongside the existing score_cap "+
			"frontmatter. 23 live evals already carry both schemas; "+
			".evolve/evals/retro-delivery-format-binding.md:49 is the canonical shape.\n\n"+
			"exit=%d\ncombined go-test output:\n%s", gradingPredicate, gradedEvalPkg, code, out)
	}
}

func collectTags(e constraint.Expr, set map[string]bool) {
	switch v := e.(type) {
	case *constraint.TagExpr:
		set[v.Tag] = true
	case *constraint.NotExpr:
		collectTags(v.X, set)
	case *constraint.AndExpr:
		collectTags(v.X, set)
		collectTags(v.Y, set)
	case *constraint.OrExpr:
		collectTags(v.X, set)
		collectTags(v.Y, set)
	}
}

func buildTagsOf(t *testing.T, abs string) (tags []string, runnable bool) {
	t.Helper()
	f, err := os.Open(abs)
	if err != nil {
		t.Fatalf("open %s: %v", abs, err)
	}
	defer func() { _ = f.Close() }()

	var expr constraint.Expr
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if constraint.IsGoBuild(line) {
			expr, err = constraint.Parse(line)
			if err != nil {
				t.Fatalf("parse build constraint in %s: %v", abs, err)
			}
			break
		}
		if line != "" && !strings.HasPrefix(line, "//") {
			break
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", abs, err)
	}
	if expr == nil {
		return nil, true
	}
	set := map[string]bool{}
	collectTags(expr, set)
	if set["requires_tmux"] {
		return nil, false
	}
	for tag := range set {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags, true
}

func addedGoTestFiles(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "go/") && strings.HasSuffix(p, "_test.go") {
			seen[p] = true
		}
	}

	for _, base := range []string{"origin/main", "main"} {
		mb, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", base)
		if code != 0 {
			continue
		}
		out, _, code, _ := acsassert.SubprocessOutput(
			"git", "-C", root, "diff", "--name-only", "--diff-filter=A", strings.TrimSpace(mb), "HEAD")
		if code == 0 {
			for _, line := range strings.Split(out, "\n") {
				add(line)
			}
			break
		}
	}

	if out, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain"); code == 0 {
		for _, line := range strings.Split(out, "\n") {
			if len(line) < 4 {
				continue
			}
			if strings.HasPrefix(line, "A") || strings.HasPrefix(line, "??") {
				add(line[3:])
			}
		}
	}

	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

var recordedAddedTests = []string{
	"go/acs/cycle1676/predicates_test.go",
	"go/acs/cycle1679/predicates_test.go",
}

func TestC1679_007_EveryAddedGoTestPackageIsGreenBeforeShip(t *testing.T) {
	root := acsassert.RepoRoot(t)
	added := addedGoTestFiles(t, root)
	if len(added) == 0 {
		for _, rel := range recordedAddedTests {
			if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
				t.Fatalf("RED (audit M1): the live added-test seed is empty (merged tree) and the recorded added file %s is missing (%v) — the lane's own tag-guarded package is gone", rel, err)
			}
		}
		added = recordedAddedTests
		t.Logf("live added-test seed empty (the lane's commit is on main) — verifying the recorded added set %v", added)
	}

	pkgsByTagKey := map[string]map[string]bool{}
	tagsByKey := map[string][]string{}
	var excluded, untagged []string
	for _, rel := range added {
		pkg := "./" + filepath.ToSlash(filepath.Dir(strings.TrimPrefix(rel, "go/")))
		if pkg == selfPkg {
			continue
		}
		tags, runnable := buildTagsOf(t, filepath.Join(root, rel))
		if !runnable {
			excluded = append(excluded, rel)
			continue
		}
		if len(tags) == 0 {
			untagged = append(untagged, pkg)
			continue
		}
		key := strings.Join(tags, ",")
		if pkgsByTagKey[key] == nil {
			pkgsByTagKey[key] = map[string]bool{}
			tagsByKey[key] = tags
		}
		pkgsByTagKey[key][pkg] = true
	}
	for _, rel := range excluded {
		t.Logf("excluded %s (requires_tmux or another build constraint unavailable on this host)", rel)
	}
	sort.Strings(untagged)
	for _, pkg := range untagged {
		t.Logf("deferred %s to the mandatory `go test -count=1 ./...` (untagged: an ordinary run already executes it)", pkg)
	}
	if len(pkgsByTagKey) == 0 {
		t.Fatalf("RED (audit M1): no TAG-GUARDED added test package was found to verify, but this lane "+
			"adds go/acs/cycle1676/predicates_test.go (`//go:build acs`). A tag-guarded package that no "+
			"gate runs in-cycle is exactly the hole M1 named. Seed was: %v", added)
	}

	keys := make([]string, 0, len(pkgsByTagKey))
	for key := range pkgsByTagKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		pkgs := make([]string, 0, len(pkgsByTagKey[key]))
		for pkg := range pkgsByTagKey[key] {
			pkgs = append(pkgs, pkg)
		}
		sort.Strings(pkgs)

		args := append([]string{"-C", goModRoot(t), "test", "-count=1",
			"-tags", strings.Join(tagsByKey[key], ",")}, pkgs...)

		stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
		if code == -1 {
			t.Fatalf("go test failed to launch for %v (tags %q): %v\nstderr:\n%s", pkgs, key, err, stderr)
		}
		if code != 0 {
			t.Errorf("RED (audit M1): added test package(s) %v are NOT green under tags %q — the "+
				"repo-contract added-test backstop will block the push at ship. Running it here, in "+
				"cycle, is the point: the previous round CLAIMED these were re-verified and no "+
				"invocation ever ran.\nexit=%d\ncombined go-test output:\n%s",
				pkgs, key, code, stdout+stderr)
		}
	}
}

var numberWords = map[string]int{
	"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}

func parseCount(tok string) (int, bool) {
	tok = strings.ToLower(strings.Trim(tok, "*_`.,"))
	if n, ok := numberWords[tok]; ok {
		return n, true
	}
	var n int
	if _, err := fmt.Sscanf(tok, "%d", &n); err == nil {
		return n, true
	}
	return 0, false
}

func TestC1679_008_ExplanationDocumentCountsAgreeWithTheShippedTree(t *testing.T) {
	root := acsassert.RepoRoot(t)

	predFile := filepath.Join(root, "go", "acs", "cycle1679", "predicates_test.go")
	src, err := os.ReadFile(predFile)
	if err != nil {
		t.Fatalf("read %s: %v", predFile, err)
	}
	actual := len(regexp.MustCompile(`(?m)^func TestC1679_\d+_`).FindAllString(string(src), -1))
	if actual == 0 {
		t.Fatalf("RED: found 0 TestC1679_* declarations in %s — the discovery is broken, and a "+
			"vacuous pass here would reproduce the very unearned claim M1 named", predFile)
	}

	matches, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-1679-*.md"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("RED: no explanation document matched docs/explain/builds/cycle-1679-*.md (err=%v) — "+
			"the cycle's explanation contract names one as required", err)
	}

	for _, abs := range matches {
		rel, relErr := filepath.Rel(root, abs)
		if relErr != nil {
			rel = abs
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("RED: %s is untracked — it may be gitignored and dropped at ship (cycle-93)", rel)
		}

		raw, readErr := os.ReadFile(abs)
		if readErr != nil {
			t.Fatalf("read %s: %v", rel, readErr)
		}
		flat := strings.Join(strings.Fields(string(raw)), " ")

		for _, m := range regexp.MustCompile(`(?i)(\S+) acceptance predicates`).FindAllStringSubmatch(flat, -1) {
			claimed, ok := parseCount(m[1])
			if !ok {
				continue
			}
			if claimed != actual {
				t.Errorf("RED (audit M1): %s claims %q but the shipped tree carries %d "+
					"TestC1679_* predicates. A cycle-owned explanation whose arithmetic disagrees "+
					"with its own tree is the claim-discrepancy class the explanation contract exists "+
					"to catch. Update the narrative to the tree; never the reverse.", rel, m[0], actual)
			}
		}

		for _, m := range regexp.MustCompile(`(?i)predicates are (\d+)/(\d+) PASS`).FindAllStringSubmatch(flat, -1) {
			passed, total := 0, 0
			fmt.Sscanf(m[1], "%d", &passed)
			fmt.Sscanf(m[2], "%d", &total)
			if total != actual {
				t.Errorf("RED (audit M1): %s asserts %q over a tree carrying %d predicates — the "+
					"document reports a green suite whose size it gets wrong.", rel, m[0], actual)
			}
			if passed != total {
				t.Errorf("RED (audit M1): %s asserts %q — a cycle must not ship an explanation "+
					"claiming a partially-red acceptance suite is verified.", rel, m[0])
			}
		}
	}
}
