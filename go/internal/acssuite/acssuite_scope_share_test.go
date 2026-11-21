package acssuite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
)

func TestScopeShare_SplitsTheLaneShareAmongTheScopesThatRunAtOnce(t *testing.T) {
	cases := []struct {
		scopes, share, slots, procs int
	}{
		{3, 0, 1, 0},
		{3, 1, 1, 1},
		{3, 2, 2, 1},
		{3, 4, 3, 1},
		{46, 1, 1, 1},
		{46, 4, 4, 1},
		{46, 14, 14, 1},
		{2, 14, 2, 7},
		{1, 4, 1, 4},
		{46, 0, 1, 0},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("scopes%d_share%d", c.scopes, c.share), func(t *testing.T) {
			slots, procs := scopeShare(c.scopes, c.share)
			if slots != c.slots || procs != c.procs {
				t.Errorf("scopeShare(%d, %d) = %d slots, %d procs, want %d, %d", c.scopes, c.share, slots, procs, c.slots, c.procs)
			}
			if c.share > 0 && slots*procs > c.share {
				t.Errorf("%d slots x %d procs ask for more than the %d CPUs of the lane", slots, procs, c.share)
			}
		})
	}
}

const fakeGoScript = `#!/bin/sh
for pattern; do :; done
rel="${pattern#./}"
case "$1" in
list) printf '{"Dir":"%s/%s","ImportPath":"fixture/%s","TestGoFiles":["acs_test.go"]}\n' "$PWD" "$rel" "$rel" ;;
test) printf '%s|%s\n' "$*" "$GOMAXPROCS" >> LOGFILE; printf '{"Action":"pass","Package":"fixture/%s","Test":"TestDeclared","Elapsed":0.01}\n' "$rel" ;;
esac
`

func TestRun_EveryConcurrentChildGetsTheSplitCPUShare(t *testing.T) {
	requireParallelHost(t)
	bin, logDir := t.TempDir(), t.TempDir()
	log := filepath.Join(logDir, "go-test.log")
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(strings.Replace(fakeGoScript, "LOGFILE", strconv.Quote(log), 1)), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	setLaneShare(t, "1")
	root := t.TempDir()
	writeFixtureModule(t, root, map[string]string{
		"go/go.mod":                       "module fixture\n\ngo 1.23\n",
		"go/acs/cycle7/acs_test.go":       "package cycle7\n\nimport \"testing\"\n\nfunc TestDeclared(t *testing.T) {}\n",
		"go/acs/regression/a/acs_test.go": "package a\n\nimport \"testing\"\n\nfunc TestDeclared(t *testing.T) {}\n",
		"go/acs/regression/b/acs_test.go": "package b\n\nimport \"testing\"\n\nfunc TestDeclared(t *testing.T) {}\n",
		"go/acs/regression/c/acs_test.go": "package c\n\nimport \"testing\"\n\nfunc TestDeclared(t *testing.T) {}\n",
		"go/acs/redteam/acs_test.go":      "package redteam\n\nimport \"testing\"\n\nfunc TestDeclared(t *testing.T) {}\n",
	})
	share := lanebudget.Procs(runtime.NumCPU(), 1)
	const scopes = 5
	_, procs := scopeShare(scopes, share)
	wantProcs := strconv.Itoa(procs)

	_, err := Run(Options{Root: root, Cycle: 7, GoTimeout: generousBudget})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("the fake go never ran a test: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != scopes {
		t.Fatalf("go test ran %d times, want one per scope (%d): %q", len(lines), scopes, lines)
	}
	for _, line := range lines {
		args, gomaxprocs, _ := strings.Cut(line, "|")
		want := "test -p " + wantProcs + " -parallel " + wantProcs + " "
		if !strings.HasPrefix(args, want) || gomaxprocs != wantProcs {
			t.Errorf("child ran %q with GOMAXPROCS=%q, want the split share %s (lane share %d over %d scopes)", args, gomaxprocs, wantProcs, share, scopes)
		}
	}
}

func overlapPredicate(pkg, mine, theirs, wantProcs string) string {
	return fmt.Sprintf(`package %s

import (
	"os"
	"testing"
	"time"
)

func TestOverlap(t *testing.T) {
	if got := os.Getenv("GOMAXPROCS"); got != %q {
		t.Fatalf("GOMAXPROCS = %%q, want %s", got)
	}
	if err := os.WriteFile(%q, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(%q); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the other regression scope never ran at the same time")
}
`, pkg, wantProcs, wantProcs, mine, theirs)
}

func TestRun_RealRegressionScopesOverlapThroughTheInventoryPath(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs real go test children")
	}
	requireParallelHost(t)
	setLaneShare(t, "1")
	markers := t.TempDir()
	a, b := filepath.Join(markers, "a"), filepath.Join(markers, "b")
	const scopes = 3
	share := lanebudget.Procs(runtime.NumCPU(), 1)
	_, procs := scopeShare(scopes, share)
	wantProcs := strconv.Itoa(procs)
	root := t.TempDir()
	writeFixtureModule(t, root, map[string]string{
		"go/go.mod":                       "module fixture\n\ngo 1.23\n",
		"go/acs/cycle7/acs_test.go":       "package cycle7\n\nimport \"testing\"\n\nfunc TestC7_001_Ok(t *testing.T) {}\n",
		"go/acs/regression/a/acs_test.go": overlapPredicate("a", a, b, wantProcs),
		"go/acs/regression/b/acs_test.go": overlapPredicate("b", b, a, wantProcs),
	})

	v, err := Run(Options{Root: root, Cycle: 7, GoTimeout: 3 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}

	if v.Verdict != "PASS" {
		t.Errorf("verdict=%q red=%d, want PASS: %v", v.Verdict, v.RedCount, resultOrder(v))
		for _, r := range v.Results {
			if r.ResultStr == "red" {
				t.Logf("%s: %s", r.ACID, r.EvidenceExcerpt)
			}
		}
	}
}

func TestExecuteCompleteGoScope_AFailingInventoryListIsAnErrorAndNoTestRuns(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	raw, err := executeCompleteGoScope(context.Background(), t.TempDir(), "./acs/cycle1", []string{"PATH=" + bin}, 2)

	if err == nil || raw != "" {
		t.Errorf("executeCompleteGoScope = %q, %v, want an error and no output", raw, err)
	}
}
