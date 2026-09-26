//go:build acs

// Package cycle1694 materialises the cycle-1694 acceptance criteria for the one
// fleet-scoped inbox id `atomicwrite-linked-state-sweep` (step 3 of
// `statejson-latent-unresolved-writers`).
//
// The defect (latent, cycle-999 class). core/worktree.go linkGuardDeps symlinks
// exactly three state files into every cycle worktree: .evolve/state.json and
// .evolve/ledger.jsonl (to the canonical host files) and .evolve/cycle-state.json
// (to the run's run.json mirror). The adapters/storage writers WriteState,
// WriteCycleState and UpdateState all end in the private writeJSONAtomic, which
// tmp+renames onto the UNRESOLVED path. A rename over a symlink replaces the LINK
// with a regular file — the cycle-999 sever — and every later write strands in
// the detached copy. statemap.WriteStateMap and core.SealCycle already resolve
// the target first (cycle 1690); storage does not.
//
// The accepted fix: route the storage writers' write target through
// statemap.ResolveWriteTarget, keeping storage and statemap as separate paths
// (statemap.go:1-21) — only the write target is resolved.
//
// Predicate strategy — every predicate exercises the system under test (the
// cycle-85 degenerate-predicate ban) and every fixture lives under t.TempDir()
// (the worktree's own .evolve/state.json IS a live link to the host state file,
// so no predicate may ever touch a repo-relative .evolve path):
//
//   - 001/002 drive storage.WriteState through absolute, relative and two-hop
//     links (plus a regular-file baseline) and through a dangling link.
//   - 003 drives storage.UpdateState's locked lossless RMW through every link
//     shape, including dangling.
//   - 004 drives storage.WriteCycleState through links to a canonical
//     cycle-state.json and through the exact linkGuardDeps topology (worktree
//     cycle-state.json -> the run's run.json, dangling until the first write).
//   - 005 is the separation negative: through a link, the storage writers must
//     keep their own semantics (no statemap CAS refusal, no statemapRevision
//     bump) — rerouting storage through statemap.WriteStateMap fails it.
//   - 006 proves the durable in-package regression tests ran, passed per link
//     shape, and are git-tracked (a `-run` matching nothing exits 0).
//   - 007 proves those durable tests are RED on the pre-fix code: it
//     neutralises every statemap.ResolveWriteTarget reference in the storage
//     package through a `go test -overlay` and requires every shape to fail.
//   - 008 is the no-regression floor for the one touched package.
//
// Audit repair (round 2). Round 1's audit found the code correct but FAILed the
// cycle on two things the predicates above never observed:
//
//   - 009 is AC3 / audit M1: build-report.md carried no atomicwrite-caller
//     inventory. It measures the inventory at the base commit and requires the
//     report to record it: the three linked files, the measured count, "none
//     targets a linked file", and "not migrated".
//   - 010 is AC3's "do not migrate them": while the cycle is live, no
//     atomicwrite caller changed and no production file outside storage/statemap
//     gained a ResolveWriteTarget reference.
//   - 011 is the host predicate-execution gate (audit/predicate_authority.go):
//     while the cycle is live, the full worktree tree must equal the tracked
//     ship tree. Round 1 left this predicate file and the eval untracked, so
//     the gate refused to run the suite and acs-verdict.json was never written.
package cycle1694

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// storagePkg is the one package this lane changes.
const storagePkg = "internal/adapters/storage"

// statemapImport is the resolver's home; 007 neutralises references to it.
const statemapImport = "github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"

// cycleStateEnv is ipcenv.CycleStateFileKey. The fleet orchestrator sets it to
// the LIVE lane's cycle-state (core/cyclerun.go); left set, WriteCycleState
// would write the running cycle's state instead of the fixture.
const cycleStateEnv = "EVOLVE_CYCLE_STATE_FILE"

// durableTests are the in-package regression tests the eval file names as
// permanent evidence (ACS predicates are cycle-scoped). Each must carry one
// subtest per link shape in durableShapes.
var durableTests = []string{
	"TestWriteState_WritesThroughSymlinkedStatePath",
	"TestUpdateState_WritesThroughSymlinkedStatePath",
	"TestWriteCycleState_WritesThroughSymlinkedCycleStatePath",
}

// durableShapes are the subtest names AC2 requires for every migrated writer.
var durableShapes = []string{"absolute", "relative", "dangling"}

// ---------------------------------------------------------------------------
// 001-002 — WriteState
// ---------------------------------------------------------------------------

// TestC1694_001_WriteStateWritesThroughLinkedStateJSON: a WriteState rooted at
// an evolve dir whose state.json is a link must keep every hop a symlink and
// land the new bytes on the canonical file. The regular-file row is the
// baseline that must stay green.
func TestC1694_001_WriteStateWritesThroughLinkedStateJSON(t *testing.T) {
	for _, shape := range linkShapes() {
		t.Run(shape.name, func(t *testing.T) {
			root := t.TempDir()
			canonical := writeFile(t, filepath.Join(root, "canon", ".evolve", "state.json"), `{"lastCycleNumber":1,"version":1}`)
			entry, hops := shape.build(t, root, canonical, "state.json")

			in := core.State{LastCycleNumber: 1694, Version: 2}
			if err := storage.New(filepath.Dir(entry)).WriteState(context.Background(), in); err != nil {
				t.Fatalf("WriteState via %s: %v", entry, err)
			}
			for _, h := range hops {
				assertLinkIntact(t, h.path, h.target)
			}
			got := readStateFile(t, canonical)
			if numField(got, "lastCycleNumber") != 1694 || numField(got, "version") != 2 {
				t.Errorf("canonical %s did not receive the write through %s: %v", canonical, shape.name, got)
			}
		})
	}
}

// TestC1694_002_WriteStateThroughDanglingLinkCreatesCanonical: linkGuardDeps
// documents that a link "may briefly dangle". A write through a dangling link
// must create the TARGET and leave the link in place — never replace it.
func TestC1694_002_WriteStateThroughDanglingLinkCreatesCanonical(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(mkdir(t, filepath.Join(root, "canon", ".evolve")), "state.json")
	link := symlink(t, canonical, filepath.Join(root, "wt", ".evolve", "state.json"))

	if err := storage.New(filepath.Dir(link)).WriteState(context.Background(), core.State{LastCycleNumber: 1694}); err != nil {
		t.Fatalf("WriteState via dangling %s: %v", link, err)
	}
	assertLinkIntact(t, link, canonical)
	if _, err := os.Stat(canonical); err != nil {
		t.Fatalf("canonical %s was not created through the dangling link: %v", canonical, err)
	}
	if got := readStateFile(t, canonical); numField(got, "lastCycleNumber") != 1694 {
		t.Errorf("canonical %s holds %v, want lastCycleNumber 1694", canonical, got)
	}
}

// ---------------------------------------------------------------------------
// 003 — UpdateState
// ---------------------------------------------------------------------------

// TestC1694_003_UpdateStateWritesThroughLinkedStateJSON drives the locked
// lossless RMW through every link shape. It must read the canonical state
// through the link, bump stateRevision from the CANONICAL value, keep unmodelled
// operator keys, write the result back to canonical and keep every hop a link.
func TestC1694_003_UpdateStateWritesThroughLinkedStateJSON(t *testing.T) {
	shapes := append(linkShapes(), linkShape{"dangling-link", func(t *testing.T, root, canonical, name string) (string, []hop) {
		if err := os.Remove(canonical); err != nil {
			t.Fatalf("unseed %s: %v", canonical, err)
		}
		link := symlink(t, canonical, filepath.Join(root, "wt", ".evolve", name))
		return link, []hop{{link, canonical}}
	}})
	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			root := t.TempDir()
			canonical := writeFile(t, filepath.Join(root, "canon", ".evolve", "state.json"),
				`{"lastCycleNumber":5,"stateRevision":7,"operatorOwnedKey":"must-survive"}`)
			entry, hops := shape.build(t, root, canonical, "state.json")
			seeded := shape.name != "dangling-link"

			st, err := storage.New(filepath.Dir(entry)).UpdateState(context.Background(), func(s *core.State) {
				s.LastCycleNumber = 1694
			})
			if err != nil {
				t.Fatalf("UpdateState via %s: %v", entry, err)
			}
			wantRev := 1.0
			if seeded {
				wantRev = 8
			}
			if float64(st.StateRevision) != wantRev {
				t.Errorf("UpdateState returned stateRevision %d, want %v (bumped from the canonical value read through the link)", st.StateRevision, wantRev)
			}
			for _, h := range hops {
				assertLinkIntact(t, h.path, h.target)
			}
			got := readStateFile(t, canonical)
			if numField(got, "lastCycleNumber") != 1694 || numField(got, "stateRevision") != wantRev {
				t.Errorf("canonical %s did not receive the RMW through %s: %v", canonical, shape.name, got)
			}
			if seeded && got["operatorOwnedKey"] != "must-survive" {
				t.Errorf("an unmodelled operator key did not survive UpdateState through %s: %v", shape.name, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 004 — WriteCycleState
// ---------------------------------------------------------------------------

// TestC1694_004_WriteCycleStateWritesThroughLinkedCycleState covers the third
// linked file. Rows 1-3 link the evolve dir's cycle-state.json to a canonical
// one (absolute, relative, dangling); the checkpoint block on canonical must be
// spliced through. Row 4 is the production topology linkGuardDeps builds: the
// worktree's cycle-state.json is an absolute link to <workspace>/run.json that
// dangles until the first WriteCycleState.
func TestC1694_004_WriteCycleStateWritesThroughLinkedCycleState(t *testing.T) {
	const seed = `{"cycle_id":1694,"phase":"tdd","workspace_path":"","checkpoint":{"resumeFrom":"tdd","marker":"keep-me"}}`
	rows := []struct {
		name     string
		relative bool
		seeded   bool
	}{
		{"absolute-link", false, true},
		{"relative-link", true, true},
		{"dangling-link", false, false},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			t.Setenv(cycleStateEnv, "")
			root := t.TempDir()
			canonical := filepath.Join(mkdir(t, filepath.Join(root, "canon", ".evolve")), "cycle-state.json")
			if r.seeded {
				writeFile(t, canonical, seed)
			}
			target := canonical
			if r.relative {
				target = filepath.Join("..", "..", "canon", ".evolve", "cycle-state.json")
			}
			link := symlink(t, target, filepath.Join(root, "wt", ".evolve", "cycle-state.json"))
			ws := mkdir(t, filepath.Join(root, "runs", "cycle-1694"))

			cs := core.CycleState{CycleID: 1694, Phase: "build", WorkspacePath: ws}
			if err := storage.New(filepath.Dir(link)).WriteCycleState(context.Background(), cs); err != nil {
				t.Fatalf("WriteCycleState via %s: %v", link, err)
			}
			assertLinkIntact(t, link, target)
			got := readStateFile(t, canonical)
			if got["phase"] != "build" {
				t.Errorf("canonical %s did not receive the write through %s: %v", canonical, r.name, got)
			}
			cp, _ := got["checkpoint"].(map[string]any)
			if r.seeded && (cp == nil || cp["marker"] != "keep-me") {
				t.Errorf("canonical checkpoint block was not spliced through %s: %v", r.name, got["checkpoint"])
			}
			if mirror := readStateFile(t, filepath.Join(ws, core.RunStateFile)); mirror["phase"] != "build" {
				t.Errorf("run.json mirror regressed: %v", mirror)
			}
		})
	}

	t.Run("linkGuardDeps-run-mirror", func(t *testing.T) {
		t.Setenv(cycleStateEnv, "")
		root := t.TempDir()
		ws := mkdir(t, filepath.Join(root, "canon", ".evolve", "runs", "cycle-1694"))
		runJSON := filepath.Join(ws, core.RunStateFile)
		link := symlink(t, runJSON, filepath.Join(root, "wt", ".evolve", "cycle-state.json"))

		cs := core.CycleState{CycleID: 1694, Phase: "build", WorkspacePath: ws}
		if err := storage.New(filepath.Dir(link)).WriteCycleState(context.Background(), cs); err != nil {
			t.Fatalf("WriteCycleState via %s: %v", link, err)
		}
		assertLinkIntact(t, link, runJSON)
		if got := readStateFile(t, runJSON); got["phase"] != "build" {
			t.Errorf("run.json %s does not hold the write: %v", runJSON, got)
		}
	})
}

// ---------------------------------------------------------------------------
// 005 — storage stays off the statemap path
// ---------------------------------------------------------------------------

// TestC1694_005_StorageWritersKeepTheirOwnSemanticsThroughALink is the
// separation negative (AC1: "Keep storage and statemap as separate paths; only
// resolve the write target"). Through a link, WriteState is still a typed
// replace with no CAS floor (statemap.WriteStateMap would refuse a lower
// stateRevision with ErrStaleRevision), and UpdateState owns stateRevision and
// leaves statemap's own statemapRevision counter untouched (UpdateStateMap
// would bump it).
func TestC1694_005_StorageWritersKeepTheirOwnSemanticsThroughALink(t *testing.T) {
	const seed = `{"lastCycleNumber":1,"stateRevision":10,"statemapRevision":4,"lastUpdated":"2026-07-14T05:01:14Z"}`

	t.Run("WriteState-no-cas-floor", func(t *testing.T) {
		root := t.TempDir()
		canonical := writeFile(t, filepath.Join(root, "canon", ".evolve", "state.json"), seed)
		link := symlink(t, canonical, filepath.Join(root, "wt", ".evolve", "state.json"))

		err := storage.New(filepath.Dir(link)).WriteState(context.Background(), core.State{LastCycleNumber: 1694, StateRevision: 3})
		if err != nil {
			t.Fatalf("WriteState of a lower stateRevision through a link was refused (%v) — storage must not inherit statemap's CAS floor", err)
		}
		assertLinkIntact(t, link, canonical)
		got := readStateFile(t, canonical)
		if numField(got, "stateRevision") != 3 || numField(got, "lastCycleNumber") != 1694 {
			t.Errorf("canonical does not hold the typed replace: %v", got)
		} else if _, ok := got["statemapRevision"]; ok {
			t.Errorf("canonical kept statemapRevision — the write went through statemap's map writer, not storage's typed replace: %v", got)
		}
	})

	t.Run("UpdateState-owns-stateRevision-only", func(t *testing.T) {
		root := t.TempDir()
		canonical := writeFile(t, filepath.Join(root, "canon", ".evolve", "state.json"), seed)
		link := symlink(t, canonical, filepath.Join(root, "wt", ".evolve", "state.json"))

		if _, err := storage.New(filepath.Dir(link)).UpdateState(context.Background(), func(s *core.State) {
			s.LastCycleNumber = 1694
		}); err != nil {
			t.Fatalf("UpdateState via %s: %v", link, err)
		}
		assertLinkIntact(t, link, canonical)
		got := readStateFile(t, canonical)
		if numField(got, "stateRevision") != 11 {
			t.Errorf("stateRevision = %v, want 11 (storage.UpdateState's own ++ over the canonical 10)", got["stateRevision"])
		}
		if numField(got, "statemapRevision") != 4 {
			t.Errorf("statemapRevision = %v, want 4 untouched — UpdateState went through statemap.UpdateStateMap", got["statemapRevision"])
		}
		if got["lastUpdated"] != "2026-07-14T05:01:14Z" {
			t.Errorf("lastUpdated = %v, want the canonical value untouched (the mutate did not set it)", got["lastUpdated"])
		}
	})
}

// ---------------------------------------------------------------------------
// 006-007 — durable regression tests: they run, they pass, they are RED pre-fix
// ---------------------------------------------------------------------------

// TestC1694_006_DurableRegressionTestsRanPassedAndAreTracked: each durable test
// must run and PASS once per link shape (a -run matching nothing still exits
// 0), and the file declaring it must be git-TRACKED (an unadded file is dropped
// at ship).
func TestC1694_006_DurableRegressionTestsRanPassedAndAreTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	out, errOut, code := runIn(t, goDir, "go", "test", "-count=1", "-v", "-run", durableRunPattern(), "./"+storagePkg)
	if code != 0 {
		t.Fatalf("go test -run %s ./%s exited %d\nstdout:\n%s\nstderr:\n%s", durableRunPattern(), storagePkg, code, out, errOut)
	}
	for _, name := range durableTests {
		for _, shape := range durableShapes {
			if !strings.Contains(out, "--- PASS: "+name+"/"+shape+" ") {
				t.Errorf("%s/%s never ran and passed in ./%s — AC2 needs the %s-link case for this writer", name, shape, storagePkg, shape)
			}
		}
		decl := regexp.MustCompile(`(?m)^func ` + regexp.QuoteMeta(name) + `\(t \*testing\.T\)`)
		carriers := testFilesMatching(t, filepath.Join(goDir, storagePkg), decl)
		if len(carriers) == 0 {
			t.Errorf("no _test.go in ./%s declares func %s(t *testing.T)", storagePkg, name)
			continue
		}
		for _, f := range carriers {
			rel, err := filepath.Rel(root, f)
			if err != nil {
				t.Fatalf("relativise %s: %v", f, err)
			}
			if _, _, code := runIn(t, root, "git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
				t.Errorf("%s is UNTRACKED — a test file that is not added is dropped at ship", rel)
			}
		}
	}
}

// TestC1694_007_DurableTestsAreRedWithoutTheResolver is AC2's "RED on the
// pre-fix code" made executable, and AC1's "through statemap.ResolveWriteTarget"
// made load-bearing. It rewrites every statemap.ResolveWriteTarget reference in
// the storage package's production files to an identity function (the pre-fix
// behaviour), compiles that through `go test -overlay` without touching the
// tree, and requires every durable test to FAIL on every link shape. No
// reference at all means the writers do not go through the shared resolver.
func TestC1694_007_DurableTestsAreRedWithoutTheResolver(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	overlay, sites := neutraliseResolver(t, filepath.Join(goDir, storagePkg), t.TempDir())
	if sites == 0 {
		t.Fatalf("no production file in ./%s references statemap.ResolveWriteTarget — AC1 requires the storage writers to resolve their write target through it (reuse, do not re-implement)", storagePkg)
	}
	out, errOut, code := runIn(t, goDir, "go", "test", "-count=1", "-v", "-overlay="+overlay, "-run", durableRunPattern(), "./"+storagePkg)
	if strings.Contains(out, "[build failed]") || strings.Contains(out, "[setup failed]") || strings.Contains(errOut, "[build failed]") {
		t.Fatalf("the resolver-neutralised storage package did not compile — the durable tests must build against the pre-fix production code\nstdout:\n%s\nstderr:\n%s", out, errOut)
	}
	if code == 0 {
		t.Errorf("durable tests PASSED with statemap.ResolveWriteTarget neutralised at %d site(s) — they do not detect the cycle-999 sever\nstdout:\n%s", sites, out)
	}
	for _, name := range durableTests {
		for _, shape := range durableShapes {
			if !strings.Contains(out, "--- FAIL: "+name+"/"+shape+" ") {
				t.Errorf("%s/%s did not FAIL on the pre-fix (resolver-neutralised) code — it is not a regression test for the %s-link sever", name, shape, shape)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 008 — no-regression floor
// ---------------------------------------------------------------------------

// TestC1694_008_StoragePackageGreenVetAndGofmtClean: the touched package's
// suite stays green, go vet is clean (it also compiles the new storage ->
// statemap edge, so an import cycle surfaces here), and the tree is
// gofmt-clean.
func TestC1694_008_StoragePackageGreenVetAndGofmtClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	if out, errOut, code := runIn(t, goDir, "go", "test", "-count=1", "./"+storagePkg); code != 0 {
		t.Errorf("go test ./%s exited %d\nstdout:\n%s\nstderr:\n%s", storagePkg, code, out, errOut)
	}
	if out, errOut, code := runIn(t, goDir, "go", "vet", "./"+storagePkg); code != 0 {
		t.Errorf("go vet ./%s exited %d\nstdout:\n%s\nstderr:\n%s", storagePkg, code, out, errOut)
	}
	listed, errOut, code := runIn(t, goDir, "gofmt", "-l", storagePkg)
	if code != 0 {
		t.Fatalf("gofmt -l exited %d: %s", code, errOut)
	}
	if strings.TrimSpace(listed) != "" {
		t.Errorf("gofmt -l is not empty:\n%s", listed)
	}
}

// ---------------------------------------------------------------------------
// 009-011 — audit repair: AC3 inventory, no migration, complete ship tree
// ---------------------------------------------------------------------------

// baseSHA is the cycle's base commit (the explanation handoff's base_sha). The
// worktree HEAD stays on it until ship, because phases never commit.
const baseSHA = "4a2103349a79bde1f8cbc2c96e84c975cc7f8f99"

// thisCycle names the run workspace whose build-report.md AC3 grades.
const thisCycle = 1694

// linkedStateFiles are the three files core/worktree.go linkGuardDeps symlinks
// into every cycle worktree.
var linkedStateFiles = []string{"state.json", "ledger.jsonl", "cycle-state.json"}

// atomicwriteCallPattern is the inventory's call-site shape (AC3, audit M1).
const atomicwriteCallPattern = `atomicwrite\.(Bytes|JSON)\(`

// inventoryPathspec scopes the inventory to production code: no tests, no ACS
// predicates, not the atomicwrite package itself.
var inventoryPathspec = []string{"--", "go/*.go", ":(exclude)*_test.go", ":(exclude)go/acs/**", ":(exclude)go/internal/atomicwrite/**"}

var (
	inventoryHeadingRe = regexp.MustCompile(`(?i)^(#{2,3})\s+.*atomicwrite.*inventory`)
	noneTargetsRe      = regexp.MustCompile(`(?i)\b(none|no|zero)\b[^\n]{0,120}\b(target|write|name|touch|reach)`)
	notMigratedRe      = regexp.MustCompile(`(?i)(\b(not|no)\b[^\n]{0,60}\bmigrat|\bunmigrated\b)`)
)

// TestC1694_009_BuildReportRecordsTheAtomicwriteCallerInventory is AC3 ("the
// atomicwrite-caller inventory is recorded in the build report") and audit M1.
// It measures the inventory from the base commit's object store, so the count
// is fixed forever. It checks the premise, that no call line names a linked
// file. It then requires this run's build-report.md to carry an inventory
// section that records the three linked files, the measured count, that none
// targets a linked file, and that the callers were not migrated.
func TestC1694_009_BuildReportRecordsTheAtomicwriteCallerInventory(t *testing.T) {
	root := acsassert.RepoRoot(t)
	calls := atomicwriteInventory(t, root, baseSHA)
	for _, c := range calls {
		for _, f := range linkedStateFiles {
			if linkedFileRe(f).MatchString(c) {
				t.Errorf("AC3's premise is broken: %s names %s. That caller must be migrated, not merely recorded", c, f)
			}
		}
	}

	report := filepath.Join(cycleRunDir(t), "build-report.md")
	raw, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("read %s: %v. AC3 requires the inventory to be recorded IN the build report", report, err)
	}
	section, ok := markdownSection(string(raw), inventoryHeadingRe)
	if !ok {
		t.Fatalf("%s has no atomicwrite-caller inventory section (a `## ` heading naming both \"atomicwrite\" and \"inventory\"). This is audit M1, AC3 under-delivered", report)
	}
	for _, f := range linkedStateFiles {
		if !linkedFileRe(f).MatchString(section) {
			t.Errorf("the inventory section does not name the linked file %s (core/worktree.go linkGuardDeps)", f)
		}
	}
	if n := len(calls); !countStatedRe(n).MatchString(section) {
		t.Errorf("the inventory section does not state the measured count %d (production lines matching %s at %s) next to the word call/site/line/caller", n, atomicwriteCallPattern, baseSHA[:8])
	}
	if !noneTargetsRe.MatchString(section) {
		t.Errorf("the inventory section does not state that none of the callers targets a linked file")
	}
	if !notMigratedRe.MatchString(section) {
		t.Errorf("the inventory section does not state that the callers were not migrated (AC3: \"record it, do not migrate them\")")
	}
}

// TestC1694_010_AtomicwriteCallersAreNotMigrated is AC3's negative axis: "do
// not migrate them". While the cycle is live, the atomicwrite package and every
// production call line are unchanged since base. No production file outside
// storage and statemap gains a ResolveWriteTarget reference.
func TestC1694_010_AtomicwriteCallersAreNotMigrated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	requireLiveCycle(t, root)

	if out, errOut, code := runIn(t, root, "git", "-C", root, "diff", "--name-only", baseSHA, "--", "go/internal/atomicwrite"); code != 0 || strings.TrimSpace(out) != "" {
		t.Errorf("go/internal/atomicwrite changed since base (exit %d): %s%s. AC3 says record the callers, do not migrate them", code, out, errOut)
	}
	before := atomicwriteInventory(t, root, baseSHA)
	after := atomicwriteInventory(t, root, "")
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("the production atomicwrite call lines changed since base (%d -> %d). AC3 says do not migrate them\nbase:\n%s\nnow:\n%s",
			len(before), len(after), strings.Join(before, "\n"), strings.Join(after, "\n"))
	}

	changed, errOut, code := runIn(t, root, "git", "-C", root, "diff", "--name-only", baseSHA, "--", "go")
	if code != 0 {
		t.Fatalf("git diff --name-only %s -- go exited %d: %s", baseSHA, code, errOut)
	}
	for _, rel := range strings.Fields(changed) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "go/acs/") ||
			strings.HasPrefix(rel, "go/internal/adapters/storage/") || strings.HasPrefix(rel, "go/internal/adapters/statemap/") {
			continue
		}
		baseSrc, _, _ := runIn(t, root, "git", "-C", root, "show", baseSHA+":"+rel)
		nowSrc, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("read %s: %v", rel, err)
		}
		if b, n := strings.Count(baseSrc, "ResolveWriteTarget"), strings.Count(string(nowSrc), "ResolveWriteTarget"); n > b {
			t.Errorf("%s gained ResolveWriteTarget references (%d -> %d). Only the storage writers are in scope; AC3 says the atomicwrite callers are recorded, not migrated", rel, b, n)
		}
	}
}

// TestC1694_011_PredicateInputsAreInTheShipTree encodes the gate that refused
// round 1's audit (go/internal/phases/audit/predicate_authority.go
// predicateTreeFor). The tree the predicates execute (every non-ignored file)
// must equal the tracked ship tree. An untracked predicate, eval or test file is
// an "undeclared input absent from the ship tree". The suite then never runs and
// acs-verdict.json is never written. It calls the gate's own two treefence
// snapshots.
func TestC1694_011_PredicateInputsAreInTheShipTree(t *testing.T) {
	root := acsassert.RepoRoot(t)
	requireLiveCycle(t, root)

	ctx := context.Background()
	full, err := treefence.Take(ctx, root)
	if err != nil {
		t.Fatalf("treefence.Take(%s): %v", root, err)
	}
	tracked, err := treefence.TakeTracked(ctx, root)
	if err != nil {
		t.Fatalf("treefence.TakeTracked(%s): %v", root, err)
	}
	if full.Tree != tracked.Tree {
		untracked, _, _ := runIn(t, root, "git", "-C", root, "ls-files", "--others", "--exclude-standard")
		t.Errorf("the predicate execution tree (%s) != the ship tree (%s). These untracked, non-ignored files are undeclared inputs; `git add` each one you intend to ship, or remove it:\n%s",
			full.Tree, tracked.Tree, untracked)
	}
}

// atomicwriteInventory lists the production atomicwrite.Bytes/JSON call lines
// as "<path>:<text>". Line numbers are left out, so an edit elsewhere in a
// caller's file does not count as a changed call. With a rev, it reads from the
// rev's object store. With rev == "", it reads the working tree, untracked files
// included.
func atomicwriteInventory(t *testing.T, root, rev string) []string {
	t.Helper()
	args := []string{"-C", root, "grep", "--untracked", "-E", atomicwriteCallPattern}
	if rev != "" {
		if _, _, code := runIn(t, root, "git", "-C", root, "cat-file", "-e", rev+"^{commit}"); code != 0 {
			t.Skipf("base commit %s is not in this clone (shallow or pruned); the inventory cannot be measured", rev)
		}
		args = []string{"-C", root, "grep", "-E", atomicwriteCallPattern, rev}
	}
	out, errOut, code := runIn(t, root, "git", append(args, inventoryPathspec...)...)
	if code != 0 {
		t.Fatalf("git grep for the atomicwrite inventory (rev %q) exited %d: %s. Zero call sites means the scan is broken, not that the inventory is empty", rev, code, errOut)
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		lines = append(lines, strings.TrimPrefix(l, rev+":"))
	}
	return lines
}

// requireLiveCycle skips once this cycle has shipped. The worktree HEAD is the
// base commit until ship, and later trees legitimately move past it.
func requireLiveCycle(t *testing.T, root string) {
	t.Helper()
	head, errOut, code := runIn(t, root, "git", "-C", root, "rev-parse", "HEAD")
	if code != 0 {
		t.Fatalf("git rev-parse HEAD exited %d: %s", code, errOut)
	}
	if h := strings.TrimSpace(head); h != baseSHA {
		t.Skipf("HEAD %s is not cycle %d's base %s: the cycle has shipped, and this live-cycle check is historical", h, thisCycle, baseSHA)
	}
}

// cycleRunDir resolves this cycle's workspace on the STATE root:
// EVOLVE_PROJECT_ROOT (the acs suite sets it), else the project root that owns
// a lane worktree at <root>/.evolve/worktrees/<lane>, else the repo root. An
// absent workspace (an archived run) is the documented SKIP posture, never a
// false red.
func cycleRunDir(t *testing.T) string {
	t.Helper()
	stateRoot := os.Getenv("EVOLVE_PROJECT_ROOT")
	if stateRoot == "" {
		stateRoot = acsassert.RepoRoot(t)
		marker := string(filepath.Separator) + filepath.Join(".evolve", "worktrees") + string(filepath.Separator)
		if i := strings.Index(stateRoot, marker); i >= 0 {
			stateRoot = stateRoot[:i]
		}
	}
	dir := filepath.Join(stateRoot, ".evolve", "runs", "cycle-"+strconv.Itoa(thisCycle))
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Skipf("cycle workspace %s absent (runtime state not present): %v", dir, err)
	}
	return dir
}

// markdownSection returns the body under the first heading matching re, up to
// the next heading of the same or a higher level. Fenced code blocks are
// skipped, so a `# comment` in a pasted shell block does not end the section.
func markdownSection(doc string, re *regexp.Regexp) (string, bool) {
	var b strings.Builder
	level, inFence := 0, false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
		}
		if !inFence {
			if level == 0 {
				if m := re.FindStringSubmatch(line); m != nil {
					level = len(m[1])
				}
				continue
			}
			if h := len(line) - len(strings.TrimLeft(line, "#")); h > 0 && h <= level && strings.HasPrefix(line[h:], " ") {
				break
			}
		}
		if level > 0 {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String(), level > 0
}

// linkedFileRe matches a linked file's name as a whole token. `state.json`
// must not match inside `cycle-state.json`.
func linkedFileRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(name))
}

// countStatedRe matches n as a standalone integer beside a call-site noun, in
// either order ("53 call sites", "call sites: 53").
func countStatedRe(n int) *regexp.Regexp {
	num := `(^|[^0-9])` + strconv.Itoa(n) + `([^0-9]|$)`
	noun := `(?i:call|site|line|caller)`
	return regexp.MustCompile(num + `[^\n]{0,40}` + noun + `|` + noun + `[^\n]{0,40}` + num)
}

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

type hop struct{ path, target string }

type linkShape struct {
	name  string
	build func(t *testing.T, root, canonical, name string) (entry string, hops []hop)
}

// linkShapes lay a worktree-view evolve dir over root/canon/.evolve/<name>. The
// entry is always <dir>/<name>, the file name the storage writer derives.
func linkShapes() []linkShape {
	return []linkShape{
		{"regular-file", func(t *testing.T, root, canonical, name string) (string, []hop) {
			return canonical, nil
		}},
		{"absolute-link", func(t *testing.T, root, canonical, name string) (string, []hop) {
			link := symlink(t, canonical, filepath.Join(root, "wt", ".evolve", name))
			return link, []hop{{link, canonical}}
		}},
		{"relative-link", func(t *testing.T, root, canonical, name string) (string, []hop) {
			rel := filepath.Join("..", "..", "canon", ".evolve", name)
			link := symlink(t, rel, filepath.Join(root, "wt", ".evolve", name))
			return link, []hop{{link, rel}}
		}},
		{"two-hop-chain", func(t *testing.T, root, canonical, name string) (string, []hop) {
			rel := filepath.Join("..", "..", "canon", ".evolve", name)
			mid := symlink(t, rel, filepath.Join(root, "mid", ".evolve", name))
			link := symlink(t, mid, filepath.Join(root, "wt", ".evolve", name))
			return link, []hop{{link, mid}, {mid, rel}}
		}},
	}
}

func durableRunPattern() string {
	return "^(" + strings.Join(durableTests, "|") + ")$"
}

// identityResolver replaces a statemap.ResolveWriteTarget reference: same
// signature, pre-fix behaviour (the raw path is the write target).
const identityResolver = "(func(p string) string { return p })"

// neutraliseResolver writes resolver-neutralised copies of pkgDir's production
// files that reference statemap.ResolveWriteTarget into scratch, and returns a
// `go -overlay` manifest mapping the originals to them plus the number of
// references rewritten. The import is kept used with a blank var so the only
// change is behavioural.
func neutraliseResolver(t *testing.T, pkgDir, scratch string) (overlay string, sites int) {
	t.Helper()
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("read %s: %v", pkgDir, err)
	}
	replace := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(pkgDir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		mutated, n := neutraliseFile(t, path, src)
		if n == 0 {
			continue
		}
		dst := filepath.Join(scratch, name)
		if err := os.WriteFile(dst, mutated, 0o644); err != nil {
			t.Fatalf("write %s: %v", dst, err)
		}
		replace[path] = dst
		sites += n
	}
	raw, err := json.Marshal(map[string]any{"Replace": replace})
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	overlay = filepath.Join(scratch, "overlay.json")
	if err := os.WriteFile(overlay, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", overlay, err)
	}
	return overlay, sites
}

// neutraliseFile rewrites every <alias>.ResolveWriteTarget selector in src,
// where alias is the file's name for the statemap import.
func neutraliseFile(t *testing.T, path string, src []byte) ([]byte, int) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	alias := ""
	for _, imp := range f.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err == nil && p == statemapImport {
			alias = "statemap"
			if imp.Name != nil {
				alias = imp.Name.Name
			}
		}
	}
	if alias == "" || alias == "_" || alias == "." {
		return src, 0
	}
	var spans [][2]int
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "ResolveWriteTarget" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); !ok || id.Name != alias {
			return true
		}
		spans = append(spans, [2]int{fset.Position(sel.Pos()).Offset, fset.Position(sel.End()).Offset})
		return false
	})
	if len(spans) == 0 {
		return src, 0
	}
	out := string(src)
	for i := len(spans) - 1; i >= 0; i-- { // back to front keeps earlier offsets valid
		out = out[:spans[i][0]] + identityResolver + out[spans[i][1]:]
	}
	out += "\nvar _ = " + alias + ".ResolveWriteTarget\n"
	return []byte(out), len(spans)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mkdir(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	return dir
}

func writeFile(t *testing.T, path, body string) string {
	t.Helper()
	mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func symlink(t *testing.T, target, link string) string {
	t.Helper()
	mkdir(t, filepath.Dir(link))
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink %s -> %s: %v", link, target, err)
	}
	return link
}

// assertLinkIntact fails when path is no longer a symlink to want — the
// cycle-999 sever is exactly a rename replacing the link with a regular file.
func assertLinkIntact(t *testing.T, path, want string) {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Errorf("lstat %s: %v", path, err)
		return
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("%s is no longer a symlink (mode %v) — the write replaced the link with a regular file (the cycle-999 sever)", path, fi.Mode())
		return
	}
	if got, err := os.Readlink(path); err != nil || got != want {
		t.Errorf("%s now points at %q (err=%v), want %q", path, got, err, want)
	}
}

func readStateFile(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}

func numField(m map[string]any, key string) float64 {
	v, _ := m[key].(float64)
	return v
}

// testFilesMatching returns the _test.go files directly in dir whose source
// matches re.
func testFilesMatching(t *testing.T, dir string, re *regexp.Regexp) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if re.Match(src) {
			out = append(out, p)
		}
	}
	return out
}

// runIn executes one command with an explicit working directory — never the
// process cwd, which differs between the main tree, the cycle worktree and each
// fleet lane. EVOLVE_CYCLE_STATE_FILE is stripped from the child's environment:
// the fleet orchestrator sets it to the live lane's cycle-state, and a storage
// WriteCycleState test that forgot to clear it would overwrite the RUNNING
// cycle's state.
func runIn(t *testing.T, dir, name string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), name, args...)
	var outBuf, errBuf strings.Builder
	cmd.Dir = dir
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, cycleStateEnv+"=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v in %s: %v", cmd.Args, dir, err)
	}
	return outBuf.String(), errBuf.String(), code
}
