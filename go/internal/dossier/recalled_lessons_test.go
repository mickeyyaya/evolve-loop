package dossier

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestBuild_RecordsRecalledLessonIDsInOrder(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		writes [][]string
		want   []string
	}{
		{name: "no recall leaves the field empty"},
		{name: "empty recall leaves the field empty", writes: [][]string{{}}},
		{name: "ids keep lookup order", writes: [][]string{{"L-b", "L-a"}}, want: []string{"L-b", "L-a"}},
		{name: "later phases append new ids without duplicates", writes: [][]string{{"L-a", "L-b"}, {"L-b", "L-c"}}, want: []string{"L-a", "L-b", "L-c"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			for _, ids := range tc.writes {
				if err := AppendRecalledLessons(ws, ids); err != nil {
					t.Fatalf("AppendRecalledLessons: %v", err)
				}
			}
			d, err := Build(7, BuildOpts{WorkspacePath: ws, Goal: "g", FinalVerdict: VerdictPass})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(d.RecalledLessons, tc.want) {
				t.Fatalf("RecalledLessons = %v, want %v", d.RecalledLessons, tc.want)
			}
		})
	}
}

func TestAppendRecalledLessons_MalformedRecordIsReplaced(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, RecalledLessonsFile), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readRecalledLessons(ws); got != nil {
		t.Fatalf("malformed record read as %v, want nil", got)
	}
	if err := AppendRecalledLessons(ws, []string{"L-a", " "}); err != nil {
		t.Fatal(err)
	}
	if got := readRecalledLessons(ws); !reflect.DeepEqual(got, []string{"L-a"}) {
		t.Fatalf("after replace = %v, want [L-a]", got)
	}
}

func TestAppendRecalledLessons_ConcurrentPhasesKeepEveryID(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	const writers = 16
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := AppendRecalledLessons(ws, []string{fmt.Sprintf("L-%02d", i)}); err != nil {
				t.Errorf("append %d: %v", i, err)
			}
		}()
	}
	wg.Wait()

	got := readRecalledLessons(ws)
	if len(got) != writers {
		t.Fatalf("recalled ids = %d %v, want all %d concurrent appends kept", len(got), got, writers)
	}
}
