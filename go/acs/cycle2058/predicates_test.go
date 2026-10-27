//go:build acs

package cycle2058

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func moduleDir(t *testing.T) string {
	t.Helper()
	dir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func runNamedTests(t *testing.T, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleDir(t),
		Package: "github.com/mickeyyaya/evolve-loop/go/cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2058_001_ExitCodesMatchTheTable(t *testing.T) {
	runNamedTests(t, "TestMsgCLI_ExitCodesMatchTheTable")
}

func TestC2058_002_SendReadsStdinAndExitsTwoWithoutAReceipt(t *testing.T) {
	runNamedTests(t, "TestMsgSend_ReadsTheBodyFromStdin", "TestMsgSend_NoReceiptByTheDeadlineExitsTwo")
}

func TestC2058_003_WaitWakesOnTheMailboxWithoutAPoll(t *testing.T) {
	runNamedTests(t, "TestMsgWait_WakesOnTheMailboxWithoutAPoll", "TestMsgWait_SourceHasNoPollTimer")
}

func TestC2058_004_ReplyOutOfScopeExitsFour(t *testing.T) {
	runNamedTests(t, "TestMsgReply_OutOfScopeExitsFour", "TestMsgReply_AnswersTheSenderWithTheCorrelationID")
}

func TestC2058_005_EveryMsgFunctionHasFullCoverage(t *testing.T) {
	dir := moduleDir(t)
	profile := filepath.Join(t.TempDir(), "cover.out")
	_, stderr, code, err := acsassert.SubprocessOutput("go", "-C", dir, "test", "-count=1", "-run", "^TestMsg", "-coverprofile="+profile, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("go test exited %d (%v):\n%s", code, err, stderr)
	}
	funcs, stderr, code, err := acsassert.SubprocessOutput("go", "-C", dir, "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover exited %d (%v):\n%s", code, err, stderr)
	}
	checked := 0
	for _, line := range strings.Split(funcs, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], "cmd/evolve/cmd_msg") {
			continue
		}
		checked++
		if fields[len(fields)-1] != "100.0%" {
			t.Errorf("not fully covered by the TestMsg tests: %s", line)
		}
	}
	if checked == 0 {
		t.Fatalf("no function of cmd_msg*.go appears in the coverage profile:\n%s", acsassert.OutputTail(funcs, 5))
	}
}

func TestC2058_006_MsgVerbIsReachableFromTheCommandTable(t *testing.T) {
	runNamedTests(t, "TestMsgCLI_IsRegisteredInTheCommandTable")
}
