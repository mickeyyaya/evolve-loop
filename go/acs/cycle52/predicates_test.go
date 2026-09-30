//go:build acs

package cycle52

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC52A_001_SkipPreflight_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_SKIP_PREFLIGHT"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (skip-preflight-cli-52: bucket-4 CLI flag migration).\n"+
			"The os.Getenv read must be removed from cmd_loop_preflight.go:50; replace with cfg.SkipPreflight.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_SKIP_PREFLIGHT", f.Status, f.Cluster)
	}
}

func TestC52A_002_SkipPreflightBoot_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_SKIP_PREFLIGHT_BOOT"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (skip-preflight-cli-52: bucket-4 CLI flag migration).\n"+
			"The os.Getenv read must be removed from cmd_loop_preflight.go:27; replace with cfg.SkipPreflightBoot.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_SKIP_PREFLIGHT_BOOT", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
func TestC52A_003_SkipPreflight_AbsentFromPreflightGo(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_preflight.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_SKIP_PREFLIGHT"`) {
		t.Errorf("RED: cmd_loop_preflight.go still contains the env read \"EVOLVE_SKIP_PREFLIGHT\".\n"+
			"Builder must:\n"+
			"  1. Add SkipPreflight bool to loopConfig in cmd_loop.go\n"+
			"  2. Add --skip-preflight flag var in cmd_loop_args.go (next to --force-fresh)\n"+
			"  3. Replace os.Getenv(\"EVOLVE_SKIP_PREFLIGHT\") == \"1\" with cfg.SkipPreflight\n"+
			"  4. Update the log message at :51 to say \"(--skip-preflight)\" instead of the env var name\n"+
			"Precedent: --force-fresh migration (cycle-49 EVOLVE_FORCE_FRESH → cmd_loop_args.go:54).\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC52A_004_SkipPreflightBoot_AbsentFromPreflightGo(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_preflight.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_SKIP_PREFLIGHT_BOOT"`) {
		t.Errorf("RED: cmd_loop_preflight.go still contains the env read \"EVOLVE_SKIP_PREFLIGHT_BOOT\".\n"+
			"Builder must:\n"+
			"  1. Add SkipPreflightBoot bool to loopConfig in cmd_loop.go\n"+
			"  2. Add --skip-preflight-boot flag var in cmd_loop_args.go (next to --skip-preflight)\n"+
			"  3. Replace os.Getenv(\"EVOLVE_SKIP_PREFLIGHT_BOOT\") == \"1\" with cfg.SkipPreflightBoot\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC52A_005_LoopConfig_HasSkipPreflightField(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go")
	if !acsassert.FileMatchesRegex(t, f, `SkipPreflight\s+bool`) {
		t.Errorf("RED: cmd_loop.go does not contain 'SkipPreflight <whitespace> bool' on loopConfig.\n"+
			"Builder must add an unexported bool field to loopConfig (cmd_loop.go):\n"+
			"  SkipPreflight bool  // --skip-preflight: bypass the whole readiness gate\n"+
			"Cycle-51 lesson: DO NOT use FileContains with exact single space — gofmt\n"+
			"column-aligns struct fields when SkipPreflightBoot (longer) is a sibling.\n"+
			"Precedent: ForceFresh bool field (cmd_loop.go:74, --force-fresh migration cycle-49).\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC52A_006_LoopConfig_HasSkipPreflightBootField(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go")
	if !acsassert.FileMatchesRegex(t, f, `SkipPreflightBoot\s+bool`) {
		t.Errorf("RED: cmd_loop.go does not contain 'SkipPreflightBoot <whitespace> bool' on loopConfig.\n"+
			"Builder must add:\n"+
			"  SkipPreflightBoot bool  // --skip-preflight-boot: run cheap checks but skip real bridge-boot\n"+
			"alongside the SkipPreflight bool field (Task A AC6).\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC52A_008_CmdEvolveTests_NoSetenvSkipPreflight(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cmdDir := filepath.Join(root, "go", "cmd", "evolve")
	files := []string{
		"cmd_loop_preflight_test.go",
		"cmd_loop_failbreaker_test.go",
		"main_test.go",
	}
	for _, name := range files {
		p := filepath.Join(cmdDir, name)
		if !acsassert.FileNotContains(t, p, `"EVOLVE_SKIP_PREFLIGHT"`) {
			t.Errorf("RED: %s still contains t.Setenv/os.Setenv(\"EVOLVE_SKIP_PREFLIGHT\"...) call.\n"+
				"Builder must migrate env-based test setup to CLI arg injection:\n"+
				"  - In main_test.go TestMain: remove os.Setenv(\"EVOLVE_SKIP_PREFLIGHT\", \"1\")\n"+
				"    and add \"--skip-preflight\" to all runLoop() helper calls that relied on it.\n"+
				"  - In cmd_loop_preflight_test.go: replace t.Setenv(\"EVOLVE_SKIP_PREFLIGHT\",\"1\")\n"+
				"    with cfg.SkipPreflight=true on the loopConfig, or pass --skip-preflight arg.\n"+
				"  - In cmd_loop_failbreaker_test.go: same pattern.\n"+
				"File: %s", name, p)
		}
	}
}

func TestC52A_NEG_RowCountAtMost48(t *testing.T) {
	got := len(flagregistry.All)
	if got > 48 {
		t.Errorf("RED: len(flagregistry.All) = %d, want ≤ 48 (50 − 2 Task A flags).\n"+
			"Builder must remove exactly these 2 rows from registry_table.go:\n"+
			"  EVOLVE_SKIP_PREFLIGHT\n"+
			"  EVOLVE_SKIP_PREFLIGHT_BOOT\n"+
			"Current count %d exceeds 48 — Task A flags not yet removed.",
			got, got)
	}
}

func TestC52B_001_ShipAutoConfirm_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_SHIP_AUTO_CONFIRM"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (ship-auto-confirm-split-const-52: bucket-5 split-const).\n"+
			"The literal string must be replaced with the split-const envShipAutoConfirm in verify.go.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_SHIP_AUTO_CONFIRM", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
func TestC52B_002_VerifyGo_HasSplitConst(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "phases", "ship", "verify.go")
	if !acsassert.FileMatchesRegex(t, f, `envShipAutoConfirm\s*=\s*"EVOLVE_"\s*\+\s*"SHIP_AUTO_CONFIRM"`) {
		t.Errorf("RED: verify.go does not contain the split-const definition envShipAutoConfirm.\n"+
			"Builder must add at the top of verify.go (after package+imports):\n"+
			"  // SSOT IPC-protocol-allowed: releasepipeline/rollback→ship subprocess\n"+
			"  const envShipAutoConfirm = \"EVOLVE_\" + \"SHIP_AUTO_CONFIRM\"\n"+
			"And replace the two literal occurrences at :239 and :270 with envShipAutoConfirm.\n"+
			"Precedent: cycle-49 EVOLVE_LANE split-const (SSOT IPC comment required).\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC52B_003_VerifyGo_NoBareEnvLiteral(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "phases", "ship", "verify.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_SHIP_AUTO_CONFIRM"`) {
		t.Errorf("RED: verify.go still contains the bare literal \"EVOLVE_SHIP_AUTO_CONFIRM\".\n"+
			"Builder must replace ALL occurrences with the split-const envShipAutoConfirm:\n"+
			"  :239  opts.envBool(\"EVOLVE_SHIP_AUTO_CONFIRM\") → opts.envBool(envShipAutoConfirm)\n"+
			"  :270  error message Set EVOLVE_SHIP_AUTO_CONFIRM=1 → use envShipAutoConfirm in fmt.Sprintf\n"+
			"File: %s", f)
	}
}
