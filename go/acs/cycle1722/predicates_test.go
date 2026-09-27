//go:build acs

// Package cycle1722 materialises the acceptance criteria of the fleet-scoped
// inbox id `dashboard-unchanged-root-seq-test-flakes-under-load`.
//
// Server.refresh bypasses its "unchanged" gate whenever force is set and then
// bumps seq unconditionally under the write lock, so Run's startup
// refresh(true) and currentEpoch's on-demand refresh(true) can both publish a
// new seq for the same unchanged root. TestServer_UnchangedRootDoesNotBumpSeq
// samples seq across fixed 60ms sleeps and flakes whenever the poller's first
// refresh lands after the test's first on-demand sample.
//
//   - 001 (AC1) the unit test waits on a condition, not a fixed time.Sleep.
//   - 002 (AC2) concurrent early readers of the HTTP handler publish one seq.
//   - 003 (AC2) Run's startup refresh after an on-demand build does not re-bump.
//   - 004 (AC3) a real root change is still published by the poller.
//   - 005 (AC4) the package is vet- and gofmt-clean and its tests stay green
//     for 20 race-enabled repetitions under CPU contention.
package cycle1722

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dashboard"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	dashboardPkg      = "internal/dashboard"
	unchangedRootTest = "TestServer_UnchangedRootDoesNotBumpSeq"
)

var fixedNow = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

// TestC1722_001_UnchangedRootTestWaitsOnAConditionNotAFixedSleep is AC1.
//
// acs-predicate: source-structure — AC1 is a property of the unit test's own
// body (no fixed sleep between its samples, a condition wait instead); the
// behavioral half runs that test and requires it to pass.
func TestC1722_001_UnchangedRootTestWaitsOnAConditionNotAFixedSleep(t *testing.T) {
	root := acsassert.RepoRoot(t)
	funcs := parseTestFuncs(t, filepath.Join(root, "go", dashboardPkg))
	fn, ok := funcs[unchangedRootTest]
	if !ok {
		t.Fatalf("%s is gone from ./%s — the unchanged-root test must stay and be fixed, not removed", unchangedRootTest, dashboardPkg)
	}
	if sleeps := timeSleepCalls(fn.Body); len(sleeps) > 0 {
		t.Errorf("%s still calls time.Sleep (%d call(s)) — it must wait on the poller's publication, not a fixed duration", unchangedRootTest, len(sleeps))
	}
	if n := methodCalls(fn.Body, "current"); n < 2 {
		t.Errorf("%s takes %d current() sample(s), want >= 2 — the unchanged-root comparison must stay", unchangedRootTest, n)
	}
	if !waitsOnCondition(fn.Body, funcs, 2) {
		t.Errorf("%s has no condition wait — want a channel receive, a select, s.subscribe() or readSSEEvent, directly or in a package test helper it calls", unchangedRootTest)
	}

	out, errOut, code := runIn(t, filepath.Join(root, "go"), "go", "test", "-count=1", "-v", "-run", "^"+unchangedRootTest+"$", "./"+dashboardPkg)
	if code != 0 || !strings.Contains(out, "--- PASS: "+unchangedRootTest) {
		t.Errorf("go test -run %s exited %d without a PASS line\nstdout:\n%s\nstderr:\n%s", unchangedRootTest, code, out, errOut)
	}
}

// TestC1722_002_ConcurrentEarlyReadersPublishOneSeqForAnUnchangedRoot is AC2
// through the on-demand path: every reader that arrives before the first
// publish must see seq 1, and seq must still be 1 afterwards.
func TestC1722_002_ConcurrentEarlyReadersPublishOneSeqForAnUnchangedRoot(t *testing.T) {
	const rounds, readers = 5, 32
	for round := 1; round <= rounds; round++ {
		h := newServer(seedRoot(t), time.Hour).Handler()
		start := make(chan struct{})
		replies := make([]snapshotReply, readers)
		errs := make([]error, readers)
		var wg sync.WaitGroup
		for i := 0; i < readers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				replies[i], errs[i] = getSnapshot(h)
			}(i)
		}
		close(start)
		wg.Wait()

		seen := map[uint64]int{}
		for i, err := range errs {
			if err != nil {
				t.Fatalf("round %d reader %d: %v", round, i, err)
			}
			seen[replies[i].Seq]++
		}
		after := mustSnapshot(t, h)
		if len(seen) != 1 || seen[1] != readers || after.Seq != 1 {
			t.Fatalf("round %d: %d concurrent early readers of an unchanged root saw seqs %s and seq is %d afterwards, want every reader and the follow-up at seq 1 — each racing forced refresh re-published instead of one publish covering them all",
				round, readers, formatSeqs(seen), after.Seq)
		}
	}
}

// TestC1722_003_RunStartupRefreshDoesNotRebumpAnOnDemandBuiltRoot is AC2
// through Run: once an early reader has built and published the snapshot,
// Run's startup refresh of the same unchanged root must not publish again.
// This is the order the unit test hits when the poller goroutine starts late.
func TestC1722_003_RunStartupRefreshDoesNotRebumpAnOnDemandBuiltRoot(t *testing.T) {
	s := newServer(seedRoot(t), time.Hour)
	h := s.Handler()
	first := mustSnapshot(t, h)
	if first.Seq != 1 {
		t.Fatalf("on-demand build published seq %d, want 1", first.Seq)
	}

	// With its context already cancelled, Run performs its startup refresh and
	// returns at the first select (the hour-long ticker cannot fire first), so
	// the refresh has fully completed when Run returns.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Run(ctx)

	if after := mustSnapshot(t, h); after.Seq != first.Seq {
		t.Errorf("Run's startup refresh re-published an unchanged root: seq %d -> %d", first.Seq, after.Seq)
	}
}

// TestC1722_004_PollerStillPublishesARealRootChange is AC3, the negative
// case: coalescing must not swallow a genuine change. After an on-demand
// build, a new inbox item must reach the snapshot through the running poller
// with a strictly higher seq.
func TestC1722_004_PollerStillPublishesARealRootChange(t *testing.T) {
	root := seedRoot(t)
	s := newServer(root, 5*time.Millisecond)
	h := s.Handler()
	before := mustSnapshot(t, h)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	addInboxItem(t, root, "c1722-fresh")
	after := waitForPending(t, h, "c1722-fresh")
	if after.Seq <= before.Seq {
		t.Errorf("a real root change reached the snapshot without a new seq: %d -> %d", before.Seq, after.Seq)
	}
}

// TestC1722_005_DashboardPackageStaysGreenUnderContention is AC4: the touched
// package is vet- and gofmt-clean, and `go test -race -count=20` of the whole
// package passes while every CPU is busy — the scheduling pressure a parallel
// whole-module `go test ./...` puts on it.
func TestC1722_005_DashboardPackageStaysGreenUnderContention(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	if out, errOut, code := runIn(t, goDir, "go", "vet", "./"+dashboardPkg); code != 0 {
		t.Errorf("go vet ./%s exited %d\nstdout:\n%s\nstderr:\n%s", dashboardPkg, code, out, errOut)
	}
	if listed, errOut, code := runIn(t, goDir, "gofmt", "-l", dashboardPkg); code != 0 || strings.TrimSpace(listed) != "" {
		t.Errorf("gofmt -l %s exited %d, listed:\n%s\nstderr:\n%s", dashboardPkg, code, listed, errOut)
	}

	// Build the race binary before loading the CPUs, so the contended run
	// measures the test rather than the compiler.
	if out, errOut, code := runIn(t, goDir, "go", "test", "-race", "-count=1", "-run", "^$", "./"+dashboardPkg); code != 0 {
		t.Fatalf("race build of ./%s exited %d\nstdout:\n%s\nstderr:\n%s", dashboardPkg, code, out, errOut)
	}
	stop := spinCPUs(runtime.NumCPU())
	defer stop()
	out, errOut, code := runIn(t, goDir, "go", "test", "-race", "-count=20", "./"+dashboardPkg)
	stop()
	if code != 0 {
		t.Errorf("go test -race -count=20 ./%s under CPU contention exited %d\nstdout:\n%s\nstderr:\n%s", dashboardPkg, code, out, errOut)
	}
}

type snapshotReply struct {
	Seq   uint64 `json:"seq"`
	Queue struct {
		Pending []struct {
			ID string `json:"id"`
		} `json:"pending"`
	} `json:"queue"`
}

func (r snapshotReply) hasPending(id string) bool {
	for _, p := range r.Queue.Pending {
		if p.ID == id {
			return true
		}
	}
	return false
}

func newServer(root string, poll time.Duration) *dashboard.Server {
	return dashboard.New(root, dashboard.Options{PollInterval: poll, Now: func() time.Time { return fixedNow }})
}

// getSnapshot serves GET /api/snapshot through the production handler. Host is
// loopback because the handler's DNS-rebinding guard rejects any other name.
func getSnapshot(h http.Handler) (snapshotReply, error) {
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		return snapshotReply{}, fmt.Errorf("GET /api/snapshot: status %d: %s", rec.Code, rec.Body.String())
	}
	var r snapshotReply
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		return snapshotReply{}, fmt.Errorf("GET /api/snapshot: decode: %w", err)
	}
	return r, nil
}

func mustSnapshot(t *testing.T, h http.Handler) snapshotReply {
	t.Helper()
	r, err := getSnapshot(h)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func waitForPending(t *testing.T, h http.Handler, id string) snapshotReply {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		r := mustSnapshot(t, h)
		if r.hasPending(id) {
			return r
		}
		select {
		case <-ctx.Done():
			t.Fatalf("the poller never published inbox item %q (last seq %d): %v", id, r.Seq, ctx.Err())
		case <-tick.C:
		}
	}
}

func seedRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, id := range []string{"seed-a", "seed-b", "seed-c"} {
		writeFile(t, filepath.Join(root, ".evolve", "inbox", id+".json"), fmt.Sprintf(`{"id":%q,"title":%q,"weight":0.5}`, id, id))
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// addInboxItem stages the item outside every fingerprinted path and renames it
// in, so the poller sees one complete change rather than a half-written file.
func addInboxItem(t *testing.T, root, id string) {
	t.Helper()
	staged := writeFile(t, filepath.Join(root, "staged-"+id+".json"), fmt.Sprintf(`{"id":%q,"title":%q,"weight":0.9}`, id, id))
	if err := os.Rename(staged, filepath.Join(root, ".evolve", "inbox", id+".json")); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func formatSeqs(seen map[uint64]int) string {
	keys := make([]uint64, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d×%d", k, seen[k]))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// spinCPUs keeps n goroutines busy until the returned stop is called; stop is
// idempotent and waits for every spinner to exit.
func spinCPUs(n int) (stop func()) {
	quit := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-quit:
					return
				default:
				}
			}
		}()
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			close(quit)
			wg.Wait()
		})
	}
}

// parseTestFuncs returns the package-level functions declared in the _test.go
// files of dir, keyed by name.
func parseTestFuncs(t *testing.T, dir string) map[string]*ast.FuncDecl {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no _test.go files in %s (err=%v)", dir, err)
	}
	fset := token.NewFileSet()
	funcs := map[string]*ast.FuncDecl{}
	for _, p := range paths {
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil {
				funcs[fn.Name.Name] = fn
			}
		}
	}
	return funcs
}

func timeSleepCalls(body *ast.BlockStmt) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && isPkgFunc(call, "time", "Sleep") {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

func methodCalls(body *ast.BlockStmt, name string) int {
	n := 0
	ast.Inspect(body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
				n++
			}
		}
		return true
	})
	return n
}

// waitsOnCondition reports whether body blocks on a publication signal — a
// channel receive, a select, s.subscribe() or the package's SSE reader — either
// itself or through a package test helper it calls, up to depth levels deep.
func waitsOnCondition(body *ast.BlockStmt, funcs map[string]*ast.FuncDecl, depth int) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.SelectStmt:
			found = true
		case *ast.UnaryExpr:
			found = n.Op == token.ARROW
		case *ast.CallExpr:
			switch fun := n.Fun.(type) {
			case *ast.SelectorExpr:
				found = fun.Sel.Name == "subscribe"
			case *ast.Ident:
				if fun.Name == "readSSEEvent" {
					found = true
				} else if helper, ok := funcs[fun.Name]; ok && depth > 0 {
					found = waitsOnCondition(helper.Body, funcs, depth-1)
				}
			}
		}
		return !found
	})
	return found
}

func isPkgFunc(call *ast.CallExpr, pkg, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

// runIn runs one command with an explicit working directory — never the
// process cwd, which differs between the main tree, the worktree and each
// fleet lane.
func runIn(t *testing.T, dir, name string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Dir = dir
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	var exitErr *exec.ExitError
	if err := cmd.Run(); errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v in %s: %v", cmd.Args, dir, err)
	}
	return outBuf.String(), errBuf.String(), code
}
