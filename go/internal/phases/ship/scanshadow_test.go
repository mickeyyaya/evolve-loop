package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/testverdict"
)

const (
	redSwitch      = "red-switch"
	redSwitchTest  = "package user\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestUserContract(t *testing.T) {\n\tif _, err := os.Stat(\"../../../.evolve/" + redSwitch + "\"); err == nil {\n\t\tt.Fatal(\"the switch outside the content key is on\")\n\t}\n}\n"
	userPackage    = "example.com/lane/internal/user"
	xonlyPackage   = "example.com/lane/internal/xonly"
	summaryOrigin  = "scanShadow.finish"
	shadowCycle    = 7
	shadowStoreDir = "testcache"
)

var testLimits = policy.ScanCacheConfig{Gate: "shadow", MaxBytes: 1 << 30, Budget: time.Minute}

type scanRun struct {
	err     error
	notes   string
	scanLog string
	events  []signalcenter.Event
}

func (r scanRun) summary(t *testing.T) map[string]string {
	t.Helper()
	var found []map[string]string
	for _, e := range r.events {
		if e.Origin == summaryOrigin {
			found = append(found, e.Fields)
		}
	}
	if len(found) != 1 {
		t.Fatalf("one scan must emit exactly one shadow summary, got %d in %v", len(found), r.events)
	}
	return found[0]
}

func (r scanRun) escapes() []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range r.events {
		if e.Code == CodeScanShadowEscape {
			out = append(out, e)
		}
	}
	return out
}

func shadowFixture(t *testing.T) (repo string) {
	t.Helper()
	repo, goDir := importerFixture(t)
	mustWrite(t, filepath.Join(goDir, "internal", "base", "base.go"), "package base\n\nconst Stop = \"old-stop\"\n\nconst Touched = 1\n")
	mustWrite(t, filepath.Join(goDir, "internal", "user", "user_test.go"), redSwitchTest)
	runGit(t, repo, "add", "go")
	runGit(t, repo, "commit", "-qm", "the user test reads a switch that no content key holds")
	mustWrite(t, filepath.Join(goDir, "internal", "base", "base.go"), "package base\n\nconst Stop = \"old-stop\"\n\nconst Touched = 2\n")
	return repo
}

func scanOnce(t *testing.T, repo string, shadow *scanShadow, center *signalcenter.Center) scanRun {
	t.Helper()
	ctx := context.Background()
	if shadow != nil {
		ctx = withScanShadow(ctx, shadow)
	}
	workspace := t.TempDir()
	var notes strings.Builder
	err := runRepoContractGateAt(ctx, "enforce", repo, "HEAD", workspace, &notes, nil)
	raw, _ := os.ReadFile(filepath.Join(workspace, scanLogName))
	return scanRun{err: err, notes: notes.String(), scanLog: string(raw), events: center.Recent()}
}

func shadowFor(storeDir string, center *signalcenter.Center) *scanShadow {
	return newScanShadow(testverdict.NewStore(storeDir, time.Now), sysexec.DefaultRunner, center, shadowCycle, testLimits)
}

func switchOn(t *testing.T, repo string) {
	t.Helper()
	mustWrite(t, filepath.Join(repo, ".evolve", redSwitch), "on\n")
}

func TestScanShadow_RunsEveryPackageAndRecordsTheHits(t *testing.T) {
	t.Parallel()
	repo := shadowFixture(t)
	store := filepath.Join(t.TempDir(), shadowStoreDir)

	firstCenter := signalcenter.New()
	first := scanOnce(t, repo, shadowFor(store, firstCenter), firstCenter)
	if first.err != nil {
		t.Fatalf("a green tree ships green with the shadow on: %v\n%s", first.err, first.notes)
	}
	got := first.summary(t)
	if got["packages"] != "2" || got["hits"] != "0" || got["recorded"] != "2" || got["escapes"] != "0" {
		t.Fatalf("an empty store records both green packages and hits none; got %v", got)
	}

	secondCenter := signalcenter.New()
	second := scanOnce(t, repo, shadowFor(store, secondCenter), secondCenter)
	if second.err != nil {
		t.Fatalf("a rescan of the same tree ships green: %v\n%s", second.err, second.notes)
	}
	got = second.summary(t)
	if got["packages"] != "2" || got["hits"] != "2" || got["recorded"] != "0" || got["escapes"] != "0" {
		t.Fatalf("a rescan of the same tree hits both recorded packages; got %v", got)
	}
	if got["hit_packages"] != userPackage+","+xonlyPackage {
		t.Fatalf("the summary names the packages that could have been skipped; got %q", got["hit_packages"])
	}
	for _, pkg := range []string{userPackage, xonlyPackage} {
		if !strings.Contains(second.scanLog, "ok  \t"+pkg+"\t") {
			t.Errorf("shadow skips nothing: %s must still have run in the rescan\n%s", pkg, second.scanLog)
		}
	}
}

func TestScanShadow_AHitThatFailsEmitsAnEscape(t *testing.T) {
	t.Parallel()
	repo := shadowFixture(t)
	store := filepath.Join(t.TempDir(), shadowStoreDir)
	seedCenter := signalcenter.New()
	if seed := scanOnce(t, repo, shadowFor(store, seedCenter), seedCenter); seed.err != nil {
		t.Fatalf("seed scan: %v", seed.err)
	}

	switchOn(t, repo)
	center := signalcenter.New()
	run := scanOnce(t, repo, shadowFor(store, center), center)

	se, ok := shiperr.AsShipError(run.err)
	if !ok || se.Code != shiperr.CodeRepoContractGate {
		t.Fatalf("the real run still decides: a red package fails the ship as REPO_CONTRACT_GATE; got %v", run.err)
	}
	escapes := run.escapes()
	if len(escapes) != 1 {
		t.Fatalf("exactly the one hit that failed is an escape; got %v", escapes)
	}
	e := escapes[0]
	if e.Severity != signalcenter.SeverityWarn || e.Module != signalcenter.ModuleShip || e.Fields["package"] != userPackage || e.Cycle != shadowCycle {
		t.Fatalf("an escape is a loud ship WARN naming the package and the cycle; got %+v", e)
	}
	if e.Fields["drift"] != "" {
		t.Fatalf("the escape code must be registered, got drift %q", e.Fields["drift"])
	}
	got := run.summary(t)
	if got["escapes"] != "1" || got["hits"] != "1" {
		t.Fatalf("the summary counts the escape and the hit that stayed green; got %v", got)
	}
}

type verdictTrace struct {
	err   string
	lines []string
}

var (
	elapsedRE = regexp.MustCompile(`in \d+s`)
	seconds   = regexp.MustCompile(`\d+\.\d+s`)
)

func traceOf(r scanRun) verdictTrace {
	var lines []string
	for _, l := range strings.Split(r.notes, "\n") {
		if strings.HasPrefix(l, "[ship]") {
			l = elapsedRE.ReplaceAllString(l, "in Ns")
			l = seconds.ReplaceAllString(l, "Ns")
			lines = append(lines, l)
		}
	}
	msg := ""
	if r.err != nil {
		msg = seconds.ReplaceAllString(r.err.Error(), "Ns")
	}
	return verdictTrace{err: msg, lines: lines}
}

func normalizeScanLogPath(r scanRun) scanRun {
	r.notes = regexp.MustCompile(`full output: \S+`).ReplaceAllString(r.notes, "full output: LOG")
	return r
}

func TestScanShadow_ChangesNoVerdict(t *testing.T) {
	t.Parallel()
	for _, tree := range []string{"green", "red"} {
		repo := shadowFixture(t)
		if tree == "red" {
			switchOn(t, repo)
		}
		off := traceOf(normalizeScanLogPath(scanOnce(t, repo, nil, signalcenter.New())))
		if (tree == "red") != (off.err != "") {
			t.Fatalf("%s tree: baseline verdict is wrong: %q", tree, off.err)
		}

		populated := filepath.Join(t.TempDir(), shadowStoreDir)
		greenRepo := shadowFixture(t)
		seedCenter := signalcenter.New()
		scanOnce(t, greenRepo, shadowFor(populated, seedCenter), seedCenter)

		blockedStore := filepath.Join(t.TempDir(), "blocked")
		mustWrite(t, blockedStore, "a file where the store directory should be\n")

		variants := map[string]*scanShadow{}
		center := signalcenter.New()
		variants["empty store"] = shadowFor(filepath.Join(t.TempDir(), shadowStoreDir), center)
		variants["populated store"] = shadowFor(populated, center)
		variants["blocked store"] = shadowFor(blockedStore, center)
		variants["no git or go"] = newScanShadow(testverdict.NewStore(filepath.Join(t.TempDir(), shadowStoreDir), time.Now), brokenRunner, center, shadowCycle, testLimits)
		names := make([]string, 0, len(variants))
		for name := range variants {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			on := traceOf(normalizeScanLogPath(scanOnce(t, repo, variants[name], center)))
			if on.err != off.err {
				t.Errorf("%s tree, %s: the verdict must not change\n off: %q\n  on: %q", tree, name, off.err, on.err)
			}
			if !slices.Equal(on.lines, off.lines) {
				t.Errorf("%s tree, %s: the ship notes and their order must not change\n off: %q\n  on: %q", tree, name, off.lines, on.lines)
			}
		}
	}
}

func brokenRunner(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return -1, errors.New("no toolchain on this host")
}

func panickingClock() time.Time { panic("the clock exploded") }

func TestScanShadow_APanicInTheStoreNeverChangesTheGate(t *testing.T) {
	t.Parallel()
	repo := shadowFixture(t)
	off := traceOf(normalizeScanLogPath(scanOnce(t, repo, nil, signalcenter.New())))

	center := signalcenter.New()
	store := testverdict.NewStore(filepath.Join(t.TempDir(), shadowStoreDir), panickingClock)
	shadow := newScanShadow(store, sysexec.DefaultRunner, center, shadowCycle, testLimits)
	on := scanOnce(t, repo, shadow, center)

	got := traceOf(normalizeScanLogPath(on))
	if got.err != off.err || !slices.Equal(got.lines, off.lines) {
		t.Fatalf("a panic in the shadow must leave the notes and the verdict byte-identical\n off: %q %q\n  on: %q %q", off.err, off.lines, got.err, got.lines)
	}
	var faults []signalcenter.Event
	for _, e := range on.events {
		if e.Code == CodeScanShadowFault {
			faults = append(faults, e)
		}
	}
	if len(faults) != 1 || faults[0].Severity != signalcenter.SeverityWarn || faults[0].Fields["stage"] != "settle" {
		t.Fatalf("a panic is never silent: exactly one WARN for the scan, naming the stage; got %v", faults)
	}
	if sum := on.summary(t); !strings.Contains(sum["degraded"], "the clock exploded") || sum["packages"] != "2" {
		t.Fatalf("the summary carries the cause and still counts both packages; got %v", sum)
	}
}

func TestScanShadow_APanicInBeginDegradesTheScanAndWarns(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()
	panicking := func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		panic("the runner exploded")
	}
	s := newScanShadow(testverdict.NewStore(filepath.Join(t.TempDir(), shadowStoreDir), time.Now), panicking, center, shadowCycle, testLimits)
	s.begin(context.Background(), t.TempDir())
	s.observe(packOutcome{pkgs: []pkgResult{green("p/a")}})
	s.finish()
	run := scanRun{events: center.Recent()}
	sum := run.summary(t)
	if !strings.Contains(sum["degraded"], "panic in begin") || sum["unkeyed"] != "1" {
		t.Fatalf("a panic in begin degrades the scan and keys nothing; got %v", sum)
	}
	warns := 0
	for _, e := range run.events {
		if e.Code == CodeScanShadowFault {
			warns++
		}
	}
	if warns != 1 {
		t.Fatalf("one WARN; got %d", warns)
	}
}

func TestScanShadow_RepeatedPanicsWarnOncePerScan(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()
	s := newScanShadow(testverdict.NewStore(filepath.Join(t.TempDir(), shadowStoreDir), panickingClock), healthyTools().run, center, shadowCycle, testLimits)
	s.begin(context.Background(), t.TempDir())
	s.observe(packOutcome{pkgs: []pkgResult{green("p/a"), green("p/b"), green("p/c")}})
	s.finish()
	warns := 0
	for _, e := range center.Recent() {
		if e.Code == CodeScanShadowFault {
			warns++
		}
	}
	if warns != 1 {
		t.Fatalf("three panics, one WARN; got %d", warns)
	}
}

func TestRunGoTestJSON_OnlyAShadowedScanTapsTheEventStream(t *testing.T) {
	t.Parallel()
	_, goDir := importerFixture(t)
	args := repoContractTestArgs([]string{"./internal/other"}, nil)
	log := packLog{notes: io.Discard, raw: io.Discard}

	plain := runGoTestJSON(context.Background(), goDir, log, args)
	if plain.err != nil || plain.pkgs != nil || plain.droppedLines != 0 {
		t.Fatalf("without a shadow nothing is tapped; got %+v", plain)
	}
	center := signalcenter.New()
	tapped := runGoTestJSON(withScanShadow(context.Background(), shadowFor(filepath.Join(t.TempDir(), shadowStoreDir), center)), goDir, log, args)
	if tapped.err != nil || len(tapped.pkgs) != 1 || tapped.pkgs[0].Package != "example.com/lane/internal/other" || !tapped.pkgs[0].Passed {
		t.Fatalf("with a shadow the package result is tapped; got %+v", tapped)
	}
}
