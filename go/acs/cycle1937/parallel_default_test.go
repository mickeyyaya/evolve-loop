//go:build acs

package cycle1937

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var amplifiedGCScopeTests = []string{
	"TestRunGC_OnlyRefusesMalformedScopes",
	"TestRunGC_OnlyCacheDryRunTrimsNothing",
	"TestRunGC_OnlyCacheWithoutFloorTrimsStaleEntriesAndKeepsFreshOnes",
	"TestCacheTrimFreeBytes_ALargeFloorNeverWrapsToASmallerOne",
}

func goTestCmdEvolve(t *testing.T, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", append([]string{"test", "-count=1"}, append(args, "./cmd/evolve")...)...)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestC1937_009_EveryGCScopeTestOfCmdEvolveIsParallelOrListedSerial(t *testing.T) {
	listed, err := goTestCmdEvolve(t, "-list", "^("+strings.Join(amplifiedGCScopeTests, "|")+")$")
	if err != nil {
		t.Fatalf("list cmd/evolve tests: %v\n%s", err, listed)
	}
	for _, name := range amplifiedGCScopeTests {
		if !strings.Contains(listed, name) {
			t.Errorf("RED: the amplified test %s is gone from cmd/evolve; fix it, never delete it", name)
		}
	}
	out, err := goTestCmdEvolve(t, "-run", "^TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial$")
	if err != nil {
		t.Errorf("RED: the cmd/evolve parallel-default gate fails: %v\n%s", err, out)
	}
}
