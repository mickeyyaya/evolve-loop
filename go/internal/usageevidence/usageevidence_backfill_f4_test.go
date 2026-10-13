package usageevidence

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func TestAppendLine_EachFailureStageIsWrappedAndLeavesNoPartialRecord(t *testing.T) {
	t.Run("an evidence value that cannot be encoded", func(t *testing.T) {
		ws := t.TempDir()
		r := Record{Workspace: ws, Driver: "agy-tmux", Evidence: usageprobe.Evidence{
			Verdict: usageprobe.VerdictExhausted,
			Windows: []quotastate.UsageWindow{{Scope: "5h", PercentUsed: math.NaN()}},
		}}

		err := appendLine(r)

		if err == nil || !strings.HasPrefix(err.Error(), "usage evidence:") {
			t.Fatalf("err = %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(ws, core.UsageEvidenceFile)); !os.IsNotExist(statErr) {
			t.Errorf("nothing may be appended when the record cannot be encoded: %v", statErr)
		}
	})
	t.Run("the evidence file cannot be opened", func(t *testing.T) {
		err := appendLine(Record{Workspace: filepath.Join(t.TempDir(), "absent"), Driver: "d"})
		if err == nil || !strings.HasPrefix(err.Error(), "usage evidence:") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the append itself fails", func(t *testing.T) {
		orig := evidenceWrite
		t.Cleanup(func() { evidenceWrite = orig })
		boom := errors.New("disk full")
		evidenceWrite = func(*os.File, []byte) (int, error) { return 0, boom }

		err := appendLine(Record{Workspace: t.TempDir(), Driver: "d"})

		if !errors.Is(err, boom) || !strings.HasPrefix(err.Error(), "usage evidence:") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a successful append is one newline-terminated record", func(t *testing.T) {
		ws := t.TempDir()
		for i := 0; i < 2; i++ {
			if err := appendLine(Record{Workspace: ws, Driver: "d", Origin: "o", Evidence: usageprobe.Evidence{Verdict: usageprobe.VerdictHealthy}}); err != nil {
				t.Fatal(err)
			}
		}
		body, _ := os.ReadFile(filepath.Join(ws, core.UsageEvidenceFile))
		if got := strings.Count(string(body), "\n"); got != 2 || !strings.HasSuffix(string(body), "\n") {
			t.Errorf("records = %q", body)
		}
	})
}
