package main

import (
	"strings"
	"testing"
)

func TestParseCoverFunc_ParsesFuncLineAndSkipsTotal(t *testing.T) {
	in := "github.com/x/y/foo.go:12:\tExportedFunc\t87.5%\n" +
		"github.com/x/y/sub/bar.go:30:\tMethod\t0.0%\n" +
		"total:\t(statements)\t91.2%\n"

	entries, err := ParseCoverFunc(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseCoverFunc: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (total: line must be skipped): %+v", len(entries), entries)
	}
	want0 := CoverEntry{Path: "github.com/x/y/foo.go", File: "foo.go", Line: 12, Func: "ExportedFunc", Pct: 87.5}
	if entries[0] != want0 {
		t.Errorf("entry0 = %+v, want %+v", entries[0], want0)
	}
	want1 := CoverEntry{Path: "github.com/x/y/sub/bar.go", File: "bar.go", Line: 30, Func: "Method", Pct: 0.0}
	if entries[1] != want1 {
		t.Errorf("entry1 = %+v, want %+v", entries[1], want1)
	}
}

func TestParseCoverFunc_HandlesSpacesInPath(t *testing.T) {
	// Developer machines often have spaces in the module path (e.g. /Users/Dan Lee).
	in := "/Users/Dan Lee/proj/foo.go:7:\tFn\t50.0%\n"
	entries, err := ParseCoverFunc(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseCoverFunc: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 (path with spaces must not be dropped): %+v", len(entries), entries)
	}
	want := CoverEntry{Path: "/Users/Dan Lee/proj/foo.go", File: "foo.go", Line: 7, Func: "Fn", Pct: 50.0}
	if entries[0] != want {
		t.Errorf("entry = %+v, want %+v", entries[0], want)
	}
}
