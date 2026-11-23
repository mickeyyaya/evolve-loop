package gc

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type remoteThinEnv struct {
	origin *gittest.Repo
	clone  *gittest.Repo
	opts   RemoteSweepOptions
	api    *apiRS
}

func newRemoteThinEnv(t *testing.T, land bool) (*remoteThinEnv, string) {
	t.Helper()
	origin := gittest.Fixture(t)
	commitRepoFile(t, origin, "seed.txt", "seed\n")
	bulk := make([]byte, 3<<20)
	if _, err := rand.Read(bulk); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(origin.Dir, "bulk.bin"), bulk, 0o644); err != nil {
		t.Fatal(err)
	}
	origin.Git("add", "bulk.bin")
	origin.Git("commit", "-q", "-m", "bulk history")
	origin.Git("checkout", "-q", "-b", "feat/x")
	commitRepoFile(t, origin, "x.txt", "x\n")
	tip := origin.Git("rev-parse", "HEAD")
	origin.Git("checkout", "-q", "main")
	if land {
		origin.Git("merge", "-q", "--no-ff", "-m", "land", "feat/x")
	}
	clone := gittest.Clone(t, origin.Dir)
	api := &apiRS{}
	var log bytes.Buffer
	return &remoteThinEnv{origin: origin, clone: clone, api: api, opts: RemoteSweepOptions{
		ProjectRoot: clone.Dir, EvolveDir: filepath.Join(clone.Dir, ".evolve"), Remote: "origin",
		Stage: RemoteStageEnforce, Exec: sysexec.DefaultRunner, API: api, Log: &log,
	}}, tip
}

func (e *remoteThinEnv) sweep(t *testing.T, tip string) {
	t.Helper()
	m := RemoteManifest{Items: []RemoteItem{{Branch: "feat/x", Head: tip, Action: RemoteActionDelete, Reason: "landed"}}}
	if err := ApplyRemoteSweep(context.Background(), e.opts, m); err != nil {
		t.Fatalf("ApplyRemoteSweep: %v", err)
	}
	if len(e.api.deleted) != 1 {
		t.Fatalf("deleted = %v, want feat/x", e.api.deleted)
	}
}

func (e *remoteThinEnv) leaf(tip string) string {
	return filepath.Join(OperatorSalvageDir(e.opts.EvolveDir), "remote-origin-feat-x-"+tip[:12])
}

func TestRemoteSweepApply_ABranchOneCommitPastOriginMainSavesAThinBundle(t *testing.T) {
	e, tip := newRemoteThinEnv(t, false)

	e.sweep(t, tip)

	bundle := filepath.Join(e.leaf(tip), salvageBundleFile)
	info, err := os.Stat(bundle)
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	if info.Size() >= thinBundleLimit {
		t.Errorf("bundle is %d bytes, want under %d: it holds history that origin/main has", info.Size(), thinBundleLimit)
	}
	e.clone.Git("bundle", "verify", bundle)
}

func TestRemoteSweepApply_ABranchThatOriginMainHoldsSavesTheTipAndNoBundle(t *testing.T) {
	e, tip := newRemoteThinEnv(t, true)

	e.sweep(t, tip)

	if _, err := os.Stat(filepath.Join(e.leaf(tip), salvageBundleFile)); !os.IsNotExist(err) {
		t.Errorf("bundle stat err = %v, want none", err)
	}
	note, err := os.ReadFile(filepath.Join(e.leaf(tip), salvageHeadFile))
	if err != nil || string(note) != tip+" feat/x\n" {
		t.Errorf("note = %q, %v", note, err)
	}
}
