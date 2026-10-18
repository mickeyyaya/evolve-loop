package core

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCoreCoverageBatch_ParseUnifiedDiffToHunks(t *testing.T) {
	t.Parallel()
	diff := strings.Join([]string{
		"diff --git a/go/x.go b/go/x.go",
		"index 1111111..2222222 100644",
		"--- a/go/x.go",
		"+++ b/go/x.go\t2026-10-18 06:00:00",
		"@@ -3,2 +3,3 @@ func x() {",
		" keep",
		"+added",
		"@@ -20 +21 @@",
		"-gone",
		"diff --git a/go/y.go b/go/y.go",
		"--- a/go/y.go",
		"+++ b/go/y.go",
		"@@ -1,0 +1,1 @@",
		"+new",
	}, "\n")
	got, err := parseUnifiedDiffToHunks([]byte(diff))
	if err != nil {
		t.Fatalf("parseUnifiedDiffToHunks: %v", err)
	}
	want := []parsedHunk{
		{MergeHunk: MergeHunk{File: "go/x.go", Header: "@@ -3,2 +3,3 @@ func x() {", Body: " keep\n+added\n"}, oldStart: 3, oldCount: 2},
		{MergeHunk: MergeHunk{File: "go/x.go", Header: "@@ -20 +21 @@", Body: "-gone\n"}, oldStart: 20, oldCount: 1},
		{MergeHunk: MergeHunk{File: "go/y.go", Header: "@@ -1,0 +1,1 @@", Body: "+new\n"}, oldStart: 1, oldCount: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d hunks, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("hunk %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCoreCoverageBatch_ParseUnifiedDiffToHunksErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		diff    string
		wantErr string
	}{
		{"malformed hunk header", "+++ b/x.go\n@@ -1 @@\n", "malformed hunk header"},
		{"line beyond the scanner buffer", "+++ b/x.go\n@@ -1 +1 @@\n+" + strings.Repeat("x", 4*1024*1024+1) + "\n", "read diff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hunks, err := parseUnifiedDiffToHunks([]byte(tc.diff))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
			}
			if hunks != nil {
				t.Errorf("hunks = %+v on error, want nil", hunks)
			}
		})
	}
}

func TestCoreCoverageBatch_ParseDiffPath(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"b/go/x.go\t2026-10-18": "go/x.go",
		"a/go/x.go":             "go/x.go",
		"  go/plain.go  ":       "go/plain.go",
		"/dev/null":             "/dev/null",
	}
	for in, want := range cases {
		if got := parseDiffPath(in); got != want {
			t.Errorf("parseDiffPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCoreCoverageBatch_ParseHunkHeader(t *testing.T) {
	t.Parallel()
	cases := []struct {
		line       string
		start, cnt int
		wantErr    string
	}{
		{"@@ -7,4 +8,5 @@ ctx", 7, 4, ""},
		{"@@ -7 +8 @@", 7, 1, ""},
		{"@@ -7 +8", 0, 0, "malformed hunk header"},
		{"## -7 +8 ##", 0, 0, "malformed hunk header"},
		{"@@ +7 -8 @@", 0, 0, "want `-old +new`"},
		{"@@ -x +8 @@", 0, 0, "bad range"},
		{"@@ -7 +8,y @@", 0, 0, "bad range"},
	}
	for _, tc := range cases {
		start, cnt, err := parseHunkHeader(tc.line)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("parseHunkHeader(%q) err = %v, want it to contain %q", tc.line, err, tc.wantErr)
			}
			continue
		}
		if err != nil || start != tc.start || cnt != tc.cnt {
			t.Errorf("parseHunkHeader(%q) = %d,%d,%v; want %d,%d,nil", tc.line, start, cnt, err, tc.start, tc.cnt)
		}
	}
}

func TestCoreCoverageBatch_ParseDiffRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in         string
		start, cnt int
		ok         bool
	}{
		{"12,3", 12, 3, true},
		{"12", 12, 1, true},
		{"x,3", 0, 0, false},
		{"12,y", 0, 0, false},
	}
	for _, tc := range cases {
		start, cnt, err := parseDiffRange(tc.in)
		if (err == nil) != tc.ok || start != tc.start || cnt != tc.cnt {
			t.Errorf("parseDiffRange(%q) = %d,%d,%v; want %d,%d,ok=%v", tc.in, start, cnt, err, tc.start, tc.cnt, tc.ok)
		}
	}
}

func TestCoreCoverageBatch_DefectAndCauseHead(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 150) + "TAIL-PART-XYZ" + strings.Repeat("b", 40)
	cases := []struct {
		name   string
		in     string
		defect string
		cause  string
	}{
		{"blank", " \n ", "", ""},
		{"cycle tokens rewritten", "cycle-1942 failed\nTestC1942_003 red in Cycle 7",
			"cycle-N failed TestCN_003 red in cycle-N", "cycle-N failed TestCN_003 red in cycle-N"},
		{"over 160 bytes", long, long[:160], "…" + long[len(long)-160:]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := defectHead(tc.in); got != tc.defect {
				t.Errorf("defectHead = %q, want %q", got, tc.defect)
			}
			if got := causeHead(tc.in); got != tc.cause {
				t.Errorf("causeHead = %q, want %q", got, tc.cause)
			}
		})
	}
}

type verifyErrLedger struct {
	fakeLedger
	err error
}

func (l *verifyErrLedger) Verify(_ context.Context) error { return l.err }

func TestCoreCoverageBatch_StampingLedgerVerify(t *testing.T) {
	t.Parallel()
	want := errors.New("chain broken at entry 4")
	s := stampingLedger{inner: &verifyErrLedger{err: want}, runID: &atomic.Value{}}
	if err := s.Verify(context.Background()); !errors.Is(err, want) {
		t.Errorf("Verify = %v, want the inner ledger's error %v", err, want)
	}
	clean := stampingLedger{inner: &fakeLedger{}, runID: &atomic.Value{}}
	if err := clean.Verify(context.Background()); err != nil {
		t.Errorf("Verify on a clean inner ledger = %v, want nil", err)
	}
}
