package admission_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

const (
	evolveA   = "go/cmd/evolve/a.go"
	evolveB   = "go/cmd/evolve/b.go"
	evolveDir = "go/cmd/evolve"
)

func fileCellAdmitter(t *testing.T, optimistic bool, fileCells ...string) admission.Admitter {
	t.Helper()
	a := admitter(t, nil)
	a.Optimistic = optimistic
	a.FileCells = fileCells
	return a
}

func eachRule(t *testing.T, run func(t *testing.T, optimistic bool)) {
	t.Helper()
	for name, optimistic := range map[string]bool{"reach rule": false, "optimistic rule": true} {
		t.Run(name, func(t *testing.T) { run(t, optimistic) })
	}
}

func TestAdmit_TwoTodosThatEditDifferentFilesOfALeafPackageBothAdmit(t *testing.T) {
	t.Parallel()
	eachRule(t, func(t *testing.T, optimistic bool) {
		a := fileCellAdmitter(t, optimistic, "cmd/evolve")
		admitted, waiting := a.Admit(nil, []fleet.Todo{todo("x", evolveA), todo("y", evolveB)}, 32)
		if got := ids(admitted); !reflect.DeepEqual(got, []string{"x", "y"}) || len(waiting) != 0 {
			t.Fatalf("admitted = %v, waiting = %+v, want both: the files a.go and b.go of the leaf cmd/evolve share no file", got, waiting)
		}
	})
}

func TestAdmit_ALaneInFlightOnOneFileOfALeafLeavesAnotherFileFree(t *testing.T) {
	t.Parallel()
	a := fileCellAdmitter(t, false, "cmd/evolve")
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("x", evolveA)}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("y", evolveB)}, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"y"}) || len(waiting) != 0 {
		t.Fatalf("admitted = %v, waiting = %+v, want y admitted beside the lane on a.go", got, waiting)
	}
}

func TestAdmit_TwoTodosThatEditTheSameFileOfALeafPackageHoldTheSecond(t *testing.T) {
	t.Parallel()
	eachRule(t, func(t *testing.T, optimistic bool) {
		a := fileCellAdmitter(t, optimistic, "cmd/evolve")
		admitted, waiting := a.Admit(nil, []fleet.Todo{todo("x", evolveA), todo("y", evolveA, evolveB)}, 32)
		if got := ids(admitted); !reflect.DeepEqual(got, []string{"x"}) {
			t.Fatalf("admitted = %v, want [x]", got)
		}
		if len(waiting) != 1 || waiting[0].ID != "y" || waiting[0].BlockedBy != "x" {
			t.Fatalf("waiting = %+v, want y held by x: both list a.go", waiting)
		}
	})
}

func TestAdmit_TwoTodosThatEditTheSameTestFileOfALeafPackageHoldTheSecond(t *testing.T) {
	t.Parallel()
	a := fileCellAdmitter(t, false, "cmd/evolve")
	hotTestFile := "go/cmd/evolve/parallel_serial_tests_test.go"
	admitted, waiting := a.Admit(nil, []fleet.Todo{todo("x", hotTestFile), todo("y", hotTestFile)}, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"x"}) || len(waiting) != 1 {
		t.Fatalf("admitted = %v, waiting = %+v, want y held: a test file is a file key too", got, waiting)
	}
}

func TestAdmit_AWholePackageEditOfALeafHoldsAFileEditOfThatLeaf(t *testing.T) {
	t.Parallel()
	wholePackage := map[string][]string{
		"a directory entry":          {evolveDir},
		"a non-Go file of the leaf":  {"go/cmd/evolve/testdata/golden.json"},
		"a file beside a directory":  {evolveA, evolveDir},
		"a Go file beside a non-Go":  {evolveA, "go/cmd/evolve/testdata/golden.json"},
		"the directory with a slash": {evolveDir + "/"},
		"a nested directory in leaf": {"go/cmd/evolve/testdata"},
	}
	for name, files := range wholePackage {
		t.Run(name, func(t *testing.T) {
			eachRule(t, func(t *testing.T, optimistic bool) {
				a := fileCellAdmitter(t, optimistic, "cmd/evolve")
				x, y := todo("x", files...), todo("y", evolveB)
				for label, order := range map[string][]fleet.Todo{"whole first": {x, y}, "file first": {y, x}} {
					admitted, waiting := a.Admit(nil, order, 32)
					if len(admitted) != 1 || len(waiting) != 1 {
						t.Errorf("%s: admitted = %v, waiting = %+v, want the second held: %v is a whole-package edit of cmd/evolve", label, ids(admitted), waiting, files)
					}
				}
			})
		})
	}
}

func TestAdmit_TwoWholePackageEditsOfALeafHoldTheSecond(t *testing.T) {
	t.Parallel()
	a := fileCellAdmitter(t, false, "cmd/evolve")
	admitted, waiting := a.Admit(nil, []fleet.Todo{todo("x", evolveDir), todo("y", "go/cmd/evolve/testdata/x.json")}, 32)
	if len(admitted) != 1 || len(waiting) != 1 {
		t.Fatalf("admitted = %v, waiting = %+v, want one held", ids(admitted), waiting)
	}
}

func TestAdmit_TwoTodosThatEditDifferentFilesOfANonLeafPackageHoldTheSecond(t *testing.T) {
	t.Parallel()
	for name, fileCells := range map[string][]string{
		"the non-leaf is not listed": {"cmd/evolve"},
		"the non-leaf is listed":     {"cmd/evolve", "internal/fleet"},
	} {
		t.Run(name, func(t *testing.T) {
			eachRule(t, func(t *testing.T, optimistic bool) {
				a := fileCellAdmitter(t, optimistic, fileCells...)
				admitted, waiting := a.Admit(nil, []fleet.Todo{todo("x", "go/internal/fleet/a.go"), todo("y", "go/internal/fleet/b.go")}, 32)
				if got := ids(admitted); !reflect.DeepEqual(got, []string{"x"}) || len(waiting) != 1 || waiting[0].BlockedBy != "x" {
					t.Fatalf("admitted = %v, waiting = %+v, want y held: internal/fleet has importers, so a file edit there is a package edit", got, waiting)
				}
			})
		})
	}
}

func TestAdmit_WithNoFileCellsTwoFilesOfALeafStillShareOneCell(t *testing.T) {
	t.Parallel()
	for name, fileCells := range map[string][]string{
		"no list":                   nil,
		"an empty list":             {},
		"a list of another package": {"cmd/other"},
	} {
		t.Run(name, func(t *testing.T) {
			eachRule(t, func(t *testing.T, optimistic bool) {
				a := fileCellAdmitter(t, optimistic, fileCells...)
				admitted, waiting := a.Admit(nil, []fleet.Todo{todo("x", evolveA), todo("y", evolveB)}, 32)
				if got := ids(admitted); !reflect.DeepEqual(got, []string{"x"}) || len(waiting) != 1 || waiting[0].BlockedBy != "x" {
					t.Fatalf("admitted = %v, waiting = %+v, want y held behind x as before: cmd/evolve is not in file_cells", got, waiting)
				}
				if want := []string{"cmd/evolve"}; !reflect.DeepEqual(admitted[0].Cells, want) {
					t.Errorf("cells = %v, want %v: package keys only", admitted[0].Cells, want)
				}
			})
		})
	}
}

func TestAdmit_AFileKeyShowsTheFileInTheCells(t *testing.T) {
	t.Parallel()
	a := fileCellAdmitter(t, false, "cmd/evolve")
	admitted, _ := a.Admit(nil, []fleet.Todo{todo("x", evolveB, evolveA)}, 32)
	if want := []string{"cmd/evolve/a.go", "cmd/evolve/b.go"}; !reflect.DeepEqual(admitted[0].Cells, want) {
		t.Fatalf("cells = %v, want %v: the operator reads the real holder in evolve wave supply", admitted[0].Cells, want)
	}
}

func TestAdmit_AWholePackageEditOfALeafShowsThePackageInTheCells(t *testing.T) {
	t.Parallel()
	a := fileCellAdmitter(t, false, "cmd/evolve")
	admitted, _ := a.Admit(nil, []fleet.Todo{todo("x", evolveDir)}, 32)
	if want := []string{"cmd/evolve"}; !reflect.DeepEqual(admitted[0].Cells, want) {
		t.Fatalf("cells = %v, want %v", admitted[0].Cells, want)
	}
}

func TestAdmit_FileKeysAddNoReachToALeaf(t *testing.T) {
	t.Parallel()
	a := fileCellAdmitter(t, false, "cmd/evolve")
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("x", evolveA)}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("f", "go/internal/fleet/pool.go")}, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"f"}) || len(waiting) != 0 {
		t.Fatalf("admitted = %v, waiting = %+v, want f free: cmd/evolve is a leaf and holds no reach", got, waiting)
	}
}

func TestAdmit_FileKeysKeepTheHubRule(t *testing.T) {
	t.Parallel()
	a := fileCellAdmitter(t, false, "cmd/evolve")
	admitted, waiting := a.Admit(nil, []fleet.Todo{todo("x", evolveA, "go/go.mod"), todo("y", evolveB, "go/go.sum")}, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"x"}) || len(waiting) != 1 {
		t.Fatalf("admitted = %v, waiting = %+v, want y held: both todos take the hub cell, whatever file keys they hold", got, waiting)
	}
}

func TestAdmit_FileKeysKeepTheFamilyRule(t *testing.T) {
	t.Parallel()
	a := fileCellAdmitter(t, false, "cmd/evolve")
	admitted, waiting := a.Admit(nil, []fleet.Todo{todo("tokenopt-a", evolveA), todo("tokenopt-b", evolveB)}, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"tokenopt-a"}) || len(waiting) != 1 || !waiting[0].FamilyHold {
		t.Fatalf("admitted = %v, waiting = %+v, want tokenopt-b held by the family cell", got, waiting)
	}
}

func TestAdmit_FileKeysSendNoReachShadowForAFileHold(t *testing.T) {
	t.Parallel()
	a := fileCellAdmitter(t, false, "cmd/evolve")
	var shadows []admission.ShadowAdmission
	a.OnShadow = func(s admission.ShadowAdmission) { shadows = append(shadows, s) }
	a.Admit(nil, []fleet.Todo{todo("x", evolveA), todo("y", evolveA)}, 32)
	if len(shadows) != 0 {
		t.Fatalf("shadows = %+v, want none: a shared file is a shared edit, not a reach hold", shadows)
	}
}

func TestAdmit_TwoAdmittedLeafTodosNeverShareAFileOrAWholePackageOverRandomBacklogs(t *testing.T) {
	t.Parallel()
	pool := []string{evolveA, evolveB, "go/cmd/evolve/c.go", "go/cmd/evolve/d_test.go", evolveDir, "go/cmd/evolve/testdata/x.json"}
	wholePackageEntries := []string{evolveDir, "go/cmd/evolve/testdata/x.json"}
	conflicts := func(x, y []string) bool {
		whole := func(files []string) bool {
			return slices.ContainsFunc(files, func(f string) bool { return slices.Contains(wholePackageEntries, f) })
		}
		if whole(x) || whole(y) {
			return true
		}
		return slices.ContainsFunc(x, func(f string) bool { return slices.Contains(y, f) })
	}
	eachRule(t, func(t *testing.T, optimistic bool) {
		r := rand.New(rand.NewSource(4617))
		for round := 0; round < 200; round++ {
			a := fileCellAdmitter(t, optimistic, "cmd/evolve")
			files := map[string][]string{}
			var candidates []fleet.Todo
			for i := 0; i < 6; i++ {
				id := fmt.Sprintf("t%d", i)
				for _, f := range pool {
					if r.Intn(3) == 0 {
						files[id] = append(files[id], f)
					}
				}
				if len(files[id]) == 0 {
					files[id] = []string{pool[r.Intn(len(pool))]}
				}
				candidates = append(candidates, todo(id, files[id]...))
			}
			admitted, waiting := a.Admit(nil, candidates, 32)
			for i, x := range admitted {
				for _, y := range admitted[i+1:] {
					if conflicts(files[x.ID], files[y.ID]) {
						t.Fatalf("round %d: admitted %s %v and %s %v conflict", round, x.ID, files[x.ID], y.ID, files[y.ID])
					}
				}
			}
			for _, w := range waiting {
				held := false
				for _, x := range admitted {
					held = held || (x.Rank < w.Rank && conflicts(files[x.ID], files[w.ID]))
				}
				if !held {
					t.Fatalf("round %d: %s %v waits but conflicts with no earlier admitted todo", round, w.ID, files[w.ID])
				}
			}
		}
	})
}
