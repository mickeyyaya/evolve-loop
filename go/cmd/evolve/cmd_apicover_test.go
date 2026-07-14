package main

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
)

// apicoverGoRoot locates go/ from this test file (cmd/evolve/ → two up).
func apicoverGoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate go root")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// TestApicoverSubcommand_ByteParityWithStandalone proves `evolve apicover`
// reproduces the standalone cmd/apicover exactly. The standalone's main() is
// literally os.Exit(apicover.Main(os.Args[1:], os.Stdout, os.Stderr)), so
// apicover.Main IS the standalone behavior; the subcommand handler runApicover
// must emit byte-identical stdout/stderr and the same exit code across enforced
// packages (guards against the subcommand munging args or swapping streams).
func TestApicoverSubcommand_ByteParityWithStandalone(t *testing.T) {
	root := apicoverGoRoot(t)
	pkgs := []string{
		filepath.Join(root, "internal", "skillcheck"),
		filepath.Join(root, "internal", "flagregistry"),
		filepath.Join(root, "internal", "soakreport"),
	}
	for _, pkg := range pkgs {
		args := []string{pkg}

		var subOut, subErr bytes.Buffer
		subCode := runApicover(args, nil, &subOut, &subErr)

		var libOut, libErr bytes.Buffer
		libCode := apicover.Main(args, &libOut, &libErr)

		if subCode != libCode {
			t.Errorf("%s: exit code sub=%d lib=%d", pkg, subCode, libCode)
		}
		if subOut.String() != libOut.String() {
			t.Errorf("%s: stdout diverged\n--sub--\n%s\n--lib--\n%s", pkg, subOut.String(), libOut.String())
		}
		if subErr.String() != libErr.String() {
			t.Errorf("%s: stderr diverged\n--sub--\n%s\n--lib--\n%s", pkg, subErr.String(), libErr.String())
		}
		// Prove apicover actually ran (not two identical no-ops): the report
		// always ends with a summary line.
		if !strings.Contains(subOut.String(), "summary:") {
			t.Errorf("%s: subcommand produced no apicover report:\n%s", pkg, subOut.String())
		}
	}
}
