//go:build acs

package cycle1892

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func buildLedgerTestBinary(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	bin := filepath.Join(t.TempDir(), "ledger.test")
	cmd := exec.Command("go", "test", "-c", "-o", bin, "./internal/adapters/ledger")
	cmd.Dir = filepath.Join(root, "go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile ledger test binary: %v\n%s", err, out)
	}
	return bin
}

func runRealCopyTestAgainstLedger(t *testing.T, bin string, entries int) (string, string, error) {
	t.Helper()
	base := t.TempDir()
	evolveDir := filepath.Join(base, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	l := ledger.New(evolveDir)
	for i := 0; i < entries; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{
			Cycle: i, Role: "builder", Kind: "phase_complete",
			Message: fmt.Sprintf("entry-%d", i),
		}); err != nil {
			t.Fatalf("seed ledger entry %d: %v", i, err)
		}
	}
	cwd := filepath.Join(base, "a", "b", "c", "d")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-test.run", "^TestSeal_RealLedgerCopy$", "-test.v")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	return string(out), filepath.Join(evolveDir, "ledger.jsonl"), err
}

func TestC1892_001_ShortLedgerSkipsNamingPathAndLineCount(t *testing.T) {
	bin := buildLedgerTestBinary(t)
	out, ledgerPath, err := runRealCopyTestAgainstLedger(t, bin, 2)
	if err != nil {
		t.Errorf("RED: short ledger must not fail TestSeal_RealLedgerCopy: %v\n%s", err, out)
	}
	if !strings.Contains(out, "--- SKIP: TestSeal_RealLedgerCopy") {
		t.Errorf("RED: short ledger must skip, output:\n%s", out)
	}
	if !strings.Contains(out, filepath.Base(filepath.Dir(ledgerPath))+"/ledger.jsonl") || !strings.Contains(out, "2 lines") {
		t.Errorf("RED: skip message must name the ledger path and its 2-line count, output:\n%s", out)
	}
}

func TestC1892_002_BoundaryLedgerOfExactlyOneHundredLinesSkips(t *testing.T) {
	bin := buildLedgerTestBinary(t)
	out, _, err := runRealCopyTestAgainstLedger(t, bin, 100)
	if err != nil {
		t.Errorf("RED: 100-line ledger must not fail: %v\n%s", err, out)
	}
	if !strings.Contains(out, "--- SKIP: TestSeal_RealLedgerCopy") || !strings.Contains(out, "100 lines") {
		t.Errorf("RED: 100-line ledger must skip naming 100 lines, output:\n%s", out)
	}
}

func TestC1892_003_LongLedgerStillSealsAndPasses(t *testing.T) {
	bin := buildLedgerTestBinary(t)
	out, _, err := runRealCopyTestAgainstLedger(t, bin, 150)
	if err != nil {
		t.Fatalf("long ledger must pass: %v\n%s", err, out)
	}
	if !strings.Contains(out, "--- PASS: TestSeal_RealLedgerCopy") || strings.Contains(out, "--- SKIP") {
		t.Errorf("long ledger must seal and pass without skipping, output:\n%s", out)
	}
}
