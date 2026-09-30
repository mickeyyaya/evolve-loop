//go:build acs

package cycle358

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func TestC358_001_ChannelFlagAbsentFromLookup(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_CHANNEL"); ok {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_CHANNEL\") returned (flag, true) — deprecated bridge flag still registered.\n"+
			"Builder must remove this row from go/internal/flagregistry/registry_table.go.\n"+
			"Current entry: Status=%q Cluster=%q RemoveIn=%q Doc=%q",
			f.Status, f.Cluster, f.RemoveIn, f.Doc)
	}
}

// acs-predicate: source-structure
func TestC358_002_EnabledFunctionNoExplicitChannelParam(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enablement := filepath.Join(root, "go", "internal", "bridge", "channel", "enablement.go")

	count, err := acsassert.CountInGoFunc(enablement, "Enabled", "explicitChannel", "deprecated bool")
	if err != nil {
		t.Fatalf("CountInGoFunc(Enabled, explicit+deprecated): %v", err)
	}
	if count != 0 {
		t.Errorf("RED: channel.Enabled still has %d line(s) containing \"explicitChannel\" or \"deprecated bool\".\n"+
			"Builder must simplify the signature to: func Enabled(stage string) bool { return stage == \"enforce\" }\n"+
			"File: %s", count, enablement)
	}

	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"./internal/bridge/channel/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("RED: go test ./internal/bridge/channel/... failed (exit=%d): %v\nOutput:\n%s",
			code, err, combined)
	}
}

func TestC358_003_NoProductionGoReferencesChannelFlag(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goSrc := filepath.Join(root, "go")
	out, _, _, _ := acsassert.SubprocessOutput("bash", "-c",
		`grep -rl "EVOLVE_CHANNEL" "`+goSrc+`" --include="*.go" 2>/dev/null | grep -v "_test.go"; true`)
	if strings.TrimSpace(out) != "" {
		t.Errorf("RED: non-test Go files still reference deprecated bridge flag EVOLVE_CHANNEL:\n%s\n"+
			"Builder must remove all production references:\n"+
			"  - go/internal/bridge/tmux_inject.go: remove lookupEnv call + WARN block\n"+
			"  - go/internal/adapters/observer/core_adapter.go: remove envGet arg + WARN block\n"+
			"  - go/internal/bridge/channel/enablement.go: simplify func (doc comment + body)",
			strings.TrimSpace(out))
	}
}

func TestC358_004_ChannelTestsPass(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"./internal/bridge/channel/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("go test ./internal/bridge/channel/... failed (exit=%d): %v\nOutput:\n%s",
			code, err, combined)
	}
}

func TestC358_005_BridgeAndObserverTestsPass(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"./internal/bridge/...",
		"./internal/adapters/observer/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("go test ./internal/bridge/... ./internal/adapters/observer/... failed (exit=%d): %v\nOutput:\n%s",
			code, err, combined)
	}
}

func TestC358_006_FlagRegistryTestsPass(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"./internal/flagregistry/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("go test ./internal/flagregistry/... failed (exit=%d): %v\nOutput:\n%s",
			code, err, combined)
	}
}

// acs-predicate: config-check
func TestC358_007_ControlFlagsDocNoChannelRow(t *testing.T) {
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")

	if !acsassert.FileExists(t, controlFlags) {
		t.Fatalf("docs/architecture/control-flags.md missing — cannot verify AC-9")
	}
	if !acsassert.FileNotContains(t, controlFlags, "| `EVOLVE_CHANNEL` |") {
		t.Errorf("RED: docs/architecture/control-flags.md still contains the EVOLVE_CHANNEL table row.\n"+
			"Builder must run `evolve flags generate` after removing the registry entry to drop this row.\n"+
			"File: %s", controlFlags)
	}
}
