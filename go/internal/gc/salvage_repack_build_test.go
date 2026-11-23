package gc

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSalvageRepack_ARepackKeepsTheOldBundleOnEachFailedStep(t *testing.T) {
	steps := map[string]func(args []string) bool{
		"scratch init": func(a []string) bool { return a[0] == "init" },
		"set head ref": func(a []string) bool { return a[0] == "update-ref" },
		"create":       func(a []string) bool { return len(a) > 1 && a[0] == "bundle" && a[1] == "create" },
		"verify the new": func(a []string) bool {
			return len(a) > 2 && a[1] == "verify" && strings.HasSuffix(a[2], salvageRepackSuffix)
		},
		"list the new": func(a []string) bool {
			return len(a) > 2 && a[1] == "list-heads" && strings.HasSuffix(a[2], salvageRepackSuffix)
		},
		"locate the store": func(a []string) bool { return a[0] == "rev-parse" && len(a) > 1 && a[1] == "--path-format=absolute" },
	}
	for name, match := range steps {
		t.Run(name, func(t *testing.T) {
			e := newSalvageRepackEnv(t)
			tip := e.commitOn("lane/step", "step.txt")
			e.fullLeaf("cycle-aaaa-81", "lane/step", tip)
			items := e.plan()
			before := treeDigest(t, e.leafDir("cycle-aaaa-81"))
			e.failing(match)

			freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{items["cycle-aaaa-81"]})

			if err == nil || freed != 0 {
				t.Fatalf("freed = %d, err = %v, want a failure", freed, err)
			}
			assertSameDigest(t, before, treeDigest(t, e.leafDir("cycle-aaaa-81")))
		})
	}
}

func TestSalvageRepack_ANewBundleWithOtherHeadsIsRefused(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/heads", "heads.txt")
	e.fullLeaf("cycle-aaaa-91", "lane/heads", tip)
	items := e.plan()
	before := treeDigest(t, e.leafDir("cycle-aaaa-91"))
	e.rewriting(func(a []string) bool {
		return len(a) > 2 && a[1] == "list-heads" && strings.HasSuffix(a[2], salvageRepackSuffix)
	}, strings.Repeat("0", 40)+" refs/heads/lane/heads\n")

	_, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{items["cycle-aaaa-91"]})

	if !errors.Is(err, errSalvageHeads) {
		t.Fatalf("err = %v, want a heads mismatch", err)
	}
	assertSameDigest(t, before, treeDigest(t, e.leafDir("cycle-aaaa-91")))
}

func TestSalvageRepack_ARepackThatCannotBuildNamesTheFailedStepAndKeepsTheBundle(t *testing.T) {
	steps := map[string]func(t *testing.T, e *salvageRepackEnv){
		"no scratch dir": func(t *testing.T, e *salvageRepackEnv) {
			t.Setenv("TMPDIR", filepath.Join(e.opts.EvolveDir, "missing"))
		},
		"no object store link": func(t *testing.T, e *salvageRepackEnv) {
			e.rewriting(func(a []string) bool { return a[0] == "init" }, "")
		},
		"no basis ref": func(t *testing.T, e *salvageRepackEnv) {
			e.failing(func(a []string) bool { return a[0] == "update-ref" && a[1] == salvageOriginMainRef })
		},
		"tmp bundle vanished": func(t *testing.T, e *salvageRepackEnv) {
			inner := e.opts.Exec
			e.opts.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
				code, err := inner(ctx, name, dir, args, env, in, out, errw)
				if len(args) > 2 && args[1] == "list-heads" && strings.HasSuffix(args[2], salvageRepackSuffix) {
					_ = os.Remove(args[2])
				}
				return code, err
			}
		},
	}
	for name, arm := range steps {
		t.Run(name, func(t *testing.T) {
			e := newSalvageRepackEnv(t)
			tip := e.commitOn("lane/build", "build.txt")
			e.fullLeaf("cycle-aaaa-120", "lane/build", tip)
			item := e.planned("cycle-aaaa-120")
			before := treeDigest(t, e.leafDir("cycle-aaaa-120"))
			arm(t, e)

			freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{item})

			if err == nil || freed != 0 {
				t.Fatalf("freed = %d, err = %v, want a failure", freed, err)
			}
			assertSameDigest(t, before, treeDigest(t, e.leafDir("cycle-aaaa-120")))
		})
	}
}

func TestSalvageRepack_ABundleWhoseHeadsCannotBeListedKeepsTheLeaf(t *testing.T) {
	cases := map[string]struct {
		arm  func(e *salvageRepackEnv)
		want string
	}{
		"list-heads fails": {func(e *salvageRepackEnv) {
			e.failing(func(a []string) bool { return len(a) > 1 && a[1] == "list-heads" })
		}, "does not verify"},
		"list-heads is empty": {func(e *salvageRepackEnv) {
			e.rewriting(func(a []string) bool { return len(a) > 1 && a[1] == "list-heads" }, "\n")
		}, "no readable head"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newSalvageRepackEnv(t)
			tip := e.commitOn("lane/lh", "lh.txt")
			e.fullLeaf("cycle-aaaa-121", "lane/lh", tip)
			tc.arm(e)

			if it := e.planned("cycle-aaaa-121"); it.Action != SalvageKeep || !strings.Contains(it.Reason, tc.want) {
				t.Errorf("item = %+v", it)
			}
		})
	}
}

func TestSalvageRepack_ACommitCountThatIsNotANumberKeepsTheBundle(t *testing.T) {
	for name, out := range map[string]string{"text": "many\n", "negative": "-3\n"} {
		t.Run(name, func(t *testing.T) {
			e := newSalvageRepackEnv(t)
			e.landedLeaf("cycle-aaaa-140", "lane/count"+name)
			e.rewriting(func(a []string) bool { return a[0] == "rev-list" }, out)

			it := e.planned("cycle-aaaa-140")

			if it.Action != SalvageKeep {
				t.Errorf("item = %+v, want keep", it)
			}
		})
	}
}
