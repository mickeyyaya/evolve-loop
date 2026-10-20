package explanationdocs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const absentCommit = "0000000000000000000000000000000000000000"

func sealedFixture(t *testing.T) fixture {
	t.Helper()
	f := newFixture(t)
	f.sealRequired(t)
	return f
}

func rebasedFixture(t *testing.T) (fixture, string) {
	t.Helper()
	f := sealedFixture(t)
	newBase := f.commit(t, "peer.txt")
	if err := RebaseBuild(context.Background(), f.binding(), newBase); err != nil {
		t.Fatal(err)
	}
	return f, newBase
}

func (f fixture) commit(t *testing.T, name string) string {
	t.Helper()
	f.write(t, name, name+"\n")
	f.git(t, "add", name)
	f.git(t, "commit", "-q", "-m", name)
	return f.git(t, "rev-parse", "HEAD")
}

func editJSONFile(t *testing.T, path string, edit func(doc map[string]any)) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverRebaseSplit_RefusesEverySplitItCannotProve(t *testing.T) {
	cases := map[string]struct {
		setup   func(t *testing.T) CycleBinding
		wantErr string
	}{
		"no active contract": {setup: func(t *testing.T) CycleBinding {
			b := newFixture(t).binding()
			b.ContractVersion = 0
			return b
		}},
		"no activation marker": {setup: func(t *testing.T) CycleBinding { return newFixture(t).binding() }},
		"an unreadable activation marker": {setup: func(t *testing.T) CycleBinding {
			f := newFixture(t)
			if err := os.MkdirAll(filepath.Dir(activationPath(f.root, f.cycle)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(activationPath(f.root, f.cycle), []byte("{"), 0o644); err != nil {
				t.Fatal(err)
			}
			return f.binding()
		}, wantErr: "parse activation marker"},
		"another run": {setup: func(t *testing.T) CycleBinding {
			f, _ := rebasedFixture(t)
			b := f.binding()
			b.RunID = "run-other"
			return b
		}},
		"another workspace": {setup: func(t *testing.T) CycleBinding {
			f, _ := rebasedFixture(t)
			b := f.binding()
			b.Workspace = filepath.Join(f.root, ".evolve", "runs", "cycle-43")
			return b
		}},
		"a marker of another contract version": {setup: func(t *testing.T) CycleBinding {
			f, _ := rebasedFixture(t)
			editJSONFile(t, activationPath(f.root, f.cycle), func(doc map[string]any) { doc["contract_version"] = CurrentContractVersion + 1 })
			return f.binding()
		}},
		"a marker of another cycle": {setup: func(t *testing.T) CycleBinding {
			f, _ := rebasedFixture(t)
			editJSONFile(t, activationPath(f.root, f.cycle), func(doc map[string]any) { doc["cycle"] = f.cycle + 1 })
			return f.binding()
		}},
		"another worktree": {setup: func(t *testing.T) CycleBinding {
			f, _ := rebasedFixture(t)
			b := f.binding()
			b.Worktree = t.TempDir()
			return b
		}},
		"a marker base that is not a commit": {setup: func(t *testing.T) CycleBinding {
			f := sealedFixture(t)
			editJSONFile(t, activationPath(f.root, f.cycle), func(doc map[string]any) { doc["base_sha"] = absentCommit })
			return f.binding()
		}, wantErr: "base SHA is not a commit"},
		"no sealed result": {setup: func(t *testing.T) CycleBinding {
			f := sealedFixture(t)
			if err := os.Remove(resultSnapshotPath(f.root, f.cycle)); err != nil {
				t.Fatal(err)
			}
			return f.binding()
		}},
		"no split": {setup: func(t *testing.T) CycleBinding { return sealedFixture(t).binding() }},
		"a sealed base that is not a commit": {setup: func(t *testing.T) CycleBinding {
			f, newBase := rebasedFixture(t)
			editJSONFile(t, resultSnapshotPath(f.root, f.cycle), func(doc map[string]any) {
				doc["view"].(map[string]any)["base_sha"] = absentCommit
			})
			b := f.binding()
			b.BaseSHA = newBase
			return b
		}},
		"a base neither side holds": {setup: func(t *testing.T) CycleBinding {
			f, _ := rebasedFixture(t)
			b := f.binding()
			b.BaseSHA = f.commit(t, "third.txt")
			return b
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			base, recoverable, err := RecoverRebaseSplit(context.Background(), tc.setup(t))
			if base != "" || recoverable {
				t.Errorf("RecoverRebaseSplit = %q, %v; want no recovery", base, recoverable)
			}
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Errorf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
