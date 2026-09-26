package main

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
)

func apicoverGoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate go root")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// apicover.Main is the whole of the standalone binary's main, so comparing
// against it proves byte parity with cmd/apicover.
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
		// Two identical no-ops would also match; the summary line proves a run.
		if !strings.Contains(subOut.String(), "summary:") {
			t.Errorf("%s: subcommand produced no apicover report:\n%s", pkg, subOut.String())
		}
	}
}
