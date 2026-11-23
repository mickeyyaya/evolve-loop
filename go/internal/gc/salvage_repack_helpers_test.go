package gc

import (
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type salvageRepackEnv struct {
	*branchSweepEnv
}

func newSalvageRepackEnv(t *testing.T) *salvageRepackEnv {
	t.Helper()
	e := &salvageRepackEnv{newThinBundleEnv(t)}
	exclude := filepath.Join(e.repo.Dir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exclude, []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *salvageRepackEnv) leafDir(leaf string) string {
	return filepath.Join(OperatorSalvageDir(e.opts.EvolveDir), leaf)
}

func (e *salvageRepackEnv) bundlePath(leaf string) string {
	return filepath.Join(e.leafDir(leaf), salvageBundleFile)
}

func (e *salvageRepackEnv) fullLeaf(leaf, branch, tip string) {
	e.t.Helper()
	e.writeLeaf(leaf, branch, tip, "refs/heads/"+branch)
}

func (e *salvageRepackEnv) thinLeaf(leaf, branch, tip string) {
	e.t.Helper()
	e.writeLeaf(leaf, branch, tip, "refs/heads/"+branch, "^refs/remotes/origin/main")
}

func (e *salvageRepackEnv) writeLeaf(leaf, branch, tip string, revs ...string) {
	e.t.Helper()
	dir := e.leafDir(leaf)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.t.Fatal(err)
	}
	e.repo.Git(append([]string{"bundle", "create", e.bundlePath(leaf)}, revs...)...)
	if err := os.WriteFile(filepath.Join(dir, salvageHeadFile), []byte(tip+" "+branch+"\n"), 0o644); err != nil {
		e.t.Fatal(err)
	}
	e.age(leaf)
}

func (e *salvageRepackEnv) age(leaf string) {
	e.t.Helper()
	old := time.Now().Add(-3 * time.Hour)
	dir := e.leafDir(leaf)
	for _, p := range []string{filepath.Join(dir, salvageBundleFile), filepath.Join(dir, salvageHeadFile), dir} {
		if err := os.Chtimes(p, old, old); err != nil && !os.IsNotExist(err) {
			e.t.Fatal(err)
		}
	}
}

func (e *salvageRepackEnv) plan() map[string]SalvageRepackItem {
	e.t.Helper()
	items, err := PlanSalvageRepack(e.opts)
	if err != nil {
		e.t.Fatalf("PlanSalvageRepack: %v", err)
	}
	byLeaf := map[string]SalvageRepackItem{}
	for _, it := range items {
		byLeaf[it.Leaf] = it
	}
	return byLeaf
}

func (e *salvageRepackEnv) heads(leaf string) string {
	e.t.Helper()
	return e.repo.Git("bundle", "list-heads", e.bundlePath(leaf))
}

func (e *salvageRepackEnv) refs() string {
	e.t.Helper()
	return e.repo.Git("for-each-ref", "--format=%(refname) %(objectname)")
}

func (e *salvageRepackEnv) audit() []SalvageRepackRecord {
	e.t.Helper()
	raw, err := os.ReadFile(SalvageRepackAuditPath(e.opts.EvolveDir))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		e.t.Fatal(err)
	}
	var rows []SalvageRepackRecord
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var row SalvageRepackRecord
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			e.t.Fatalf("audit line %q: %v", line, err)
		}
		rows = append(rows, row)
	}
	return rows
}

func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	digest := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		sum := ""
		if d.Type().IsRegular() {
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			h := sha256.Sum256(raw)
			sum = string(h[:])
		}
		digest[p] = sum + "|" + info.ModTime().String() + "|" + info.Mode().String()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func assertSameDigest(t *testing.T, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("entries changed: %d before, %d after", len(before), len(after))
	}
	for p, d := range before {
		if after[p] != d {
			t.Errorf("%s changed", p)
		}
	}
}

func sortedLines(s string) []string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	slices.Sort(lines)
	return lines
}

func fileSize(t *testing.T, p string) int64 {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func (e *salvageRepackEnv) planned(leaf string) SalvageRepackItem {
	e.t.Helper()
	return e.plan()[leaf]
}

func (e *salvageRepackEnv) landedLeaf(leaf, branch string) SalvageRepackItem {
	e.t.Helper()
	tip := e.land(branch, branch+".txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	e.fullLeaf(leaf, branch, tip)
	return e.planned(leaf)
}

func (e *salvageRepackEnv) lockPath() string {
	return filepath.Join(e.opts.ProjectRoot, ".evolve", "ship.lock")
}

func (e *salvageRepackEnv) observeShipLock() *[]bool {
	e.t.Helper()
	runDir := filepath.Join(e.opts.EvolveDir, "runs", "cycle-130")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "cycle-130", OwnerPID: 1}, time.Now()); err != nil {
		e.t.Fatal(err)
	}
	seen := &[]bool{}
	e.opts.PidAlive = func(int) bool {
		release, held, err := flock.TryLock(e.lockPath())
		if err != nil {
			e.t.Error(err)
		}
		if release != nil {
			release()
		}
		*seen = append(*seen, held)
		return false
	}
	return seen
}
