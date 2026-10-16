package statemap

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverfixCounterOfAcceptsEveryNumericRepresentation(t *testing.T) {
	good, err := json.Number("7").Float64()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		in   any
		want float64
		ok   bool
	}{
		"int":        {3, 3, true},
		"int64":      {int64(4), 4, true},
		"jsonNumber": {json.Number("7"), good, true},
		"badNumber":  {json.Number("x"), 0, false},
		"string":     {"5", 0, false},
	}
	for name, tc := range cases {
		got, ok := counterOf(map[string]any{"k": tc.in}, "k")
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: counterOf = (%v,%v), want (%v,%v)", name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCoverfixTodosLenCountsArrayAndFlagsAbsence(t *testing.T) {
	if got := todosLen(map[string]any{"carryoverTodos": []any{1, 2, 3}}); got != 3 {
		t.Errorf("todosLen = %d, want 3", got)
	}
	if got := todosLen(map[string]any{}); got != -1 {
		t.Errorf("todosLen absent = %d, want -1", got)
	}
}

func TestCoverfixResolveWriteTargetReturnsLinkWhenReadlinkFails(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "state.json")
	if err := os.Symlink(filepath.Join(dir, "elsewhere.json"), link); err != nil {
		t.Fatal(err)
	}
	saved := readlink
	t.Cleanup(func() { readlink = saved })
	readlink = func(string) (string, error) { return "", errors.New("boom") }
	if got := ResolveWriteTarget(link); got != link {
		t.Errorf("ResolveWriteTarget = %q, want the unresolved link %q", got, link)
	}
}

func TestCoverfixWriteStateMapWarnsWhenCarryoverTodosShrink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	many := make([]any, carryoverShrinkWarnFloor+1)
	for i := range many {
		many[i] = i
	}
	if err := WriteStateMap(path, map[string]any{"carryoverTodos": many}); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	savedErr := os.Stderr
	os.Stderr = w
	werr := WriteStateMap(path, map[string]any{"carryoverTodos": []any{}})
	os.Stderr = savedErr
	_ = w.Close()
	out, _ := io.ReadAll(r)
	if werr != nil {
		t.Fatalf("shrinking write must be allowed: %v", werr)
	}
	if !strings.Contains(string(out), "write shrinks carryoverTodos 21 -> 0") {
		t.Errorf("missing shrink warning, stderr = %q", out)
	}
}
