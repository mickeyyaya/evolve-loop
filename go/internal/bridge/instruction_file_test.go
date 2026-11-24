package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/guidance"
)

const instructionRepoRoot = "../../.."

func manifestJSON(extra string) []byte {
	return []byte(`{"cli":"x","binary":"x"` + extra + `}`)
}

func TestParseManifest_InstructionFileShape(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"", "inject", "AGENTS.md", "OTHER.md"} {
		extra := ""
		if ok != "" {
			extra = `,"instruction_file":"` + ok + `"`
		}
		if _, err := parseManifestWithStderr("x", manifestJSON(extra), &strings.Builder{}); err != nil {
			t.Errorf("instruction_file %q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"../AGENTS.md", "dir/AGENTS.md", `dir\\AGENTS.md`, "AGENTS.txt", "injects"} {
		if _, err := parseManifestWithStderr("x", manifestJSON(`,"instruction_file":"`+bad+`"`), &strings.Builder{}); err == nil {
			t.Errorf("instruction_file %q accepted", bad)
		}
	}
}

func TestRequireInstructionFile_EmptyFailsLoudly(t *testing.T) {
	t.Parallel()
	if _, err := requireInstructionFile("bare", ""); err == nil {
		t.Error("an empty instruction_file must fail loudly")
	}
	if got, err := requireInstructionFile("bare", "AGENTS.md"); err != nil || got != "AGENTS.md" {
		t.Errorf("got %q, %v; a named file passes through", got, err)
	}
}

func TestInstructionFile_UnknownCLIErrors(t *testing.T) {
	t.Parallel()
	if _, err := InstructionFile("no-such-cli"); err == nil {
		t.Error("an unknown cli must error")
	}
}

func TestRealManifests_EachResolvesAnInstructionFile(t *testing.T) {
	t.Parallel()
	names := ManifestNames()
	if len(names) < 9 {
		t.Fatalf("only %d manifests found", len(names))
	}
	injecting := 0
	for _, cli := range names {
		file, err := InstructionFile(cli)
		if err != nil {
			t.Errorf("%s: %v", cli, err)
			continue
		}
		if file == InstructionInject {
			injecting++
		}
	}
	if injecting != 1 {
		t.Errorf("%d manifests inject; only the CLI with no instruction file does", injecting)
	}
}

func TestRealManifests_InstructionFileCarriesL0(t *testing.T) {
	t.Parallel()
	l0, err := guidance.L0Region(instructionRepoRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, cli := range ManifestNames() {
		file, err := InstructionFile(cli)
		if err != nil {
			t.Errorf("%s: %v", cli, err)
			continue
		}
		if file == InstructionInject {
			continue
		}
		body, err := os.ReadFile(filepath.Join(instructionRepoRoot, file))
		if err != nil {
			t.Errorf("%s names %s: %v", cli, file, err)
			continue
		}
		if !strings.Contains(string(body), l0) {
			t.Errorf("%s names %s, which does not carry the L0 region byte for byte", cli, file)
		}
	}
}
