//go:build acs

package cycle2640

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	baseFile      = "base.txt"
	gcTimeout     = 8 * time.Minute
	cmdCoverRun   = `^(TestRunGC|TestGC|TestDevWorktree|TestGh|TestParseOpenPR)`
	fleetTreeSize = 98
)

var evolveBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "evolve-acs-2640-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	evolveBinary = filepath.Join(dir, "evolve")
	moduleDir, err := acsassert.ModuleDir(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	build := exec.Command("go", "build", "-o", evolveBinary, "./cmd/evolve")
	build.Dir = moduleDir
	build.Env = ipcenv.GoTestEnv()
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build ./cmd/evolve: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(2)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type pullRequest struct {
	Branch    string `json:"headRefName"`
	Oid       string `json:"headRefOid"`
	State     string `json:"state"`
	UpdatedAt string `json:"updatedAt"`
}

type hub struct {
	t       *testing.T
	store   *gittest.Repo
	root    string
	runtime string
	bin     string
	base    string
	tip     string
	anchor  time.Time
	prs     []pullRequest
}

func newHub(t *testing.T, policyJSON string) *hub {
	t.Helper()
	src := gittest.Fixture(t)
	src.Write(baseFile, "base\n")
	src.Git("add", "-A")
	src.Git("commit", "-q", "-m", "base")
	store := gittest.Bare(t)
	src.Git("push", "-q", store.Dir, "main")
	store.Git("update-ref", "refs/remotes/origin/main", "main")
	store.Git("remote", "add", "origin", src.Dir)
	root := filepath.Dir(store.Dir)
	runtime := filepath.Join(root, "runtime")
	store.Git("worktree", "add", "-q", "--detach", runtime, "main")
	evolveDir := filepath.Join(runtime, ".evolve")
	runDir := filepath.Join(evolveDir, "runs", "cycle-9999")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "cycle-9999", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	base := store.Git("rev-parse", "main")
	seconds, err := strconv.ParseInt(store.Git("log", "-1", "--format=%ct", base), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	h := &hub{t: t, store: store, root: root, runtime: runtime, bin: t.TempDir(), base: base, tip: base, anchor: time.Unix(seconds, 0)}
	h.flushGh()
	return h
}

const ghScript = `#!/bin/sh
dir=$(dirname "$0")
case "$*" in
*"--state open"*) cat "$dir/open.json";;
*"--state closed"*) cat "$dir/closed.json";;
*"--state merged"*) cat "$dir/merged.json";;
*"--state all"*) cat "$dir/all.json";;
*) echo '[]';;
esac
`

func (h *hub) flushGh() {
	h.t.Helper()
	write := func(name string, keep func(pullRequest) bool) {
		rows := []pullRequest{}
		for _, pr := range h.prs {
			if keep(pr) {
				rows = append(rows, pr)
			}
		}
		raw, err := json.Marshal(rows)
		if err != nil {
			h.t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(h.bin, name), raw, 0o644); err != nil {
			h.t.Fatal(err)
		}
	}
	state := func(want string) func(pullRequest) bool {
		return func(pr pullRequest) bool { return pr.State == want }
	}
	write("all.json", func(pullRequest) bool { return true })
	write("open.json", state("OPEN"))
	write("closed.json", state("CLOSED"))
	write("merged.json", state("MERGED"))
	if err := os.WriteFile(filepath.Join(h.bin, "gh"), []byte(ghScript), 0o755); err != nil {
		h.t.Fatal(err)
	}
}

func (h *hub) devDir(name string) string { return filepath.Join(h.root, "dev", name) }

func branchOf(name string) string { return "br-" + name }

func (h *hub) add(name, from string) string {
	h.t.Helper()
	dir := h.devDir(name)
	h.store.Git("worktree", "add", "-q", "-b", branchOf(name), dir, from)
	return dir
}

func (h *hub) commit(dir, rel string) string {
	h.t.Helper()
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(rel+"\n"), 0o644); err != nil {
		h.t.Fatal(err)
	}
	h.store.Git("-C", dir, "add", "-A")
	h.store.Git("-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "work "+rel)
	return h.store.Git("-C", dir, "rev-parse", "HEAD")
}

func (h *hub) fresh(name string) string {
	return h.add(name, h.base)
}

func (h *hub) unlanded(name string) (string, string) {
	dir := h.add(name, h.base)
	return dir, h.commit(dir, name+".txt")
}

func (h *hub) landed(name string) string {
	h.t.Helper()
	dir := h.add(name, h.tip)
	h.tip = h.commit(dir, name+".txt")
	h.store.Git("update-ref", "refs/remotes/origin/main", h.tip)
	return dir
}

func (h *hub) squashMerged(name string) string {
	dir, tip := h.unlanded(name)
	h.prs = append(h.prs, pullRequest{Branch: branchOf(name), Oid: tip, State: "MERGED", UpdatedAt: h.stamp(-30 * 24 * time.Hour)})
	return dir
}

func (h *hub) closed(name string) string {
	dir, tip := h.unlanded(name)
	h.prs = append(h.prs, pullRequest{Branch: branchOf(name), Oid: tip, State: "CLOSED", UpdatedAt: h.stamp(-30 * 24 * time.Hour)})
	return dir
}

func (h *hub) openPR(dir, name string) {
	tip := h.store.Git("-C", dir, "rev-parse", "HEAD")
	h.prs = append(h.prs, pullRequest{Branch: branchOf(name), Oid: tip, State: "OPEN", UpdatedAt: h.stamp(-24 * time.Hour)})
}

func (h *hub) stamp(offset time.Duration) string {
	return h.anchor.Add(offset).UTC().Format(time.RFC3339)
}

func (h *hub) dirty(dir, tag string) []string {
	h.t.Helper()
	changed := []string{baseFile, "scratch-" + tag + ".txt", "staged-" + tag + ".txt"}
	for _, rel := range changed {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("changed "+tag+"\n"), 0o644); err != nil {
			h.t.Fatal(err)
		}
	}
	h.store.Git("-C", dir, "add", "staged-"+tag+".txt")
	return changed
}

func (h *hub) gc(dry bool) (string, string, int) {
	h.t.Helper()
	h.flushGh()
	args := []string{"gc"}
	if dry {
		args = append(args, "--dry-run")
	}
	args = append(args, "--project-root", h.runtime)
	home, scratch := h.t.TempDir(), h.t.TempDir()
	if err := os.MkdirAll(filepath.Join(scratch, "gocache"), 0o755); err != nil {
		h.t.Fatal(err)
	}
	env := append(ipcenv.GoTestEnv(
		"HOME="+home,
		"TMPDIR="+scratch,
		"TMUX_TMPDIR="+scratch,
		"GOCACHE="+filepath.Join(scratch, "gocache"),
		"PATH="+h.bin+string(os.PathListSeparator)+os.Getenv("PATH"),
	), gittest.ConfigEnv()...)
	ctx, cancel := context.WithTimeout(context.Background(), gcTimeout)
	defer cancel()
	stdout, stderr, code, err := sysexec.Capture(ctx, sysexec.WithEnv(sysexec.DefaultRunner, env), h.runtime, evolveBinary, args...)
	if err != nil {
		h.t.Fatalf("evolve %s: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), err, stdout, stderr)
	}
	return stdout, stderr, code
}

func (h *hub) exists(name string) bool {
	_, err := os.Stat(h.devDir(name))
	return err == nil
}

func (h *hub) branchExists(name string) bool {
	return strings.TrimSpace(h.store.Git("branch", "--list", branchOf(name))) != ""
}

type treeRow struct {
	verb   string
	reason string
}

var treeLine = regexp.MustCompile(`^  (KEEP|WOULD-ARCHIVE|ARCHIVE|NOT-JUDGED|SKIPPED) path=(\S+)(?: branch=\S+)? idle=[0-9.]+d \((.*)\)$`)

func treeRows(out string) map[string]treeRow {
	rows := map[string]treeRow{}
	for _, line := range strings.Split(out, "\n") {
		if m := treeLine.FindStringSubmatch(line); m != nil {
			rows[filepath.Base(m[2])] = treeRow{verb: m[1], reason: m[3]}
		}
	}
	return rows
}

func requireRow(t *testing.T, rows map[string]treeRow, name, out string) treeRow {
	t.Helper()
	row, ok := rows[name]
	if !ok {
		t.Fatalf("RED: the gc output has no line for the tree %s:\n%s", name, out)
	}
	return row
}

func requireOK(t *testing.T, code int, stderr string) {
	t.Helper()
	if code != 0 {
		t.Fatalf("evolve gc exited %d, want 0\nstderr:\n%s", code, stderr)
	}
}

func TestC2640_001_CleanMergedAndClosedTreesAreRemovedThroughGcAndEachBranchStays(t *testing.T) {
	h := newHub(t, `{}`)
	h.landed("merged-by-ancestry")
	h.squashMerged("merged-by-pull-request")
	h.closed("closed-unmerged")
	h.unlanded("unlanded-no-pull-request")
	h.fresh("fresh-without-work")

	stdout, stderr, code := h.gc(false)
	requireOK(t, code, stderr)

	for _, name := range []string{"merged-by-ancestry", "merged-by-pull-request", "closed-unmerged"} {
		if h.exists(name) {
			t.Errorf("RED: the clean tree %s was not removed\n%s", name, stdout)
		}
		if !h.branchExists(name) {
			t.Errorf("RED: the branch of %s did not stay", name)
		}
	}
	for _, name := range []string{"unlanded-no-pull-request", "fresh-without-work"} {
		if !h.exists(name) {
			t.Errorf("RED: the tree %s has no landed or closed branch and must stay", name)
		}
	}
	if list := h.store.Git("worktree", "list", "--porcelain"); strings.Contains(list, "closed-unmerged") {
		t.Errorf("RED: git still lists the removed closed tree:\n%s", list)
	}
}

func TestC2640_002_ADirtyTreeIsKeptAndTheReportListsItsChangedPaths(t *testing.T) {
	h := newHub(t, `{}`)
	dirtyDir := h.landed("dirty-landed")
	changed := h.dirty(dirtyDir, "a")
	h.landed("clean-landed")

	dryOut, dryErr, dryCode := h.gc(true)
	requireOK(t, dryCode, dryErr)
	row := requireRow(t, treeRows(dryOut), "dirty-landed", dryOut)
	if row.verb != "KEEP" {
		t.Errorf("RED: the dirty tree is %s in the dry run, want KEEP", row.verb)
	}
	for _, rel := range changed {
		if !strings.Contains(row.reason, rel) {
			t.Errorf("RED: the keep reason of the dirty tree does not list %s: %q", rel, row.reason)
		}
	}

	out, stderr, code := h.gc(false)
	requireOK(t, code, stderr)
	if !h.exists("dirty-landed") {
		t.Fatalf("RED: a real run removed the dirty tree\n%s", out)
	}
	if h.exists("clean-landed") {
		t.Errorf("RED: the clean landed tree stayed, so the run removes nothing at all\n%s", out)
	}
	for _, rel := range changed[1:] {
		if _, err := os.Stat(filepath.Join(dirtyDir, rel)); err != nil {
			t.Errorf("RED: the uncommitted file %s is gone: %v", rel, err)
		}
	}
	if !strings.Contains(treeRows(out)["dirty-landed"].reason, "scratch-a.txt") {
		t.Errorf("RED: the real run does not list the changed paths of the dirty tree:\n%s", out)
	}
}

func TestC2640_003_ATreeThatMatchesAProtectedGlobStaysAndTheReasonNamesTheGlob(t *testing.T) {
	h := newHub(t, `{"gc":{"dev_worktrees":{"protected":["code-index","lane-*"]}}}`)
	h.landed("code-index")
	h.landed("lane-7")
	h.landed("plain-landed")
	h.landed("my-code-index-copy")

	dryOut, dryErr, dryCode := h.gc(true)
	requireOK(t, dryCode, dryErr)
	if strings.Contains(dryErr, "policy load failed") {
		t.Fatalf("RED: policy.json with gc.dev_worktrees.protected does not load:\n%s", dryErr)
	}
	rows := treeRows(dryOut)
	for name, glob := range map[string]string{"code-index": "code-index", "lane-7": "lane-*"} {
		row := requireRow(t, rows, name, dryOut)
		if row.verb != "KEEP" || !strings.Contains(row.reason, "protected by policy: "+glob) {
			t.Errorf("RED: %s is %s (%q), want KEEP with the reason %q", name, row.verb, row.reason, "protected by policy: "+glob)
		}
	}
	for _, name := range []string{"plain-landed", "my-code-index-copy"} {
		if row := requireRow(t, rows, name, dryOut); row.verb != "WOULD-ARCHIVE" {
			t.Errorf("RED: %s is %s, want WOULD-ARCHIVE: a glob matches the whole name, not a part of it", name, row.verb)
		}
	}

	out, stderr, code := h.gc(false)
	requireOK(t, code, stderr)
	for _, name := range []string{"code-index", "lane-7"} {
		if !h.exists(name) {
			t.Errorf("RED: a real run removed the protected tree %s\n%s", name, out)
		}
	}
	for _, name := range []string{"plain-landed", "my-code-index-copy"} {
		if h.exists(name) {
			t.Errorf("RED: a real run kept %s, which no glob protects\n%s", name, out)
		}
	}
}

func TestC2640_004_NoCodeListProtectsCodeIndexSoTheNameIsFreeWithoutPolicy(t *testing.T) {
	h := newHub(t, `{}`)
	h.landed("code-index")

	out, stderr, code := h.gc(false)
	requireOK(t, code, stderr)
	if h.exists("code-index") {
		t.Errorf("RED: the tree code-index stayed with an empty policy, so a code list protects it\n%s", out)
	}
	root := acsassert.GoModuleDir(t)
	for _, rel := range []string{"internal/gc", "internal/gcpolicy"} {
		files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(rel), "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no sources under %s: %v", rel, err)
		}
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			acsassert.FileNotContains(t, path, "code-index")
		}
	}
}

func TestC2640_005_ATreeWithAProcessInsideStaysAndTheReasonNamesTheProcess(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Fatalf("lsof is not on PATH, so the process check cannot run: %v", err)
	}
	h := newHub(t, `{}`)
	busy := h.landed("busy-landed")
	h.landed("idle-landed")
	sleeper := exec.Command("sleep", "600")
	sleeper.Dir = busy
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	pid := sleeper.Process.Pid
	t.Cleanup(func() {
		_ = sleeper.Process.Kill()
		_ = sleeper.Wait()
	})

	out, stderr, code := h.gc(false)
	requireOK(t, code, stderr)
	row := requireRow(t, treeRows(out), "busy-landed", out)
	want := "process " + strconv.Itoa(pid) + " runs inside"
	if row.verb != "KEEP" || !strings.Contains(row.reason, want) {
		t.Errorf("RED: busy-landed is %s (%q), want KEEP with %q", row.verb, row.reason, want)
	}
	if !h.exists("busy-landed") {
		t.Errorf("RED: a tree with a live process inside was removed")
	}
	if h.exists("idle-landed") {
		t.Errorf("RED: the landed tree with no process stayed, so the run removes nothing at all\n%s", out)
	}
}

func TestC2640_006_AnOpenPullRequestKeepsATreeWhoseBranchLooksLandedOrClosed(t *testing.T) {
	h := newHub(t, `{}`)
	landedDir := h.landed("landed-with-open-pr")
	h.openPR(landedDir, "landed-with-open-pr")
	reopenedDir := h.closed("closed-then-reopened")
	h.openPR(reopenedDir, "closed-then-reopened")
	h.landed("landed-no-pr")
	h.closed("closed-no-reopen")

	out, stderr, code := h.gc(false)
	requireOK(t, code, stderr)
	rows := treeRows(out)
	for _, name := range []string{"landed-with-open-pr", "closed-then-reopened"} {
		row := requireRow(t, rows, name, out)
		if row.verb != "KEEP" || !strings.Contains(row.reason, "open pull request") {
			t.Errorf("RED: %s is %s (%q), want KEEP because of the open pull request", name, row.verb, row.reason)
		}
		if !h.exists(name) {
			t.Errorf("RED: the tree %s has an open pull request and was removed", name)
		}
	}
	for _, name := range []string{"landed-no-pr", "closed-no-reopen"} {
		if h.exists(name) {
			t.Errorf("RED: the tree %s has no open pull request and stayed\n%s", name, out)
		}
	}
}

type fleetClass struct {
	prefix string
	count  int
	build  func(h *hub, name string)
}

func fleetClasses() []fleetClass {
	return []fleetClass{
		{"merged-ancestry", 25, func(h *hub, name string) { h.landed(name) }},
		{"merged-pr", 10, func(h *hub, name string) { h.squashMerged(name) }},
		{"closed-clean", 7, func(h *hub, name string) { h.closed(name) }},
		{"dirty", 14, func(h *hub, name string) { h.dirty(h.landed(name), name) }},
		{"lane", 2, func(h *hub, name string) { h.landed(name) }},
		{"open-pr", 5, func(h *hub, name string) { h.openPR(h.landed(name), name) }},
		{"closed-reopened", 3, func(h *hub, name string) { h.openPR(h.closed(name), name) }},
		{"unlanded", 20, func(h *hub, name string) { h.unlanded(name) }},
		{"fresh", 11, func(h *hub, name string) { h.fresh(name) }},
	}
}

func buildFleet(t *testing.T) (*hub, map[string]string) {
	t.Helper()
	h := newHub(t, `{"gc":{"dev_worktrees":{"protected":["code-index","lane-*"]}}}`)
	want := map[string]string{}
	h.landed("code-index")
	want["code-index"] = "protected"
	for _, class := range fleetClasses() {
		for i := 0; i < class.count; i++ {
			name := fmt.Sprintf("%s-%02d", class.prefix, i)
			class.build(h, name)
			want[name] = class.prefix
		}
	}
	return h, want
}

func isRemovedClass(class string) bool {
	return slices.Contains([]string{"merged-ancestry", "merged-pr", "closed-clean"}, class)
}

func TestC2640_007_TheDryRunOnThe98TreeFixtureListsAKeepReasonForEachTree(t *testing.T) {
	h, want := buildFleet(t)
	if len(want) != fleetTreeSize {
		t.Fatalf("fixture error: %d trees, want %d", len(want), fleetTreeSize)
	}

	out, stderr, code := h.gc(true)
	requireOK(t, code, stderr)
	rows := treeRows(out)
	if len(rows) != fleetTreeSize {
		t.Fatalf("RED: the dry run lists %d tree lines, want %d (one per tree)\n%s", len(rows), fleetTreeSize, acsassert.OutputTail(out, 30))
	}
	removed, kept := 0, 0
	for _, name := range slices.Sorted(maps.Keys(want)) {
		class := want[name]
		row, ok := rows[name]
		if !ok {
			t.Errorf("RED: no line for %s", name)
			continue
		}
		if strings.TrimSpace(row.reason) == "" {
			t.Errorf("RED: %s has no reason", name)
		}
		switch {
		case isRemovedClass(class):
			removed++
			if row.verb != "WOULD-ARCHIVE" {
				t.Errorf("RED: %s (%s) is %s, want WOULD-ARCHIVE", name, class, row.verb)
			}
		default:
			kept++
			if row.verb != "KEEP" {
				t.Errorf("RED: %s (%s) is %s, want KEEP", name, class, row.verb)
			}
		}
		switch class {
		case "protected", "lane":
			if !strings.Contains(row.reason, "protected by policy: ") {
				t.Errorf("RED: %s keeps for the reason %q, want one that names the glob", name, row.reason)
			}
		case "dirty":
			for _, rel := range []string{baseFile, "scratch-" + name + ".txt", "staged-" + name + ".txt"} {
				if !strings.Contains(row.reason, rel) {
					t.Errorf("RED: the reason of %s lacks the changed path %s: %q", name, rel, row.reason)
				}
			}
		case "open-pr", "closed-reopened":
			if !strings.Contains(row.reason, "open pull request") {
				t.Errorf("RED: %s keeps for the reason %q, want the open pull request", name, row.reason)
			}
		}
	}
	if removed != 42 || kept != 56 {
		t.Errorf("fixture error: %d removed and %d kept classes, want 42 and 56", removed, kept)
	}
	wantSummary := "evolve gc --dry-run: 42 dev worktree(s) would be archived (56 kept, 0 not judged)"
	if !strings.Contains(out, wantSummary) {
		t.Errorf("RED: the summary line %q is missing:\n%s", wantSummary, acsassert.OutputTail(out, 20))
	}
	for _, name := range slices.Sorted(maps.Keys(want)) {
		if !h.exists(name) {
			t.Errorf("RED: the dry run removed %s", name)
		}
	}
}

func TestC2640_008_TheRealVerbRemovesExactlyWhatTheDryRunPlannedAndRecordsIt(t *testing.T) {
	h := newHub(t, `{"gc":{"dev_worktrees":{"protected":["code-index"]}}}`)
	classes := map[string]string{}
	add := func(class, name string, arrange func()) {
		arrange()
		classes[name] = class
	}
	add("merged-ancestry", "merged-ancestry", func() { h.landed("merged-ancestry") })
	add("merged-pr", "merged-pr", func() { h.squashMerged("merged-pr") })
	add("closed-clean", "closed-clean", func() { h.closed("closed-clean") })
	add("dirty", "dirty", func() { h.dirty(h.landed("dirty"), "e2e") })
	add("protected", "code-index", func() { h.landed("code-index") })
	add("open-pr", "open-pr", func() { h.openPR(h.landed("open-pr"), "open-pr") })
	add("unlanded", "unlanded", func() { h.unlanded("unlanded") })
	add("fresh", "fresh", func() { h.fresh("fresh") })

	dryOut, dryErr, dryCode := h.gc(true)
	requireOK(t, dryCode, dryErr)
	planned := map[string]bool{}
	for name, row := range treeRows(dryOut) {
		if row.verb == "WOULD-ARCHIVE" {
			planned[name] = true
		}
	}
	for _, name := range slices.Sorted(maps.Keys(classes)) {
		class := classes[name]
		if isRemovedClass(class) != planned[name] {
			t.Errorf("RED: the dry run plans %s (%s) as removed=%v", name, class, planned[name])
		}
		if !h.exists(name) {
			t.Fatalf("RED: the dry run removed %s", name)
		}
	}

	out, stderr, code := h.gc(false)
	requireOK(t, code, stderr)
	for _, name := range slices.Sorted(maps.Keys(classes)) {
		class := classes[name]
		if got := !h.exists(name); got != planned[name] {
			t.Errorf("RED: after the real run %s (%s) removed=%v, the dry run planned %v\n%s", name, class, got, planned[name], out)
		}
		if !h.branchExists(name) {
			t.Errorf("RED: the branch of %s did not stay", name)
		}
	}
	records, err := gc.ReadDevArchive(filepath.Join(h.runtime, ".evolve"))
	if err != nil {
		t.Fatalf("read the dev archive record: %v", err)
	}
	if len(records) != len(planned) {
		t.Errorf("RED: the dev archive holds %d records, want %d (one per removed tree)", len(records), len(planned))
	}
}

type changedFile struct {
	rel   string
	lines map[int]bool
}

func gitOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root}, args...)...)
	if err != nil || code != 0 {
		t.Fatalf("git %s: code=%d err=%v\n%s", strings.Join(args, " "), code, err, stderr)
	}
	return stdout
}

func diffBase(t *testing.T, root string) string {
	t.Helper()
	for _, ref := range []string{"refs/heads/main", "refs/remotes/origin/main"} {
		if base, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", ref); code == 0 {
			return strings.TrimSpace(base)
		}
	}
	return "HEAD"
}

var hunkHeader = regexp.MustCompile(`^@@ -\S+ \+(\d+)(?:,\d+)? @@`)

func isProductionGo(rel string) bool {
	return strings.HasPrefix(rel, "go/") && strings.HasSuffix(rel, ".go") &&
		!strings.HasSuffix(rel, "_test.go") && !strings.HasPrefix(rel, "go/acs/")
}

func changedProduction(t *testing.T) []changedFile {
	t.Helper()
	root := acsassert.RepoRoot(t)
	var files []changedFile
	var current *changedFile
	next := 0
	for _, line := range strings.Split(gitOut(t, root, "diff", "-U0", "--no-color", diffBase(t, root), "--"), "\n") {
		switch {
		case strings.HasPrefix(line, "+++ b/"):
			current = nil
			if rel := strings.TrimPrefix(line, "+++ b/"); isProductionGo(rel) {
				files = append(files, changedFile{rel: rel, lines: map[int]bool{}})
				current = &files[len(files)-1]
			}
		case current == nil:
		case strings.HasPrefix(line, "@@"):
			if m := hunkHeader.FindStringSubmatch(line); m != nil {
				next, _ = strconv.Atoi(m[1])
			}
		case strings.HasPrefix(line, "+"):
			current.lines[next] = true
			next++
		}
	}
	for _, rel := range strings.Fields(gitOut(t, root, "ls-files", "--others", "--exclude-standard")) {
		if !isProductionGo(rel) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		all := map[int]bool{}
		for i := range strings.Split(string(raw), "\n") {
			all[i+1] = true
		}
		files = append(files, changedFile{rel: rel, lines: all})
	}
	return files
}

func changedFuncNames(t *testing.T, f changedFile) []string {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filepath.Join(acsassert.RepoRoot(t), f.rel), nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", f.rel, err)
	}
	var names []string
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		for line := fset.Position(fn.Pos()).Line; line <= fset.Position(fn.End()).Line; line++ {
			if f.lines[line] {
				names = append(names, fn.Name.Name)
				break
			}
		}
	}
	return names
}

func goOutput(t *testing.T, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), gcTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = acsassert.GoModuleDir(t), ipcenv.GoTestEnv(), 5*time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func coverRun(t *testing.T, pkg, profile string) (string, error) {
	t.Helper()
	if pkg == "./cmd/evolve" {
		return goOutput(t, "test", "-count=1", "-coverprofile="+profile, "-run", cmdCoverRun, "./cmd/evolve")
	}
	return goOutput(t, "test", "-count=1", "-coverprofile="+profile, pkg)
}

func TestC2640_009_EveryChangedProductionFunctionIsFullyCoveredByTheTestsThatDriveIt(t *testing.T) {
	files := changedProduction(t)
	byPackage := map[string][]string{}
	for _, f := range files {
		pkg := "./" + filepath.ToSlash(filepath.Dir(strings.TrimPrefix(f.rel, "go/")))
		byPackage[pkg] = append(byPackage[pkg], changedFuncNames(t, f)...)
	}
	if len(byPackage) == 0 {
		t.Fatalf("RED: no production Go file changed; the dev sweep must change")
	}
	for pkg, names := range byPackage {
		profile := filepath.Join(t.TempDir(), "cover.out")
		if out, err := coverRun(t, pkg, profile); err != nil {
			t.Fatalf("RED: go test %s exited with %v\n%s", pkg, err, acsassert.OutputTail(out, 40))
		}
		report, err := goOutput(t, "tool", "cover", "-func="+profile)
		if err != nil {
			t.Fatalf("go tool cover: %v\n%s", err, report)
		}
		slices.Sort(names)
		for _, name := range slices.Compact(names) {
			pct, err := acsassert.FuncCoverage(report, name)
			if err != nil {
				t.Errorf("RED: %s %s: %v", pkg, name, err)
				continue
			}
			if pct < 100 {
				t.Errorf("RED: %s %s is covered %.1f%%, want 100%%", pkg, name, pct)
			}
		}
	}
}

func TestC2640_010_TheInPackageTestsOfTheDevSweepPass(t *testing.T) {
	names := []string{
		"TestPlanDevWorktrees_X12_AProtectedGlobKeepsALandedTreeAndNamesTheGlob",
		"TestPlanDevWorktrees_X12_AProtectedTreeStaysEvenWhenItIsIdle",
		"TestPlanDevWorktrees_X12_WithNoPolicyNoNameIsProtected",
		"TestPlanDevWorktrees_X12_ADirtyLandedTreeIsKeptAndItsChangedPathsAreListed",
		"TestApplyDevWorktrees_X12_AStalePlanCannotArchiveATreeThatPolicyNowProtects",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/gc",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}
