package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

func TestRunSalvageList_LandedIsATriStateThatNeverFailsTheRun(t *testing.T) {
	const head = "0123456789abcdef0123456789abcdef01234567"
	cases := []struct {
		name     string
		code     int
		err      error
		wantJSON any
		wantText string
	}{
		{"ancestor", 0, nil, true, "landed=yes"},
		{"not an ancestor", 1, nil, false, "landed=no"},
		{"absent commit", 128, nil, nil, "landed=unknown"},
		{"runner error", -1, errors.New("git not found"), nil, "landed=unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			leaf := filepath.Join(gc.OperatorSalvageDir(filepath.Join(root, ".evolve")), "cycle-abc-7")
			if err := os.MkdirAll(leaf, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(leaf, "HEAD"), []byte(head+" cycle-abc-7\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var calls [][]string
			prev := salvageGitRunner
			t.Cleanup(func() { salvageGitRunner = prev })
			salvageGitRunner = func(_ context.Context, name, dir string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
				calls = append(calls, append([]string{name, dir}, args...))
				return tc.code, tc.err
			}

			var jsonOut, textOut, errb bytes.Buffer
			jsonCode := runSalvage([]string{"list", "--json", "--project-root", root}, nil, &jsonOut, &errb)
			textCode := runSalvage([]string{"list", "--project-root", root}, nil, &textOut, &errb)

			if jsonCode != 0 || textCode != 0 || errb.Len() != 0 {
				t.Fatalf("rc json=%d text=%d stderr=%q, want 0 and silence: an unknown landed state never fails the run", jsonCode, textCode, errb.String())
			}
			var rows []map[string]any
			if err := json.Unmarshal(jsonOut.Bytes(), &rows); err != nil || len(rows) != 1 {
				t.Fatalf("json rows %q err=%v", jsonOut.String(), err)
			}
			if got, present := rows[0]["landed"]; !present || !reflect.DeepEqual(got, tc.wantJSON) {
				t.Errorf("landed = %#v (present=%v), want %#v", got, present, tc.wantJSON)
			}
			if !strings.Contains(textOut.String(), tc.wantText) || !strings.Contains(textOut.String(), "branch=cycle-abc-7") {
				t.Errorf("text row %q lacks %q", textOut.String(), tc.wantText)
			}
			want := []string{"git", root, "merge-base", "--is-ancestor", head, "origin/main"}
			for _, c := range calls {
				if !reflect.DeepEqual(c, want) {
					t.Errorf("salvage list ran %v, want only %v", c, want)
				}
			}
			if len(calls) != 2 {
				t.Errorf("salvage list ran git %d times over two invocations, want one merge-base per leaf per run", len(calls))
			}
		})
	}
}
