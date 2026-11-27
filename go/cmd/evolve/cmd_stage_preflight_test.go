package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

func headPolicy(t *testing.T) policy.Policy {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("read the policy of HEAD: %v", err)
	}
	p, err := policy.Parse(raw)
	if err != nil {
		t.Fatalf("parse the policy of HEAD: %v", err)
	}
	return p
}

func requiredFlag(required bool) func() (bool, error) {
	return func() (bool, error) { return required, nil }
}

func TestHeadPolicyAcceptedWithTheMigrationRecords(t *testing.T) {
	t.Parallel()
	p := headPolicy(t)
	effective, err := p.EffectiveStages()
	if err != nil {
		t.Fatal(err)
	}
	if len(stageevidence.EnforcedKeys(effective)) == 0 {
		t.Fatal("the policy of HEAD has no key at enforce, so the fixture proves nothing")
	}
	ld := ledger.New(t.TempDir())
	if _, err := migrateStages(context.Background(), ld, migrateRequest{Effective: effective, Digest: "head"}); err != nil {
		t.Fatal(err)
	}
	for _, required := range []bool{false, true} {
		res, err := stageContractSource(stageContractInput{Effective: p.EffectiveStages, Records: ld, GenesisRequired: requiredFlag(required)})()
		if err != nil || len(res.Findings) != 0 || res.Mode == stageevidence.ModeRefuse || res.Mode == stageevidence.ModeInactive {
			t.Errorf("genesis_required=%v: result %+v err %v, want an active mode and no finding", required, res, err)
		}
	}
}

func TestHeadPolicyAcceptedWithNoGenesisIsInactiveNotARefusal(t *testing.T) {
	t.Parallel()
	ld := ledger.New(t.TempDir())
	res, err := stageContractSource(stageContractInput{Effective: headPolicy(t).EffectiveStages, Records: ld, GenesisRequired: requiredFlag(false)})()
	if err != nil || res.Mode != stageevidence.ModeInactive {
		t.Fatalf("result %+v err %v, want the inactive mode", res, err)
	}
	if len(res.Findings) == 0 {
		t.Error("the inactive mode names no key at enforce")
	}
}

func TestCheckRefusesWhenGenesisRequiredAndAbsent(t *testing.T) {
	t.Parallel()
	res, err := stageContractSource(stageContractInput{Effective: headPolicy(t).EffectiveStages, Records: ledger.New(t.TempDir()), GenesisRequired: requiredFlag(true)})()
	if err != nil || res.Mode != stageevidence.ModeRefuse {
		t.Fatalf("result %+v err %v, want the refuse mode", res, err)
	}
}

type failingRecords struct{ err error }

func (f failingRecords) FlipRecords(context.Context) (stageevidence.FlipSet, error) {
	return stageevidence.FlipSet{}, f.err
}

func TestStageContractSourceReportsEachUnreadableInput(t *testing.T) {
	t.Parallel()
	down := errors.New("down")
	ratchetDown := func() (bool, error) { return false, down }
	cases := map[string]stageContractInput{
		"ratchet":   {Effective: nil, Records: failingRecords{}, GenesisRequired: ratchetDown},
		"records":   {Effective: func() (map[string]string, error) { return nil, nil }, Records: failingRecords{err: down}, GenesisRequired: requiredFlag(false)},
		"effective": {Effective: func() (map[string]string, error) { return nil, down }, Records: failingRecords{}, GenesisRequired: requiredFlag(false)},
	}
	for name, in := range cases {
		if _, err := stageContractSource(in)(); !errors.Is(err, down) {
			t.Errorf("%s: err = %v, want %v", name, err, down)
		}
	}
}

func TestLoopPreflightOptionsWireTheStageContractCheck(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, migratePolicy)
	opts := loopPreflightOptions(loopConfig{ProjectRoot: root, EvolveDir: paths.EvolveDirOf(root)}, &bytes.Buffer{})
	if opts.StageContract == nil {
		t.Fatal("loopPreflightOptions does not wire the stage contract check")
	}
	res, err := opts.StageContract()
	if err != nil || res.Mode != stageevidence.ModeInactive || len(res.Findings) == 0 {
		t.Errorf("result %+v err %v, want the inactive mode with the keys at enforce", res, err)
	}
}
