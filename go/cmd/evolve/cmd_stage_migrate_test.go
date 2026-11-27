package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const migratePolicy = `{"held_proposals":{"stage":"enforce"},"workflow":{"retry_ladder":{"stage":"enforce"}}}`

var errStoreDown = errors.New("store down")

type fakeMigrateStore struct {
	set         stageevidence.FlipSet
	readErr     error
	failFlipAt  int
	failGenesis bool
	flips       int
	writes      []string
}

func (f *fakeMigrateStore) FlipRecords(context.Context) (stageevidence.FlipSet, error) {
	return f.set, f.readErr
}

func (f *fakeMigrateStore) AppendStageFlip(_ context.Context, flip stageevidence.Flip) error {
	f.flips++
	if f.failFlipAt > 0 && f.flips == f.failFlipAt {
		return errStoreDown
	}
	f.set.Flips = append(f.set.Flips, flip)
	f.writes = append(f.writes, "flip:"+flip.Key)
	return nil
}

func (f *fakeMigrateStore) AppendStageGenesis(_ context.Context, g stageevidence.Genesis) error {
	if f.failGenesis {
		return errStoreDown
	}
	f.set.Genesis = &g
	f.writes = append(f.writes, "genesis")
	return nil
}

func migrateEffective() map[string]string {
	return map[string]string{"a.stage": "enforce", "b.stage": "shadow", "c.stage": "enforce", "d.stage": "off"}
}

func migrateReq() migrateRequest {
	return migrateRequest{Effective: migrateEffective(), Digest: "dig"}
}

func TestMigrateWritesOneRecordPerEnforcedKeyAndOneGenesis(t *testing.T) {
	t.Parallel()
	store := &fakeMigrateStore{}
	res, err := migrateStages(context.Background(), store, migrateReq())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"flip:a.stage", "flip:c.stage", "genesis"}; !slices.Equal(store.writes, want) {
		t.Fatalf("writes = %q, want %q", store.writes, want)
	}
	g := store.set.Genesis
	if g == nil || !slices.Equal(g.Keys, []string{"a.stage", "c.stage"}) || g.Count != 2 || g.PolicyDigest != "dig" || res.Genesis == nil {
		t.Fatalf("genesis = %+v", g)
	}
	for _, f := range store.set.Flips {
		if f.Origin != stageevidence.OriginGrandfather || f.Value != "enforce" || f.PolicyDigest != "dig" || f.V != 1 || f.Reason == "" {
			t.Errorf("flip = %+v", f)
		}
	}
}

func TestMigrateSecondRunWritesNothing(t *testing.T) {
	t.Parallel()
	store := &fakeMigrateStore{}
	if _, err := migrateStages(context.Background(), store, migrateReq()); err != nil {
		t.Fatal(err)
	}
	before := len(store.writes)
	res, err := migrateStages(context.Background(), store, migrateReq())
	if err != nil || res.Existing == nil || len(store.writes) != before {
		t.Fatalf("second run: res %+v err %v writes %d -> %d", res, err, before, len(store.writes))
	}
}

func TestMigrateCrashBeforeGenesisIsFinishedWithoutDuplicates(t *testing.T) {
	t.Parallel()
	store := &fakeMigrateStore{failFlipAt: 2}
	if _, err := migrateStages(context.Background(), store, migrateReq()); !errors.Is(err, errStoreDown) {
		t.Fatalf("crash err = %v", err)
	}
	if store.set.Genesis != nil || len(store.set.Flips) != 1 {
		t.Fatalf("after the crash: %+v", store.set)
	}
	store.failFlipAt = 0
	if _, err := migrateStages(context.Background(), store, migrateReq()); err != nil {
		t.Fatal(err)
	}
	if len(store.set.Flips) != 2 || store.set.Genesis == nil {
		t.Fatalf("after the retry: %+v", store.set)
	}
	seen := map[string]int{}
	for _, f := range store.set.Flips {
		seen[f.Key]++
	}
	if seen["a.stage"] != 1 || seen["c.stage"] != 1 {
		t.Errorf("flip records per key = %v, want one each", seen)
	}
}

func TestMigrateGenesisFailureLeavesTheFlipRecordsForTheNextRun(t *testing.T) {
	t.Parallel()
	store := &fakeMigrateStore{failGenesis: true}
	if _, err := migrateStages(context.Background(), store, migrateReq()); !errors.Is(err, errStoreDown) {
		t.Fatalf("err = %v", err)
	}
	if len(store.set.Flips) != 2 || store.set.Genesis != nil {
		t.Fatalf("state = %+v", store.set)
	}
}

func TestMigrateRefusesWhenGenesisRequired(t *testing.T) {
	t.Parallel()
	store := &fakeMigrateStore{}
	req := migrateReq()
	req.GenesisRequired = true
	if _, err := migrateStages(context.Background(), store, req); !errors.Is(err, stageevidence.ErrGenesisRequired) {
		t.Fatalf("err = %v, want ErrGenesisRequired", err)
	}
	if len(store.writes) != 0 {
		t.Errorf("writes = %q, want none", store.writes)
	}
}

func TestMigratePrintsAnExistingGenesisEvenWhenGenesisRequired(t *testing.T) {
	t.Parallel()
	store := &fakeMigrateStore{set: stageevidence.FlipSet{Genesis: &stageevidence.Genesis{V: 1, Keys: []string{"a.stage"}, Count: 1}}}
	req := migrateReq()
	req.GenesisRequired = true
	res, err := migrateStages(context.Background(), store, req)
	if err != nil || res.Existing == nil || len(store.writes) != 0 {
		t.Fatalf("res %+v err %v writes %q", res, err, store.writes)
	}
}

func TestMigrateRefusesUnresolvableKey(t *testing.T) {
	t.Parallel()
	store := &fakeMigrateStore{}
	req := migrateReq()
	req.Effective["z.stage"] = ""
	_, err := migrateStages(context.Background(), store, req)
	var unresolved *unresolvedKeyError
	if !errors.Is(err, stageevidence.ErrUnresolvedStageKey) || !errors.As(err, &unresolved) || unresolved.Key != "z.stage" {
		t.Fatalf("err = %v, want ErrUnresolvedStageKey naming z.stage", err)
	}
	if len(store.writes) != 0 {
		t.Errorf("writes = %q, want none", store.writes)
	}
}

func TestMigrateDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	store := &fakeMigrateStore{}
	req := migrateReq()
	req.DryRun = true
	res, err := migrateStages(context.Background(), store, req)
	if err != nil || !slices.Equal(res.Planned, []string{"a.stage", "c.stage"}) || res.Genesis == nil || len(store.writes) != 0 {
		t.Fatalf("res %+v err %v writes %q", res, err, store.writes)
	}
}

func TestMigrateReportsAnUnreadableChain(t *testing.T) {
	t.Parallel()
	store := &fakeMigrateStore{readErr: stageevidence.ErrLedgerBroken}
	if _, err := migrateStages(context.Background(), store, migrateReq()); !errors.Is(err, stageevidence.ErrLedgerBroken) {
		t.Fatalf("err = %v", err)
	}
}

func partialMigration() *fakeMigrateStore {
	store := &fakeMigrateStore{}
	for _, key := range []string{"c.stage", "a.stage"} {
		store.set.Flips = append(store.set.Flips, stageevidence.Flip{V: 1, Key: key, Value: "enforce", Origin: stageevidence.OriginGrandfather})
	}
	return store
}

func TestRestoreGenesisFinishesIncompleteMigrationWithoutNewKey(t *testing.T) {
	t.Parallel()
	store := partialMigration()
	req := migrateReq()
	req.Effective["new.stage"] = "enforce"
	req.Restore, req.Reason, req.GenesisRequired = true, "crash at 2026-11-27", true
	res, err := migrateStages(context.Background(), store, req)
	if err != nil || res.Genesis == nil {
		t.Fatalf("res %+v err %v", res, err)
	}
	g := store.set.Genesis
	if !slices.Equal(g.Keys, []string{"a.stage", "c.stage"}) || g.Reason != "crash at 2026-11-27" {
		t.Errorf("genesis = %+v, want the two grandfathered keys and not new.stage", g)
	}
	if slices.Contains(store.writes, "flip:new.stage") || len(store.writes) != 1 {
		t.Errorf("writes = %q, want only the genesis", store.writes)
	}
}

func TestRestoreGenesisNeedsAReasonAndRecords(t *testing.T) {
	t.Parallel()
	req := migrateReq()
	req.Restore = true
	if _, err := migrateStages(context.Background(), partialMigration(), req); !errors.Is(err, errRestoreReason) {
		t.Errorf("no reason: err = %v", err)
	}
	req.Reason = "why"
	if _, err := migrateStages(context.Background(), &fakeMigrateStore{}, req); !errors.Is(err, errNothingToRestore) {
		t.Errorf("no records: err = %v", err)
	}
	req.DryRun = true
	res, err := migrateStages(context.Background(), partialMigration(), req)
	if err != nil || res.Genesis == nil || len(res.Planned) != 0 {
		t.Errorf("dry restore: res %+v err %v", res, err)
	}
}

func migrateEnv(t *testing.T) stageMigrateEnv {
	t.Helper()
	return stageMigrateEnv{
		genesisRequired: func() (bool, error) { return false, nil },
		lock:            flock.Lock,
		effective:       policy.Policy.EffectiveStages,
	}
}

func runMigrate(t *testing.T, env stageMigrateEnv, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := stageMigrateCmd{env: env, stdout: &out, stderr: &errOut}.run(args)
	return out.String(), errOut.String(), code
}

func TestStageMigrateVerbWritesThenReadsBackFromTheLedger(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, migratePolicy)
	out, errOut, code := runStageVerb(t, "migrate", "--project-root", root)
	if code != cliExitOK {
		t.Fatalf("exit %d, stderr: %s", code, errOut)
	}
	p, err := policy.Parse([]byte(migratePolicy))
	if err != nil {
		t.Fatal(err)
	}
	effective, err := p.EffectiveStages()
	if err != nil {
		t.Fatal(err)
	}
	want := stageevidence.EnforcedKeys(effective)
	set, err := ledger.New(filepath.Join(root, ".evolve")).FlipRecords(context.Background())
	if err != nil || len(set.Flips) != len(want) || set.Genesis == nil || !slices.Equal(set.Genesis.Keys, want) {
		t.Fatalf("ledger records = %+v err %v, want keys %q", set, err, want)
	}
	if !strings.Contains(out, "wrote") {
		t.Errorf("stdout = %q", out)
	}
	again, _, code := runStageVerb(t, "migrate", "--project-root", root)
	if code != cliExitOK || !strings.Contains(again, "genesis present") {
		t.Errorf("second run: exit %d stdout %q", code, again)
	}
	after, _ := ledger.New(filepath.Join(root, ".evolve")).FlipRecords(context.Background())
	if len(after.Flips) != len(set.Flips) {
		t.Errorf("second run wrote %d extra records", len(after.Flips)-len(set.Flips))
	}
}

func TestStageMigrateVerbDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, migratePolicy)
	out, errOut, code := runStageVerb(t, "migrate", "--dry-run", "--project-root", root)
	if code != cliExitOK || !strings.Contains(out, "dry run") {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "ledger.jsonl")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the dry run created the ledger: %v", err)
	}
}

func TestStageMigrateVerbRestoreGenesisThroughTheLedger(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, migratePolicy)
	ld := ledger.New(filepath.Join(root, ".evolve"))
	flip := stageevidence.Flip{V: 1, Key: "held_proposals.stage", Value: "enforce", Origin: stageevidence.OriginGrandfather}
	if err := ld.AppendStageFlip(context.Background(), flip); err != nil {
		t.Fatal(err)
	}
	if _, _, code := runStageVerb(t, "migrate", "--restore-genesis", "--project-root", root); code != cliExitUsage {
		t.Errorf("restore without a reason exited %d, want %d", code, cliExitUsage)
	}
	out, errOut, code := runStageVerb(t, "migrate", "--restore-genesis", "--reason", "crash", "--project-root", root)
	if code != cliExitOK {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, errOut)
	}
	set, _ := ld.FlipRecords(context.Background())
	if set.Genesis == nil || !slices.Equal(set.Genesis.Keys, []string{"held_proposals.stage"}) {
		t.Errorf("genesis = %+v", set.Genesis)
	}
}

func TestStageMigrateVerbRefusesWhenGenesisRequiredAndAbsent(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, migratePolicy)
	env := migrateEnv(t)
	env.genesisRequired = func() (bool, error) { return true, nil }
	_, errOut, code := runMigrate(t, env, "--project-root", root)
	if code != cliExitRefused || !strings.Contains(errOut, "D18") {
		t.Errorf("exit %d stderr %q, want refused naming D18", code, errOut)
	}
}

func TestStageMigrateVerbReportsEachFailure(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, migratePolicy)
	broken := stageProjectRoot(t, `{"held_proposals":`)
	missing := t.TempDir()
	noLedger := stageProjectRoot(t, migratePolicy)
	if err := os.Mkdir(filepath.Join(noLedger, ".evolve", "ledger.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		env  func(*stageMigrateEnv)
		args []string
		want int
	}{
		"bad flag":           {nil, []string{"--nope"}, cliExitUsage},
		"extra argument":     {nil, []string{"extra", "--project-root", root}, cliExitUsage},
		"no policy":          {nil, []string{"--project-root", missing}, cliExitIO},
		"unparsable policy":  {nil, []string{"--project-root", broken}, cliExitIO},
		"ratchet unreadable": {func(e *stageMigrateEnv) { e.genesisRequired = func() (bool, error) { return false, errStoreDown } }, []string{"--project-root", root}, cliExitIO},
		"unresolvable key": {func(e *stageMigrateEnv) {
			e.effective = func(policy.Policy) (map[string]string, error) { return nil, errStoreDown }
		}, []string{"--project-root", root}, cliExitFailed},
		"lock refused":            {func(e *stageMigrateEnv) { e.lock = func(string) (func(), error) { return nil, errStoreDown } }, []string{"--project-root", root}, cliExitIO},
		"ledger unreadable":       {nil, []string{"--project-root", noLedger}, cliExitIO},
		"reason without restore":  {nil, []string{"--reason", "x", "--project-root", root}, cliExitUsage},
		"restore with no records": {nil, []string{"--restore-genesis", "--reason", "x", "--project-root", stageProjectRoot(t, migratePolicy)}, cliExitFailed},
	}
	for name, c := range cases {
		env := migrateEnv(t)
		if c.env != nil {
			c.env(&env)
		}
		if _, errOut, code := runMigrate(t, env, c.args...); code != c.want {
			t.Errorf("%s: exit %d, want %d (stderr %q)", name, code, c.want, errOut)
		}
	}
}

func TestStageMigrateVerbReleasesItsLock(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, migratePolicy)
	if _, errOut, code := runStageVerb(t, "migrate", "--project-root", root); code != cliExitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	release, err := flock.Lock(filepath.Join(root, ".evolve", stageMigrateLockName))
	if err != nil {
		t.Fatalf("the lock was still held: %v", err)
	}
	release()
}

func TestRunStageUsageNamesMigrate(t *testing.T) {
	t.Parallel()
	_, errOut, code := runStageVerb(t)
	if code != cliExitUsage || !strings.Contains(errOut, "migrate") {
		t.Errorf("exit %d stderr %q", code, errOut)
	}
}

func TestDefaultStageMigrateEnvReadsTheEmbeddedRatchet(t *testing.T) {
	t.Parallel()
	env := defaultStageMigrateEnv()
	required, err := env.genesisRequired()
	if err != nil || required {
		t.Errorf("genesisRequired = %v, %v; the ratchet starts at false", required, err)
	}
	if env.lock == nil || env.effective == nil {
		t.Error("the default env has no lock")
	}
}
