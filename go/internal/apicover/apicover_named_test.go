package apicover

// apicover_named_test.go graduates internal/apicover into .apicover-enforce: it
// names the one exported symbol no other test exercises — Main, the shared CLI
// entry — across its full exit-code contract. (Every other export is already
// named+covered by the moved run/classify/cover/enumerate/ignore/refs tests.)

import (
	"bytes"
	"strings"
	"testing"
)

// TestMainEntry exercises Main across its exit-code contract: 0 warning-only, 1
// under -enforce with uncovered symbols present, 2 on a flag error, and 2 on a
// measurement error — the same entry the standalone cmd/apicover binary and the
// `evolve apicover` subcommand both call. (Named TestMainEntry, not TestMain,
// which the testing package reserves for the M-hook.)
func TestMainEntry(t *testing.T) {
	// Warning-only over the sample testdata → exit 0, report names a symbol.
	var out, errBuf bytes.Buffer
	if code := Main([]string{"testdata/sample"}, &out, &errBuf); code != 0 {
		t.Fatalf("Main warning-only: code=%d, want 0; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "ExportedConst") {
		t.Errorf("warning-only report should name the uncovered ExportedConst:\n%s", out.String())
	}

	// -enforce with uncovered symbols present → exit 1.
	var o2, e2 bytes.Buffer
	if code := Main([]string{"-enforce", "testdata/sample"}, &o2, &e2); code != 1 {
		t.Errorf("Main -enforce (uncovered present): code=%d, want 1", code)
	}

	// Unknown flag → exit 2 (flag error routed to stderr).
	var o3, e3 bytes.Buffer
	if code := Main([]string{"-no-such-flag"}, &o3, &e3); code != 2 {
		t.Errorf("Main unknown flag: code=%d, want 2", code)
	}

	// -h/-help is NOT an error → exit 0 with usage on stderr, matching the old
	// flag.CommandLine (ExitOnError) behavior the standalone binary must preserve.
	var o5, e5 bytes.Buffer
	if code := Main([]string{"-h"}, &o5, &e5); code != 0 {
		t.Errorf("Main -h: code=%d, want 0 (help is not an error)", code)
	}
	if !strings.Contains(e5.String(), "-enforce") {
		t.Errorf("Main -h should print usage (mentioning -enforce) to stderr; got %q", e5.String())
	}

	// Measurement error (unreadable -cover file) → exit 2, "apicover:" on stderr.
	var o4, e4 bytes.Buffer
	if code := Main([]string{"-cover", "/nonexistent-cover-xyz.txt", "testdata/sample"}, &o4, &e4); code != 2 {
		t.Errorf("Main measurement error: code=%d, want 2", code)
	} else if !strings.Contains(e4.String(), "apicover:") {
		t.Errorf("measurement error should print 'apicover:' to stderr; got %q", e4.String())
	}
}
