//go:build acs

package cycle2339

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	censusRel     = "docs/research/flaky-tests-2026-10.md"
	claudeSpan    = "TestChannelE2E_RealFixtures_ClaudeSpan"
	syncMainStamp = "TestSyncMain_TheLoopsInboxStampsNeverBlockTheSync"
	stressRuns    = 50
	stressTimeout = 12 * time.Minute
)

func startCPULoad(t *testing.T) (stop func() int64) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var spins atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU()*2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				spins.Add(1)
			}
		}()
	}
	return func() int64 {
		cancel()
		wg.Wait()
		return spins.Load()
	}
}

func stressTest(t *testing.T, pkg, name string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	stop := startCPULoad(t)
	ctx, cancel := context.WithTimeout(context.Background(), stressTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-count", strconv.Itoa(stressRuns), "-run", "^"+name+"$", pkg)
	cmd.Dir = goDir
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	if spins := stop(); spins == 0 {
		t.Fatalf("the CPU load never ran: the %d-run proof would not be beside a loaded host", stressRuns)
	}
	text := string(out)
	if err != nil {
		t.Errorf("%s failed under -count=%d beside a CPU load: %v\n%s", name, stressRuns, err, acsassert.OutputTail(text, 40))
	}
	if passes := strings.Count(text, "--- PASS: "+name+" "); passes != stressRuns {
		t.Errorf("%s reported %d passes, want %d: a run that matched no test, or lost runs, is not a proof\n%s", name, passes, stressRuns, acsassert.OutputTail(text, 20))
	}
	if strings.Contains(text, "--- FAIL: "+name) {
		t.Errorf("%s has a --- FAIL line in a %d-run loaded stress\n%s", name, stressRuns, acsassert.OutputTail(text, 40))
	}
}

func TestC2339_001_ClaudeSpanPasses50RunsBesideALoadedHost(t *testing.T) {
	stressTest(t, "./internal/bridge", claudeSpan)
}

func TestC2339_002_SyncMainInboxStampsPasses50RunsBesideALoadedHost(t *testing.T) {
	stressTest(t, "./cmd/evolve", syncMainStamp)
}

func TestC2339_003_TheSyncMainFixturesDisableGitMaintenanceInEveryRepoTheyBuild(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	ctx, cancel := context.WithTimeout(context.Background(), stressTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-count", "1", "-run", "^TestSyncMainFixture_", "./cmd/evolve")
	cmd.Dir = goDir
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		t.Errorf("the SyncMain fixture tests are red: %v\n%s", err, acsassert.OutputTail(text, 40))
	}
	for _, name := range []string{
		"TestSyncMainFixture_TheWorkRepoDisablesGitMaintenance",
		"TestSyncMainFixture_ACloneMadeThroughTheHelperDisablesGitMaintenance",
		"TestSyncMainFixture_TheHelperStillReportsAFailingGitCommand",
	} {
		if !strings.Contains(text, "--- PASS: "+name+" ") {
			t.Errorf("%s did not pass: a missing or skipped test is not a proof", name)
		}
	}
}

var placeholderCause = regexp.MustCompile(`(?i)\b(tbd|unknown|unclear|unexplained|not proven|not known|to be determined)\b`)

func censusSection(lines []string, testName string) []string {
	var section []string
	inSection := false
	for _, line := range lines {
		isHeading := strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ")
		switch {
		case isHeading && strings.HasPrefix(line, "### ") && strings.Contains(line, testName):
			inSection = true
		case isHeading:
			inSection = false
		case inSection:
			section = append(section, line)
		}
	}
	return section
}

func bulletValue(section []string, label string) string {
	prefix := "- " + label + ":"
	for _, line := range section {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func TestC2339_004_TheCensusNamesTheCauseAndTheFixOfEachFlake(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, censusRel)
	if !acsassert.FileExists(t, doc) {
		return
	}
	raw, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("read %s: %v", censusRel, err)
	}
	body := strings.Split(string(raw), "\n")
	for _, name := range []string{claudeSpan, syncMainStamp} {
		section := censusSection(body, name)
		if len(section) == 0 {
			t.Errorf("%s has no `### F4.n` section naming %s: the cause of each flake must be on record, and the commit message repeats it", censusRel, name)
			continue
		}
		cause := bulletValue(section, "Cause")
		if len(strings.Fields(cause)) < 12 {
			t.Errorf("the section of %s has no `- Cause:` bullet of 12 words or more (got %q)", name, cause)
		}
		if placeholderCause.MatchString(cause) {
			t.Errorf("the `- Cause:` bullet of %s is a placeholder (%q): name the mechanism", name, cause)
		}
		if fix := bulletValue(section, "Fix"); len(strings.Fields(fix)) < 6 {
			t.Errorf("the section of %s has no `- Fix:` bullet of 6 words or more (got %q)", name, fix)
		}
	}
}
