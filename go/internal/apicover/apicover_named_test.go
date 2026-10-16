package apicover

import (
	"bytes"
	"strings"
	"testing"
)

func TestMainEntry(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := Main([]string{"testdata/sample"}, &out, &errBuf); code != 0 {
		t.Fatalf("Main warning-only: code=%d, want 0; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "ExportedConst") {
		t.Errorf("warning-only report should name the uncovered ExportedConst:\n%s", out.String())
	}

	var o2, e2 bytes.Buffer
	if code := Main([]string{"-enforce", "testdata/sample"}, &o2, &e2); code != 1 {
		t.Errorf("Main -enforce (uncovered present): code=%d, want 1", code)
	}

	var o3, e3 bytes.Buffer
	if code := Main([]string{"-no-such-flag"}, &o3, &e3); code != 2 {
		t.Errorf("Main unknown flag: code=%d, want 2", code)
	}

	var o5, e5 bytes.Buffer
	if code := Main([]string{"-h"}, &o5, &e5); code != 0 {
		t.Errorf("Main -h: code=%d, want 0 (help is not an error)", code)
	}
	if !strings.Contains(e5.String(), "-enforce") {
		t.Errorf("Main -h should print usage (mentioning -enforce) to stderr; got %q", e5.String())
	}

	var o4, e4 bytes.Buffer
	if code := Main([]string{"-cover", "/nonexistent-cover-xyz.txt", "testdata/sample"}, &o4, &e4); code != 2 {
		t.Errorf("Main measurement error: code=%d, want 2", code)
	} else if !strings.Contains(e4.String(), "apicover:") {
		t.Errorf("measurement error should print 'apicover:' to stderr; got %q", e4.String())
	}
}
