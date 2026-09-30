//go:build acs

package cycle1690

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

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const sealCycleID = 42

const sealHangBound = 60 * time.Second

var fixedNow = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func TestC1690_001_ResolveWriteTargetFollowsChainsToFinalTarget(t *testing.T) {
	type row struct {
		name  string
		setup func(t *testing.T, root string) (input string, want []string)
	}
	rows := []row{
		{"regular file resolves to itself", func(t *testing.T, root string) (string, []string) {
			p := writeFile(t, filepath.Join(root, "canon", "state.json"), "{}")
			return p, []string{p}
		}},
		{"missing plain path resolves to itself", func(t *testing.T, root string) (string, []string) {
			p := filepath.Join(mkdir(t, filepath.Join(root, "wt")), "state.json")
			return p, []string{p}
		}},
		{"absolute link (the linkGuardDeps shape) resolves to its target", func(t *testing.T, root string) (string, []string) {
			target := writeFile(t, filepath.Join(root, "canon", "state.json"), "{}")
			link := symlink(t, target, filepath.Join(root, "wt", "state.json"))
			return link, []string{target}
		}},
		{"relative link resolves against the link's own directory", func(t *testing.T, root string) (string, []string) {
			target := writeFile(t, filepath.Join(root, "canon", "state.json"), "{}")
			link := symlink(t, filepath.Join("..", "canon", "state.json"), filepath.Join(root, "wt", "state.json"))
			return link, []string{target}
		}},
		{"two-hop chain resolves to the FINAL target, not the first hop", func(t *testing.T, root string) (string, []string) {
			target := writeFile(t, filepath.Join(root, "canon", "state.json"), "{}")
			mid := symlink(t, filepath.Join("..", "canon", "state.json"), filepath.Join(root, "mid", "state.json"))
			link := symlink(t, mid, filepath.Join(root, "wt", "state.json"))
			return link, []string{target}
		}},
		{"dangling link resolves to the missing final hop (never errors)", func(t *testing.T, root string) (string, []string) {
			target := filepath.Join(mkdir(t, filepath.Join(root, "canon")), "state.json")
			link := symlink(t, target, filepath.Join(root, "wt", "state.json"))
			return link, []string{target}
		}},
		{"symlink loop is bounded and returns a hop of the loop", func(t *testing.T, root string) (string, []string) {
			dir := mkdir(t, filepath.Join(root, "loop"))
			a, b := filepath.Join(dir, "a.json"), filepath.Join(dir, "b.json")
			symlink(t, b, a)
			symlink(t, a, b)
			return a, []string{a, b}
		}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			input, want := r.setup(t, t.TempDir())
			got := resolveWithin(t, input)
			if !samePathAny(t, got, want) {
				t.Errorf("statemap.ResolveWriteTarget(%s) = %s, want one of %v", input, got, want)
			}
		})
	}

	t.Run("agrees with WriteStateMap's actual write-through target", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "canon", "state.json"), "{}")
		mid := symlink(t, filepath.Join("..", "canon", "state.json"), filepath.Join(root, "mid", "state.json"))
		link := symlink(t, mid, filepath.Join(root, "wt", "state.json"))
		if err := statemap.WriteStateMap(link, map[string]any{"probe": "landed"}); err != nil {
			t.Fatalf("WriteStateMap(%s): %v", link, err)
		}
		resolved := resolveWithin(t, link)
		if fi, err := os.Lstat(resolved); err != nil || !fi.Mode().IsRegular() {
			t.Fatalf("ResolveWriteTarget(%s) = %s, which is not the regular file WriteStateMap wrote (err=%v)", link, resolved, err)
		}
		if got := readStateFile(t, resolved)["probe"]; got != "landed" {
			t.Errorf("WriteStateMap's bytes are not at ResolveWriteTarget(%s) (probe=%v) — the export and the internal write target disagree", link, got)
		}
	})
}

func TestC1690_002_ResolveWriteTargetIsANamedDocumentedCoveredExport(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	pkgDir := filepath.Join(goDir, "internal", "adapters", "statemap")
	if !acsassert.FileContains(t, filepath.Join(goDir, ".apicover-enforce"), "./internal/adapters/statemap") {
		t.Fatalf("internal/adapters/statemap is no longer enrolled in go/.apicover-enforce — this predicate's premise is gone; if enrollment was deliberately dropped, say so in build-report.md")
	}
	ctx := context.Background()

	syms, err := apicover.Enumerate(ctx, pkgDir)
	if err != nil {
		t.Fatalf("apicover.Enumerate(statemap): %v", err)
	}
	var found *apicover.Symbol
	for i := range syms {
		if syms[i].Name == "ResolveWriteTarget" {
			found = &syms[i]
		}
	}
	switch {
	case found == nil:
		t.Errorf("ResolveWriteTarget is not an exported symbol of internal/adapters/statemap")
	case found.Kind != apicover.KindFunc:
		t.Errorf("ResolveWriteTarget is exported as kind %v, want a package-level func", found.Kind)
	case !found.HasDoc:
		t.Errorf("ResolveWriteTarget carries no godoc — apicover's Phase-5 Definition of Done requires godoc on every export")
	}

	names, err := apicover.NamesReferencedInTests(ctx, pkgDir)
	if err != nil {
		t.Fatalf("apicover.NamesReferencedInTests(statemap): %v", err)
	}
	if !names["ResolveWriteTarget"] {
		t.Errorf("no _test.go in internal/adapters/statemap names ResolveWriteTarget — an enrolled package's unnamed export hard-fails `make apicover-enforce`")
	}

	cov := filepath.Join(t.TempDir(), "coverage.txt")
	if out, errOut, code := runIn(t, goDir, "go", "test", "-count=1", "-tags", "integration", "-coverprofile="+cov, "./internal/adapters/statemap"); code != 0 {
		t.Fatalf("go test -coverprofile ./internal/adapters/statemap exited %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	funcOut, errOut, code := runIn(t, goDir, "go", "tool", "cover", "-func="+cov)
	if code != 0 {
		t.Fatalf("go tool cover -func exited %d: %s", code, errOut)
	}
	funcFile := writeFile(t, filepath.Join(t.TempDir(), "coverage.func.txt"), funcOut)
	var report strings.Builder
	rc, err := apicover.Run(ctx, apicover.Config{Dirs: []string{pkgDir}, CoverPath: funcFile, Enforce: true}, &report)
	if err != nil || rc != 0 {
		t.Errorf("apicover enforce over internal/adapters/statemap: rc=%d err=%v\n%s", rc, err, report.String())
	}
}

func TestC1690_003_SealCycleLocksTheResolvedCanonicalSidecar(t *testing.T) {
	for _, dangling := range []bool{false, true} {
		t.Run(fmt.Sprintf("dangling=%v", dangling), func(t *testing.T) {
			fx := newSealFixture(t, !dangling, true)
			for _, p := range []string{fx.canonical + ".lock", fx.link + ".lock"} {
				if _, err := os.Lstat(p); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("fixture precondition: %s already exists (err=%v)", p, err)
				}
			}

			if err := sealWithin(t, fx.opts()); err != nil {
				t.Fatalf("SealCycle through a linked evolve dir: %v", err)
			}

			if _, err := os.Lstat(fx.canonical + ".lock"); err != nil {
				t.Errorf("SealCycle never took the canonical sidecar %s.lock (err=%v) — it locks the UNRESOLVED path, so a canonical-path writer (statemap.UpdateStateMap, storage.UpdateState) is not excluded", fx.canonical, err)
			}
			if _, err := os.Lstat(fx.link + ".lock"); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("SealCycle took a sidecar beside the LINK (%s.lock exists, err=%v) — the lock must be on statemap.ResolveWriteTarget(statePath)", fx.link, err)
			}
		})
	}
}

func TestC1690_004_SealCycleSerializesWithACanonicalWriter(t *testing.T) {
	fx := newSealFixture(t, true, true)

	entered, release := make(chan struct{}), make(chan struct{})
	writerErr := make(chan error, 1)
	go func() {
		writerErr <- statemap.UpdateStateMap(fx.canonical, func(m map[string]any) {
			close(entered)
			<-release
			m["canonicalWriter"] = "landed"
		})
	}()
	<-entered

	sealErr := make(chan error, 1)
	go func() { sealErr <- sealCycle(fx.opts()) }()

	select {
	case err := <-sealErr:
		close(release)
		<-writerErr
		t.Fatalf("SealCycle returned (err=%v) while a canonical-path writer held %s.lock mid-RMW — the seal does not contend on the canonical lock, so the paused writer's stale snapshot will clobber it", err, fx.canonical)
	case <-time.After(750 * time.Millisecond):
	}
	close(release)

	if err := <-writerErr; err != nil {
		t.Fatalf("canonical writer: %v", err)
	}
	select {
	case err := <-sealErr:
		if err != nil {
			t.Fatalf("SealCycle after the canonical writer released: %v", err)
		}
	case <-time.After(sealHangBound):
		t.Fatalf("SealCycle did not return within %s after the canonical lock was released — deadlock", sealHangBound)
	}

	final := readStateFile(t, fx.canonical)
	if final["canonicalWriter"] != "landed" {
		t.Errorf("the canonical writer's update was lost: canonicalWriter=%v", final["canonicalWriter"])
	}
	if got := numField(final, "lastCycleNumber"); got != sealCycleID {
		t.Errorf("the seal's update was lost: canonical lastCycleNumber=%v, want %d", got, sealCycleID)
	}
	if !hasFailedApproach(final, sealCycleID, string(failurelog.OperatorReset)) {
		t.Errorf("the seal's operator-reset failedApproaches entry for cycle %d is missing from the canonical file: %v", sealCycleID, final["failedApproaches"])
	}
	assertLinkIntact(t, fx.link, fx.linkTarget)
}

func TestC1690_007_SealCycleEndToEndKeepsTheLinkAndLandsOnCanonical(t *testing.T) {
	for _, absolute := range []bool{true, false} {
		t.Run(fmt.Sprintf("absolute=%v", absolute), func(t *testing.T) {
			fx := newSealFixture(t, true, absolute)
			if err := sealWithin(t, fx.opts()); err != nil {
				t.Fatalf("SealCycle: %v", err)
			}
			assertLinkIntact(t, fx.link, fx.linkTarget)

			final := readStateFile(t, fx.canonical)
			if got := numField(final, "lastCycleNumber"); got != sealCycleID {
				t.Errorf("canonical lastCycleNumber=%v, want %d — the seal's write stranded off the canonical file", got, sealCycleID)
			}
			if !hasFailedApproach(final, sealCycleID, string(failurelog.OperatorReset)) {
				t.Errorf("canonical failedApproaches lacks the operator-reset entry for cycle %d: %v", sealCycleID, final["failedApproaches"])
			}
			if cb, _ := final["currentBatch"].(map[string]any); numField(cb, "cycleAccruedCostUSD") != 0 {
				t.Errorf("canonical currentBatch.cycleAccruedCostUSD=%v, want 0", cb["cycleAccruedCostUSD"])
			}
			if final["operatorOwnedKey"] != "must-survive" {
				t.Errorf("an unmodelled operator key did not survive the seal: %v", final["operatorOwnedKey"])
			}
		})
	}
}

func TestC1690_005_FailurelogWritersWriteThroughSymlinkedState(t *testing.T) {
	for _, w := range stateWriters() {
		for _, shape := range linkShapes() {
			t.Run(w.name+"/"+shape.name, func(t *testing.T) {
				root := t.TempDir()
				canonical := writeFile(t, filepath.Join(root, "canon", "state.json"), seedState)
				entry, hops := shape.build(t, root, canonical)

				n, err := w.run(entry)
				if err != nil {
					t.Fatalf("%s(%s): %v", w.name, entry, err)
				}
				if n != w.wantCount {
					t.Errorf("%s(%s) reported %d, want %d", w.name, entry, n, w.wantCount)
				}
				for _, h := range hops {
					assertLinkIntact(t, h.path, h.target)
				}
				final := readStateFile(t, canonical)
				if msg := w.check(final); msg != "" {
					t.Errorf("canonical %s after %s through %s: %s", canonical, w.name, shape.name, msg)
				}
				if final["operatorOwnedKey"] != "must-survive" {
					t.Errorf("an unmodelled operator key did not survive %s: %v", w.name, final["operatorOwnedKey"])
				}
			})
		}
	}
}

func TestC1690_006_FailurelogDanglingLinkIsLeftUntouched(t *testing.T) {
	for _, w := range stateWriters() {
		t.Run(w.name, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(mkdir(t, filepath.Join(root, "canon")), "state.json")
			link := symlink(t, target, filepath.Join(root, "wt", "state.json"))

			n, err := w.run(link)
			if w.name == "Record" {
				if !errors.Is(err, failurelog.ErrStateMissing) {
					t.Errorf("Record through a dangling link: err=%v, want ErrStateMissing (Record never auto-creates state.json)", err)
				}
			} else if err != nil || n != 0 {
				t.Errorf("%s through a dangling link = (%d, %v), want a (0, nil) no-op", w.name, n, err)
			}
			assertLinkIntact(t, link, target)
			if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("%s materialized %s through a dangling link (err=%v) — failurelog writers must not create state.json", w.name, target, err)
			}
		})
	}
}

var durableTests = []struct{ pkg, name string }{
	{"internal/adapters/statemap", "TestResolveWriteTarget"},
	{"internal/core", "TestSealCycle_SymlinkedStateLocksCanonicalTarget"},
	{"internal/failurelog", "TestStateWriters_PreserveSymlinkedStatePath"},
}

func TestC1690_008_DurableRegressionTestsRanPassedAndAreTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	for _, dt := range durableTests {
		t.Run(dt.name, func(t *testing.T) {
			out, errOut, code := runIn(t, goDir, "go", "test", "-count=1", "-v", "-run", "^"+dt.name+"$", "./"+dt.pkg)
			if code != 0 {
				t.Fatalf("go test -run ^%s$ ./%s exited %d\nstdout:\n%s\nstderr:\n%s", dt.name, dt.pkg, code, out, errOut)
			}
			if !strings.Contains(out, "--- PASS: "+dt.name) {
				t.Errorf("%s never RAN in ./%s (exit 0 without a '--- PASS: %s' line) — the eval grader naming it would pass vacuously", dt.name, dt.pkg, dt.name)
			}
			decl := regexp.MustCompile(`(?m)^func ` + regexp.QuoteMeta(dt.name) + `\(t \*testing\.T\)`)
			carriers := testFilesMatching(t, filepath.Join(goDir, dt.pkg), decl)
			if len(carriers) == 0 {
				t.Fatalf("no _test.go in ./%s declares func %s(t *testing.T)", dt.pkg, dt.name)
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
		})
	}
}

func TestC1690_009_TouchedPackagesGreenVetAndGofmtClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	if out, errOut, code := runIn(t, goDir, "go", "test", "-count=1", "./internal/adapters/statemap"); code != 0 {
		t.Errorf("go test ./internal/adapters/statemap exited %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	if out, errOut, code := runIn(t, goDir, "go", "test", "-count=1", "./internal/failurelog"); code != 0 {
		t.Errorf("go test ./internal/failurelog exited %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	coreSeal := exec.CommandContext(context.Background(), "go", "test", "-count=1", "-run", "^Test(SealCycle|AutosealStaleMarker|MarkerShouldAutoseal)", "./internal/core")
	if out, errOut, code := runCmd(t, goDir, coreSeal); code != 0 {
		t.Errorf("go test -run <seal/reset family> ./internal/core exited %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	vet := exec.CommandContext(context.Background(), "go", "vet", "./internal/adapters/statemap", "./internal/failurelog", "./internal/core")
	if out, errOut, code := runCmd(t, goDir, vet); code != 0 {
		t.Errorf("go vet exited %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	listed, errOut, code := runIn(t, goDir, "gofmt", "-l", "internal/adapters/statemap", "internal/failurelog", "internal/core")
	if code != 0 {
		t.Fatalf("gofmt -l exited %d: %s", code, errOut)
	}
	if strings.TrimSpace(listed) != "" {
		t.Errorf("gofmt -l is not empty:\n%s", listed)
	}
}

const laneItemID = "statejson-latent-unresolved-writers"

const (
	laneScopeJSON      = `{"todo_ids":["statejson-latent-unresolved-writers"]}`
	triageDecisionJSON = `{"cycle":1690,
  "top_n":[{"id":"statemap-export-resolve-write-target"},{"id":"failurelog-symlink-resolve-writes"}],
  "deferred":[],"dropped":[],"skip_shipped":[]}`
)

func TestC1690_010_DeferredStepThreeHasAFollowUpThatSurvivesThePASSLanding(t *testing.T) {
	root := acsassert.RepoRoot(t)
	consumed := passLandingConsumedIDs(t)

	t.Run("PASS landing consumes the lane item and any committed-id namesake", func(t *testing.T) {
		inbox := mkdir(t, filepath.Join(t.TempDir(), "inbox"))
		writeFile(t, filepath.Join(inbox, "2026-07-21T09-30-00Z-"+laneItemID+".json"), `{"id":"`+laneItemID+`"}`)
		writeFile(t, filepath.Join(inbox, "2026-09-26T00-00-00Z-namesake.json"), `{"id":"failurelog-symlink-resolve-writes","title":"atomicwrite sweep"}`)
		consumeFromInbox(t, inbox, consumed)
		if left := inboxIDs(t, inbox); len(left) != 0 {
			t.Fatalf("after the PASS consume the inbox still holds %v — the simulation does not consume the lane scope, so a survival check would be vacuous", left)
		}
	})

	followUps := stepThreeFollowUps(t, root)
	if len(followUps) == 0 {
		t.Fatalf("no tracked .evolve/inbox record carries how_to_apply step (3) forward: want a record with an id other than %q that names %q (lineage) and the atomicwrite linked-state sweep (subject). Consumed ids on this cycle's PASS landing: %v — without the record, step (3) vanishes when the lane item is consumed (audit M1)", laneItemID, laneItemID, consumed)
	}
	rootItems, rootWarn, err := inboxbatch.LoadDir(filepath.Join(root, ".evolve", "inbox"))
	if err != nil {
		t.Fatalf("load the tracked inbox root: %v", err)
	}
	consumedItems, _, err := inboxbatch.LoadDir(filepath.Join(root, ".evolve", "inbox", "consumed"))
	if err != nil {
		t.Fatalf("load .evolve/inbox/consumed: %v", err)
	}
	for _, f := range followUps {
		t.Run(f.item.ID, func(t *testing.T) {
			if strings.TrimSpace(f.item.Title) == "" {
				t.Errorf("%s: empty title — the Task Contract renders the title as its heading", f.rel)
			}
			if len(f.item.Acceptance) == 0 {
				t.Errorf("%s: no acceptance[] — the lane item's own missing acceptance is why tdd/build/audit saw \"acceptance unknown\" (ADR-0098 projects acceptance verbatim)", f.rel)
			}
			for _, w := range rootWarn {
				if strings.Contains(w, "duplicate id "+f.item.ID+" ") {
					t.Errorf("%s: id collides with another inbox root record: %s", f.rel, w)
				}
			}
			for _, it := range append(append([]inboxbatch.Item{}, rootItems...), consumedItems...) {
				if it.ID == f.item.ID && it.Path != filepath.Base(f.rel) {
					t.Errorf("%s: id %q is already used by %s — a namesake record is what lanes mis-resolve to (inst-L1548a)", f.rel, f.item.ID, it.Path)
				}
			}
			inbox := mkdir(t, filepath.Join(t.TempDir(), "inbox"))
			writeFile(t, filepath.Join(inbox, "2026-07-21T09-30-00Z-"+laneItemID+".json"), `{"id":"`+laneItemID+`"}`)
			writeFile(t, filepath.Join(inbox, filepath.Base(f.rel)), string(f.raw))
			consumeFromInbox(t, inbox, consumed)
			left := inboxIDs(t, inbox)
			if len(left) != 1 || left[0] != f.item.ID {
				t.Errorf("after this cycle's PASS consume (ids %v) the inbox holds %v, want exactly [%s] — the follow-up is consumed with the lane item", consumed, left, f.item.ID)
			}
		})
	}
}

func TestC1690_011_ExplanationLimitationsStateTheConsumeAndCiteTheFollowUp(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docs, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-1690-*.md"))
	if err != nil || len(docs) != 1 {
		t.Fatalf("want exactly one cycle-1690 explanation document under docs/explain/builds, got %v (err %v)", docs, err)
	}
	rel, err := filepath.Rel(root, docs[0])
	if err != nil {
		t.Fatalf("relativise %s: %v", docs[0], err)
	}
	if _, _, code := runIn(t, root, "git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("%s is UNTRACKED — it would be dropped at ship", rel)
	}
	body, err := os.ReadFile(docs[0])
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	lim, ok, err := reportdoc.Section(string(body), "Limitations")
	if err != nil || !ok {
		t.Fatalf("%s: no single ## Limitations section (ok=%v err=%v)", rel, ok, err)
	}
	if regexp.MustCompile(`(?i)stays\s+open\s+on\s+the\s+inbox\s+record`).MatchString(lim) {
		t.Errorf("%s Limitations still claims step (3) \"stays open on the inbox record\" — a PASS landing consumes %s (ship/postship.go committedInboxIDs)", rel, laneItemID)
	}
	if !strings.Contains(lim, laneItemID) || !regexp.MustCompile(`(?i)\bconsum`).MatchString(lim) {
		t.Errorf("%s Limitations does not state that a PASS landing consumes the lane item %s:\n%s", rel, laneItemID, lim)
	}
	followUps := stepThreeFollowUps(t, root)
	var ids []string
	cited := false
	for _, f := range followUps {
		ids = append(ids, f.item.ID)
		if !strings.Contains(lim, f.item.ID) {
			continue
		}
		got, err := inboxmover.FindFileByTaskID(filepath.Join(root, filepath.Dir(f.rel)), f.item.ID)
		if err != nil || filepath.Base(got) != filepath.Base(f.rel) {
			t.Errorf("Limitations cites %q but the ship resolver finds %q (err %v), want %s", f.item.ID, got, err, f.rel)
			continue
		}
		cited = true
	}
	if !cited {
		t.Errorf("%s Limitations cites none of the tracked step-(3) follow-up records %v — the deferred sweep must be cited by its inbox id:\n%s", rel, ids, lim)
	}
}

func passLandingConsumedIDs(t *testing.T) []string {
	t.Helper()
	ws := t.TempDir()
	writeFile(t, filepath.Join(ws, "lane-scope.json"), laneScopeJSON)
	body := []byte(triageDecisionJSON)
	deferred := map[string]bool{}
	for _, id := range inboxmover.DeferredIDs(body) {
		deferred[id] = true
	}
	ids := inboxmover.CommittedIDs(body)
	scope := cycleoutcome.LaneScopeIDs(ws)
	if len(ids) == 0 || len(scope) == 0 {
		t.Fatalf("fixture decision yields committed=%v scope=%v — the consume readers no longer parse this cycle's shape", ids, scope)
	}
	for _, id := range scope {
		if !deferred[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

func consumeFromInbox(t *testing.T, inbox string, ids []string) {
	t.Helper()
	for _, id := range ids {
		src, err := inboxmover.FindFileByTaskID(inbox, id)
		if errors.Is(err, inboxmover.ErrNotFound) {
			continue
		}
		if err != nil {
			t.Fatalf("resolve %q in %s: %v", id, inbox, err)
		}
		if err := os.Remove(src); err != nil {
			t.Fatalf("consume %s: %v", src, err)
		}
	}
}

func inboxIDs(t *testing.T, inbox string) []string {
	t.Helper()
	items, _, err := inboxbatch.LoadDir(inbox)
	if err != nil {
		t.Fatalf("load %s: %v", inbox, err)
	}
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return ids
}

type stepThreeFollowUp struct {
	rel  string
	raw  []byte
	item inboxbatch.Item
}

func stepThreeFollowUps(t *testing.T, root string) []stepThreeFollowUp {
	t.Helper()
	out, errOut, code := runIn(t, root, "git", "-C", root, "ls-files", "--", ".evolve/inbox")
	if code != 0 {
		t.Fatalf("git ls-files .evolve/inbox exited %d: %s", code, errOut)
	}
	var found []stepThreeFollowUp
	for _, rel := range strings.Split(strings.TrimSpace(out), "\n") {
		dir := filepath.ToSlash(filepath.Dir(rel))
		if !strings.HasSuffix(rel, ".json") || (dir != ".evolve/inbox" && dir != ".evolve/inbox/consumed") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		text := strings.ToLower(string(raw))
		if !strings.Contains(text, laneItemID) || !strings.Contains(text, "atomicwrite") {
			continue
		}
		item, _, err := inboxbatch.LoadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("%s names the step-(3) sweep but does not load as an inbox record: %v", rel, err)
			continue
		}
		if item.ID == laneItemID || consumedByThisCycle(raw) {
			continue
		}
		found = append(found, stepThreeFollowUp{rel: rel, raw: raw, item: item})
	}
	return found
}

func consumedByThisCycle(raw []byte) bool {
	var doc struct {
		Consumed struct {
			Cycle any `json:"cycle"`
		} `json:"consumed"`
	}
	return json.Unmarshal(raw, &doc) == nil && fmt.Sprint(doc.Consumed.Cycle) == "1690"
}

const seedState = `{
  "lastCycleNumber": 41,
  "operatorOwnedKey": "must-survive",
  "currentBatch": {"cycleAccruedCostUSD": 239.2},
  "failedApproaches": [
    {"cycle": 7, "classification": "infrastructure-transient", "recordedAt": "2026-01-01T00:00:00Z", "expiresAt": "2026-01-02T00:00:00Z"}
  ],
  "carryoverTodos": [
    {"id": "expired-todo", "expiresAt": "2026-01-02T00:00:00Z"},
    {"id": "legacy-todo"}
  ]
}`

type stateWriter struct {
	name      string
	run       func(statePath string) (int, error)
	wantCount int
	check     func(final map[string]any) string
}

func stateWriters() []stateWriter {
	return []stateWriter{
		{"Record", func(p string) (int, error) {
			rec, err := failurelog.Record(p, "", failurelog.RecordRequest{
				Cycle: sealCycleID, Classification: string(failurelog.OperatorReset), Summary: "cycle-1690 predicate", Now: fixedNow,
			})
			return rec.Cycle, err
		}, sealCycleID, func(m map[string]any) string {
			if !hasFailedApproach(m, sealCycleID, string(failurelog.OperatorReset)) {
				return fmt.Sprintf("no operator-reset entry for cycle %d: %v", sealCycleID, m["failedApproaches"])
			}
			if got := numField(m, "lastCycleNumber"); got != sealCycleID {
				return fmt.Sprintf("lastCycleNumber=%v, want %d", got, sealCycleID)
			}
			return ""
		}},
		{"PruneExpired", func(p string) (int, error) {
			r, err := failurelog.PruneExpired(p, fixedNow)
			return r.Removed, err
		}, 1, func(m map[string]any) string {
			return wantLen(m, "failedApproaches", 0)
		}},
		{"PruneByClassification", func(p string) (int, error) {
			r, err := failurelog.PruneByClassification(p, []failurelog.Classification{failurelog.InfrastructureTransient})
			return r.Removed, err
		}, 1, func(m map[string]any) string {
			return wantLen(m, "failedApproaches", 0)
		}},
		{"PruneExpiredCarryoverTodos", func(p string) (int, error) {
			r, err := failurelog.PruneExpiredCarryoverTodos(p, fixedNow)
			return r.Removed, err
		}, 1, func(m map[string]any) string {
			if msg := wantLen(m, "carryoverTodos", 1); msg != "" {
				return msg
			}
			if id := todo(m, 0)["id"]; id != "legacy-todo" {
				return fmt.Sprintf("surviving todo id=%v, want legacy-todo", id)
			}
			return ""
		}},
		{"BackfillLegacyCarryoverExpiry", func(p string) (int, error) {
			return failurelog.BackfillLegacyCarryoverExpiry(p, failurelog.DefaultCarryoverBackfillTTL, fixedNow)
		}, 1, func(m map[string]any) string {
			if s, _ := todo(m, 1)["expiresAt"].(string); s == "" {
				return fmt.Sprintf("legacy-todo was not stamped with an expiresAt: %v", m["carryoverTodos"])
			}
			return ""
		}},
		{"IncrementCarryoverUnpicked", func(p string) (int, error) {
			return failurelog.IncrementCarryoverUnpicked(p)
		}, 2, func(m map[string]any) string {
			for i := 0; i < 2; i++ {
				if got := numField(todo(m, i), "cycles_unpicked"); got != 1 {
					return fmt.Sprintf("todo %d cycles_unpicked=%v, want 1", i, got)
				}
			}
			return ""
		}},
	}
}

type hop struct{ path, target string }

type linkShape struct {
	name  string
	build func(t *testing.T, root, canonical string) (entry string, hops []hop)
}

func linkShapes() []linkShape {
	return []linkShape{
		{"regular-file", func(t *testing.T, root, canonical string) (string, []hop) {
			return canonical, nil
		}},
		{"absolute-link", func(t *testing.T, root, canonical string) (string, []hop) {
			link := symlink(t, canonical, filepath.Join(root, "wt", "state.json"))
			return link, []hop{{link, canonical}}
		}},
		{"relative-link", func(t *testing.T, root, canonical string) (string, []hop) {
			rel := filepath.Join("..", "canon", "state.json")
			link := symlink(t, rel, filepath.Join(root, "wt", "state.json"))
			return link, []hop{{link, rel}}
		}},
		{"two-hop-chain", func(t *testing.T, root, canonical string) (string, []hop) {
			rel := filepath.Join("..", "canon", "state.json")
			mid := symlink(t, rel, filepath.Join(root, "mid", "state.json"))
			link := symlink(t, mid, filepath.Join(root, "wt", "state.json"))
			return link, []hop{{link, mid}, {mid, rel}}
		}},
	}
}

type sealFixture struct {
	evolveDir  string
	link       string
	linkTarget string
	canonical  string
}

func newSealFixture(t *testing.T, seedCanonical, absolute bool) sealFixture {
	t.Helper()
	t.Setenv("EVOLVE_CYCLE_STATE_FILE", "")

	root := t.TempDir()
	canonical := filepath.Join(mkdir(t, filepath.Join(root, "canon", ".evolve")), "state.json")
	if seedCanonical {
		writeFile(t, canonical, seedState)
	}
	evolveDir := mkdir(t, filepath.Join(root, "wt", ".evolve"))
	workspace := mkdir(t, filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", sealCycleID)))
	cs, err := json.Marshal(map[string]any{
		"cycle_id": sealCycleID, "phase": "build", "active_agent": "builder", "workspace_path": workspace,
	})
	if err != nil {
		t.Fatalf("marshal cycle-state: %v", err)
	}
	writeFile(t, filepath.Join(evolveDir, "cycle-state.json"), string(cs))

	target := canonical
	if !absolute {
		target = filepath.Join("..", "..", "canon", ".evolve", "state.json")
	}
	link := symlink(t, target, filepath.Join(evolveDir, "state.json"))
	return sealFixture{evolveDir: evolveDir, link: link, linkTarget: target, canonical: canonical}
}

func (fx sealFixture) opts() core.SealOptions {
	return core.SealOptions{
		EvolveDir:   fx.evolveDir,
		ProjectRoot: fx.evolveDir,
		Reason:      "cycle-1690 predicate",
		Now:         func() time.Time { return fixedNow },
		GitHead:     func(string) (string, error) { return "cycle1690head", nil },
	}
}

type noopLedger struct{}

func (noopLedger) Append(context.Context, core.LedgerEntry) error { return nil }

func sealCycle(opts core.SealOptions) error {
	_, err := core.SealCycle(context.Background(), noopLedger{}, opts)
	return err
}

func sealWithin(t *testing.T, opts core.SealOptions) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- sealCycle(opts) }()
	select {
	case err := <-done:
		return err
	case <-time.After(sealHangBound):
		t.Fatalf("SealCycle did not return within %s — a writer inside its critical section re-entered the state lock it already holds", sealHangBound)
		return nil
	}
}

func resolveWithin(t *testing.T, path string) string {
	t.Helper()
	done := make(chan string, 1)
	go func() { done <- statemap.ResolveWriteTarget(path) }()
	select {
	case got := <-done:
		return got
	case <-time.After(10 * time.Second):
		t.Fatalf("statemap.ResolveWriteTarget(%s) did not return — the chain walk is not bounded", path)
		return ""
	}
}

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

func samePathAny(t *testing.T, got string, want []string) bool {
	t.Helper()
	g := canonDir(t, got)
	for _, w := range want {
		if g == canonDir(t, w) {
			return true
		}
	}
	return false
}

func canonDir(t *testing.T, p string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return filepath.Clean(p)
	}
	return filepath.Join(dir, filepath.Base(p))
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

func wantLen(m map[string]any, key string, n int) string {
	arr, _ := m[key].([]any)
	if len(arr) != n {
		return fmt.Sprintf("%s has %d entries, want %d: %v", key, len(arr), n, m[key])
	}
	return ""
}

func todo(m map[string]any, i int) map[string]any {
	arr, _ := m["carryoverTodos"].([]any)
	if i >= len(arr) {
		return map[string]any{}
	}
	e, _ := arr[i].(map[string]any)
	return e
}

func hasFailedApproach(m map[string]any, cycle int, class string) bool {
	arr, _ := m["failedApproaches"].([]any)
	for _, e := range arr {
		fa, _ := e.(map[string]any)
		if numField(fa, "cycle") == float64(cycle) && fa["classification"] == class {
			return true
		}
	}
	return false
}

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

func runIn(t *testing.T, dir, name string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	return runCmd(t, dir, exec.CommandContext(context.Background(), name, args...))
}

func runCmd(t *testing.T, dir string, cmd *exec.Cmd) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf strings.Builder
	cmd.Dir = dir
	cmd.Env = nil
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "EVOLVE_CYCLE_STATE_FILE=") {
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
