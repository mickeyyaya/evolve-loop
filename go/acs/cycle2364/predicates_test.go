//go:build acs

package cycle2364

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/phaseidentity"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	buildOnce sync.Once
	buildDir  string
	buildBin  string
	buildErr  string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

func evolveBin(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		wd, err := os.Getwd()
		if err != nil {
			buildErr = err.Error()
			return
		}
		mod, err := acsassert.ModuleDir(wd)
		if err != nil {
			buildErr = err.Error()
			return
		}
		if buildDir, err = os.MkdirTemp("", "acs2364-bin-"); err != nil {
			buildErr = err.Error()
			return
		}
		buildBin = filepath.Join(buildDir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", buildBin, "./cmd/evolve")
		cmd.Dir = mod
		cmd.WaitDelay = 10 * time.Second
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = err.Error() + "\n" + string(out)
		}
	})
	if buildErr != "" {
		t.Fatalf("RED: build ./cmd/evolve: %s", buildErr)
	}
	return buildBin
}

type tempHost struct {
	t       *testing.T
	tmp     string
	project string
	gocache string
}

func newTempHost(t *testing.T, gcBlock string) tempHost {
	t.Helper()
	root := t.TempDir()
	h := tempHost{t: t, tmp: filepath.Join(root, "tmp"), project: filepath.Join(root, "project"),
		gocache: filepath.Join(root, "gocache")}
	for _, d := range []string{h.tmp, filepath.Join(h.project, ".evolve"), h.gocache} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	policy := `{"gc":` + gcBlock + `}`
	if err := os.WriteFile(filepath.Join(h.project, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h tempHost) dir(name string, stale bool) string {
	h.t.Helper()
	p := filepath.Join(h.tmp, name)
	if err := os.MkdirAll(filepath.Join(p, "001"), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "001", "payload"), []byte("0123456789"), 0o644); err != nil {
		h.t.Fatal(err)
	}
	if stale {
		if err := os.Chtimes(p, longAgo, longAgo); err != nil {
			h.t.Fatal(err)
		}
	}
	return p
}

func (h tempHost) gc(extra ...string) (stdout, stderr string, code int) {
	h.t.Helper()
	cmd := exec.Command(evolveBin(h.t), append([]string{"gc", "--only", "cache", "--project-root", h.project}, extra...)...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "TMPDIR=" + h.tmp, "GOCACHE=" + h.gocache}
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		h.t.Fatalf("run evolve gc: %v", err)
	}
	return out.String(), errb.String(), code
}

func (h tempHost) exists(name string) bool {
	_, err := os.Stat(filepath.Join(h.tmp, name))
	return err == nil
}

func holdByCwd(t *testing.T, dir string) (release func()) {
	t.Helper()
	cmd := exec.Command("sleep", "600")
	cmd.Dir = dir
	return startHolder(t, cmd)
}

func holdByOpenFile(t *testing.T, file string) (release func()) {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cmd := exec.Command("sleep", "600")
	cmd.Stdin = f
	return startHolder(t, cmd)
}

func startHolder(t *testing.T, cmd *exec.Cmd) func() {
	t.Helper()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start holder process: %v", err)
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		})
	}
	t.Cleanup(release)
	return release
}

var longAgo = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

const ttlPolicy = `{"temp_ttl_hours":24}`

func TestC2364_001_OrphanSuiteTempIsRecycledAndLiveHeldIsSpared(t *testing.T) {
	h := newTempHost(t, ttlPolicy)
	orphans := []string{"TestOrphanJourney1111111111", "gittest2222222222", "TestGC_Journey3333333333"}
	for _, n := range orphans {
		h.dir(n, true)
	}
	heldCwd := h.dir("TestHeldByCwd4444444444", true)
	heldFile := h.dir("gittest5555555555", true)
	holdByCwd(t, filepath.Join(heldCwd, "001"))
	holdByOpenFile(t, filepath.Join(heldFile, "001", "payload"))
	fresh := h.dir("TestFreshRun6666666666", false)
	foreign := h.dir("com.apple.launchd.x", true)
	notPipeline := h.dir("tmpabc123", true)

	stdout, stderr, code := h.gc()

	if code != 0 {
		t.Fatalf("evolve gc --only cache exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	for _, n := range orphans {
		if h.exists(n) {
			t.Errorf("%s: orphaned suite temp dir survived evolve gc", n)
		}
	}
	for _, p := range []string{heldCwd, heldFile, fresh, foreign, notPipeline} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was removed though it is held, fresh or foreign", filepath.Base(p))
		}
	}
}

func TestC2364_002_LiveHeldPipelineArtifactsAreSparedNotReaped(t *testing.T) {
	h := newTempHost(t, ttlPolicy)
	heldCwd := h.dir("go-build7654321", true)
	heldFile := h.dir("acs-cycle1515-bin-2718281828", true)
	orphan := h.dir("go-build1234567", true)
	holdByCwd(t, filepath.Join(heldCwd, "001"))
	holdByOpenFile(t, filepath.Join(heldFile, "001", "payload"))

	stdout, stderr, code := h.gc()

	if code != 0 {
		t.Fatalf("evolve gc --only cache exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if h.exists(filepath.Base(orphan)) {
		t.Errorf("orphaned go-build dir survived evolve gc")
	}
	for _, p := range []string{heldCwd, heldFile} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s: a live process holds it, yet evolve gc removed it", filepath.Base(p))
		}
	}
}

func TestC2364_003_DirBecomesReapableOnceItsHolderExits(t *testing.T) {
	h := newTempHost(t, ttlPolicy)
	p := h.dir("TestReleased7777777777", true)
	release := holdByCwd(t, filepath.Join(p, "001"))

	if _, stderr, code := h.gc(); code != 0 {
		t.Fatalf("first evolve gc exit=%d stderr=%s", code, stderr)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("held dir removed while its holder was alive: %v", err)
	}

	release()
	if _, stderr, code := h.gc(); code != 0 {
		t.Fatalf("second evolve gc exit=%d stderr=%s", code, stderr)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("dir was not recycled after its holder exited")
	}
}

func warnLines(stderr, key string) []string {
	var out []string
	for _, l := range strings.Split(stderr, "\n") {
		if strings.Contains(l, "WARN") && strings.Contains(l, key) && !strings.Contains(l, "policy load failed") {
			out = append(out, l)
		}
	}
	return out
}

func TestC2364_004_FreeDiskBelowFloorSignalsAndStillSweeps(t *testing.T) {
	h := newTempHost(t, `{"temp_ttl_hours":24,"disk_floor_gb":1000000000,"disk_warn_gb":1000000000}`)
	h.dir("TestOrphanFloor8888888888", true)

	stdout, stderr, _ := h.gc()

	if len(warnLines(stderr, "gc.disk_floor_gb")) == 0 {
		t.Errorf("no WARN line names gc.disk_floor_gb although free disk is under the floor\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	if h.exists("TestOrphanFloor8888888888") {
		t.Errorf("the floor signal replaced the sweep: the orphan survived")
	}
}

func TestC2364_005_WarnSignalFiresBeforeTheFloorIsBreached(t *testing.T) {
	h := newTempHost(t, `{"temp_ttl_hours":24,"disk_floor_gb":1,"disk_warn_gb":1000000000}`)
	free, err := looppreflight.DiskFreeBytes(h.tmp)
	if err != nil || free < 2<<30 {
		t.Skipf("host free disk %d bytes (err=%v) is not above the 1 GiB floor", free, err)
	}

	_, stderr, _ := h.gc()

	if len(warnLines(stderr, "gc.disk_warn_gb")) == 0 {
		t.Errorf("no WARN line names gc.disk_warn_gb although free disk is under the warn level\nstderr=%s", stderr)
	}
	if got := warnLines(stderr, "gc.disk_floor_gb"); len(got) != 0 {
		t.Errorf("floor breach reported while free disk is above the floor: %v", got)
	}
}

func TestC2364_006_NoSignalWhileFreeDiskIsAboveFloorAndWarn(t *testing.T) {
	h := newTempHost(t, `{"temp_ttl_hours":24,"disk_floor_gb":1,"disk_warn_gb":2}`)
	free, err := looppreflight.DiskFreeBytes(h.tmp)
	if err != nil || free < 3<<30 {
		t.Skipf("host free disk %d bytes (err=%v) is not above 3 GiB", free, err)
	}
	h.dir("TestOrphanQuiet9999999999", true)

	stdout, stderr, code := h.gc()

	if code != 0 {
		t.Fatalf("exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	for _, key := range []string{"gc.disk_floor_gb", "gc.disk_warn_gb"} {
		if got := warnLines(stderr, key); len(got) != 0 {
			t.Errorf("signal on a healthy disk for %s: %v", key, got)
		}
	}
	if h.exists("TestOrphanQuiet9999999999") {
		t.Errorf("orphan survived: the sweep must run regardless of disk state")
	}
}

func TestC2364_007_TheFloorIsStatedEvenWhenPolicySetsNoKeys(t *testing.T) {
	h := newTempHost(t, ttlPolicy)

	stdout, stderr, code := h.gc()

	if code != 0 {
		t.Fatalf("exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	stated := regexp.MustCompile(`gc\.disk_floor_gb[^\n]*\d`)
	if !stated.MatchString(stdout) {
		t.Errorf("stdout states no disk floor value (gc.disk_floor_gb <n>)\nstdout=%s", stdout)
	}
}

func TestC2364_008_SeatBriefForbidsDeletingSharedTempAndNamesGCAsTheRoute(t *testing.T) {
	brief := phaseidentity.Authority()
	sharedTemp := regexp.MustCompile(`(?i)\btemp(orary)?\b|/tmp\b|\bTMPDIR\b`)
	prohibition := regexp.MustCompile(`(?i)\b(never|do not|must not|forbidden)\b`)
	deletion := regexp.MustCompile(`(?i)\b(delet\w*|remov\w*|rm|clear\w*|purg\w*|wip\w*)\b`)
	for _, line := range strings.Split(brief, "\n") {
		if sharedTemp.MatchString(line) && prohibition.MatchString(line) && deletion.MatchString(line) && strings.Contains(line, "evolve gc") {
			return
		}
	}
	t.Errorf("no line of the seat brief forbids deleting shared temp and names `evolve gc` as the one route:\n%s", brief)
}

func TestC2364_009_SeatBriefKeepsItsTmuxAndAuthorityRules(t *testing.T) {
	brief := phaseidentity.Authority()
	for _, want := range []string{phaseidentity.AuthorityHeading, "never run `tmux kill-server`", "-L <private-socket>"} {
		if !strings.Contains(brief, want) {
			t.Errorf("seat brief lost %q", want)
		}
	}
	if !strings.Contains(brief, "evolve gc") {
		t.Errorf("seat brief does not mention evolve gc")
	}
}
