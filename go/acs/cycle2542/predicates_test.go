//go:build acs

package cycle2542

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const diskTotalBytes = uint64(100) << 30

type scriptedDisk struct {
	percentFree []float64
	step        int
	reads       int
	readErr     error
}

func (d *scriptedDisk) usage(string) (looppreflight.DiskUsage, error) {
	d.reads++
	if d.readErr != nil {
		return looppreflight.DiskUsage{}, d.readErr
	}
	pct := d.percentFree[d.step]
	return looppreflight.DiskUsage{FreeBytes: uint64(pct * float64(diskTotalBytes) / 100), TotalBytes: diskTotalBytes}, nil
}

func (d *scriptedDisk) advance() {
	if d.step < len(d.percentFree)-1 {
		d.step++
	}
}

type cycleStartRig struct {
	disk       *scriptedDisk
	warn       *bytes.Buffer
	trimCalls  int
	waitCalls  int
	events     []string
	trimErr    error
	guard      *looppreflight.DiskGuard
	waitResult func(context.Context) error
}

func newRig(percentFree ...float64) *cycleStartRig {
	r := &cycleStartRig{disk: &scriptedDisk{percentFree: percentFree}, warn: &bytes.Buffer{}}
	r.guard = looppreflight.NewDiskGuard(".", looppreflight.DefaultDiskGuardConfig(), r.disk.usage, r.warn)
	return r
}

func (r *cycleStartRig) options() looppreflight.CycleStartOptions {
	return looppreflight.CycleStartOptions{
		Trim: func() error {
			r.trimCalls++
			r.events = append(r.events, "trim")
			return r.trimErr
		},
		Guard: r.guard,
		Wait: func(ctx context.Context) error {
			r.waitCalls++
			r.events = append(r.events, "wait")
			if r.waitResult != nil {
				if err := r.waitResult(ctx); err != nil {
					return err
				}
			}
			r.disk.advance()
			return nil
		},
		Warn: r.warn,
	}
}

func warnLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "WARN") {
			lines = append(lines, l)
		}
	}
	return lines
}

func TestC2542_001_CycleStartRunsTheCacheTrimBeforeAnythingElse(t *testing.T) {
	r := newRig(50)

	if err := looppreflight.RunCycleStart(context.Background(), r.options()); err != nil {
		t.Fatalf("RunCycleStart with a healthy disk = %v, want nil", err)
	}

	if r.trimCalls != 1 {
		t.Fatalf("trim ran %d times at cycle start, want exactly 1", r.trimCalls)
	}
	if len(r.events) == 0 || r.events[0] != "trim" {
		t.Errorf("events = %v, want the trim first so freed space counts in the disk check", r.events)
	}
	if r.disk.reads == 0 {
		t.Error("the disk guard never read free space at cycle start")
	}
}

func TestC2542_001b_AFailingTrimWarnsAndDispatchGoesOn(t *testing.T) {
	r := newRig(50)
	r.trimErr = errors.New("remove denied")

	if err := looppreflight.RunCycleStart(context.Background(), r.options()); err != nil {
		t.Fatalf("RunCycleStart with a failing trim = %v, want nil: a cache trim failure must not stop a cycle", err)
	}

	lines := warnLines(r.warn.String())
	if len(lines) == 0 || !strings.Contains(strings.Join(lines, "\n"), "remove denied") {
		t.Errorf("warn output %q, want a WARN line naming the trim error", r.warn.String())
	}
	if r.waitCalls != 0 {
		t.Errorf("dispatch waited %d times on a healthy disk", r.waitCalls)
	}
}

func TestC2542_002_NineOfHundredFreeWarnsAndDispatchContinues(t *testing.T) {
	r := newRig(9)

	if err := looppreflight.RunCycleStart(context.Background(), r.options()); err != nil {
		t.Fatalf("RunCycleStart at 9%% free = %v, want nil", err)
	}

	if len(warnLines(r.warn.String())) == 0 {
		t.Errorf("warn output %q, want a WARN line at 9%% free", r.warn.String())
	}
	if r.waitCalls != 0 {
		t.Errorf("dispatch paused (%d waits) at 9%% free, want it to go on", r.waitCalls)
	}
	if r.guard.Check() {
		t.Error("DiskGuard.Check() = paused at 9% free, want not paused")
	}
}

func TestC2542_002b_TwentyPercentFreeIsSilent(t *testing.T) {
	r := newRig(20)

	if err := looppreflight.RunCycleStart(context.Background(), r.options()); err != nil {
		t.Fatalf("RunCycleStart at 20%% free = %v, want nil", err)
	}

	if lines := warnLines(r.warn.String()); len(lines) != 0 {
		t.Errorf("WARN lines at 20%% free: %v, want none", lines)
	}
	if r.waitCalls != 0 {
		t.Errorf("waited %d times at 20%% free", r.waitCalls)
	}
}

func TestC2542_003_FourPercentPausesAndSixPercentResumes(t *testing.T) {
	r := newRig(4, 4, 6)

	if err := looppreflight.RunCycleStart(context.Background(), r.options()); err != nil {
		t.Fatalf("RunCycleStart over 4%%,4%%,6%% free = %v, want nil once space returns", err)
	}

	if r.waitCalls != 2 {
		t.Errorf("waited %d times, want 2: paused while 4%% free, resumed at 6%%", r.waitCalls)
	}
	if r.disk.step != 2 {
		t.Errorf("disk step = %d, want 2: dispatch must not go on before the 6%% reading", r.disk.step)
	}
}

func TestC2542_003b_GuardStateFollowsTheFreePercent(t *testing.T) {
	disk := &scriptedDisk{percentFree: []float64{4}}
	guard := looppreflight.NewDiskGuard(".", looppreflight.DefaultDiskGuardConfig(), disk.usage, &bytes.Buffer{})

	if !guard.Check() {
		t.Fatal("DiskGuard.Check() at 4% free = not paused, want paused")
	}
	disk.percentFree = []float64{6}
	if guard.Check() {
		t.Error("DiskGuard.Check() at 6% free after a pause = paused, want resumed")
	}
}

func TestC2542_003c_APausedDispatchStopsWhenTheContextEnds(t *testing.T) {
	r := newRig(4)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.waitResult = func(c context.Context) error { return c.Err() }

	err := looppreflight.RunCycleStart(ctx, r.options())

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunCycleStart on a cancelled context while paused = %v, want context.Canceled", err)
	}
	if r.waitCalls != 1 {
		t.Errorf("waited %d times, want 1 before the cancel ended the pause", r.waitCalls)
	}
}

func TestC2542_004_AFailedFreeSpaceReadWarnsAndDoesNotPause(t *testing.T) {
	r := newRig(50)
	r.disk.readErr = errors.New("statfs denied")

	if err := looppreflight.RunCycleStart(context.Background(), r.options()); err != nil {
		t.Fatalf("RunCycleStart with an unreadable disk = %v, want nil", err)
	}

	joined := strings.Join(warnLines(r.warn.String()), "\n")
	if joined == "" || !strings.Contains(joined, "statfs denied") {
		t.Errorf("warn output %q, want a WARN line naming the read error", r.warn.String())
	}
	if r.waitCalls != 0 {
		t.Errorf("paused (%d waits) on a failed read, want dispatch to go on", r.waitCalls)
	}
	if r.guard.Check() {
		t.Error("DiskGuard.Check() = paused on a failed read, want not paused")
	}
}

func TestC2542_004b_AZeroTotalIsAFailedReadNotAnEmptyDisk(t *testing.T) {
	var out bytes.Buffer
	guard := looppreflight.NewDiskGuard(".", looppreflight.DefaultDiskGuardConfig(),
		func(string) (looppreflight.DiskUsage, error) { return looppreflight.DiskUsage{}, nil }, &out)

	if guard.Check() {
		t.Error("DiskGuard.Check() with total 0 = paused, want not paused: 0 free of 0 is unmeasured")
	}
	if len(warnLines(out.String())) == 0 {
		t.Errorf("warn output %q, want a WARN line for the unmeasurable disk", out.String())
	}
}

func TestC2542_005_TheProductionDiskUsageSeamMeasuresARealFilesystem(t *testing.T) {
	u, err := looppreflight.DiskUsageOf(t.TempDir())
	if err != nil {
		t.Fatalf("DiskUsageOf(temp dir) = %v", err)
	}
	if u.TotalBytes == 0 || u.FreeBytes > u.TotalBytes {
		t.Errorf("DiskUsageOf = %+v, want a non-zero total and free <= total", u)
	}
	if _, err := looppreflight.DiskUsageOf(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("DiskUsageOf(missing path) = nil error, want an error")
	}
}

func TestC2542_006_TheProductionCyclePathsCallRunCycleStart(t *testing.T) {
	root := acsassert.GoModuleDir(t)
	files, err := acsassert.WalkModule(root, func(rel string) bool {
		return strings.HasPrefix(rel, "cmd/evolve/") && strings.HasSuffix(rel, ".go") &&
			!strings.HasSuffix(rel, "_test.go") && strings.Count(strings.TrimPrefix(rel, "cmd/evolve/"), "/") == 0
	})
	if err != nil {
		t.Fatal(err)
	}
	enclosing := map[string]bool{}
	for _, rel := range files {
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(rel)), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "RunCycleStart" {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "looppreflight" {
					enclosing[fn.Name.Name] = true
				}
				return true
			})
		}
	}
	if len(enclosing) == 0 {
		t.Fatal("no production function in cmd/evolve calls looppreflight.RunCycleStart: the cycle-start trim and disk guard are dead code")
	}
	if !enclosing["LaunchFn"] {
		t.Errorf("callers %v: cycleLaunch.LaunchFn (the fleet, wave and campaign dispatch seam) must call looppreflight.RunCycleStart", keys(enclosing))
	}
	if len(enclosing) < 2 {
		t.Errorf("callers %v: the sequential cycle path must call looppreflight.RunCycleStart as well as LaunchFn", keys(enclosing))
	}
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
