package stageevidence_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

func grandfather(key string) stageevidence.Flip {
	return stageevidence.Flip{V: 1, Key: key, Value: "enforce", Origin: stageevidence.OriginGrandfather, PolicyDigest: "d", Reason: "enforced before the contract"}
}

func genesisOf(keys ...string) *stageevidence.Genesis {
	return &stageevidence.Genesis{V: 1, Keys: keys, PolicyDigest: "d", Count: len(keys)}
}

func TestCheck_ReportsEachOutcomeOfTheActivationTable(t *testing.T) {
	t.Parallel()
	effective := map[string]string{"a.stage": "enforce", "b.stage": "shadow", "c.stage": "enforce", "d.stage": "off"}
	cases := map[string]struct {
		in       stageevidence.CheckInput
		mode     stageevidence.Mode
		findings []string
	}{
		"absent genesis and not required is inactive and names each unrecorded enforced key": {
			in:       stageevidence.CheckInput{Effective: effective},
			mode:     stageevidence.ModeInactive,
			findings: []string{"a.stage", "c.stage"},
		},
		"present genesis and not required is report-only": {
			in: stageevidence.CheckInput{Effective: effective, GenesisRequired: false, Records: stageevidence.FlipSet{
				Flips: []stageevidence.Flip{grandfather("a.stage")}, Genesis: genesisOf("a.stage", "c.stage")}},
			mode:     stageevidence.ModeReportOnly,
			findings: []string{"c.stage"},
		},
		"present genesis and required is active": {
			in: stageevidence.CheckInput{Effective: effective, GenesisRequired: true, Records: stageevidence.FlipSet{
				Flips: []stageevidence.Flip{grandfather("a.stage"), grandfather("c.stage")}, Genesis: genesisOf("a.stage", "c.stage")}},
			mode: stageevidence.ModeActive,
		},
		"absent genesis and required refuses": {
			in:       stageevidence.CheckInput{Effective: effective, GenesisRequired: true},
			mode:     stageevidence.ModeRefuse,
			findings: []string{stageevidence.GenesisFindingKey, "a.stage", "c.stage"},
		},
	}
	for name, c := range cases {
		got := stageevidence.Check(c.in)
		var keys []string
		for _, f := range got.Findings {
			keys = append(keys, f.Key)
		}
		if got.Mode != c.mode || !slices.Equal(keys, c.findings) {
			t.Errorf("%s: mode %q findings %q, want %q %q", name, got.Mode, keys, c.mode, c.findings)
		}
	}
}

func TestGrandfatherRecordOutsideGenesisIsNotAccepted(t *testing.T) {
	t.Parallel()
	in := stageevidence.CheckInput{
		Effective: map[string]string{"a.stage": "enforce", "late.stage": "enforce"},
		Records: stageevidence.FlipSet{
			Flips:   []stageevidence.Flip{grandfather("a.stage"), grandfather("late.stage")},
			Genesis: genesisOf("a.stage"),
		},
	}
	got := stageevidence.Check(in)
	if len(got.Findings) != 1 || got.Findings[0].Key != "late.stage" {
		t.Fatalf("findings = %+v, want one for late.stage", got.Findings)
	}
}

func TestCheck_AcceptsAVerifiedRecordOutsideGenesisAndRejectsAWrongValue(t *testing.T) {
	t.Parallel()
	verified := stageevidence.Flip{V: 1, Key: "late.stage", Value: "enforce", Origin: stageevidence.OriginVerified}
	shadowOnly := stageevidence.Flip{V: 1, Key: "x.stage", Value: "shadow", Origin: stageevidence.OriginGrandfather}
	in := stageevidence.CheckInput{
		Effective: map[string]string{"late.stage": "enforce", "x.stage": "enforce"},
		Records:   stageevidence.FlipSet{Flips: []stageevidence.Flip{verified, shadowOnly}, Genesis: genesisOf("x.stage")},
	}
	got := stageevidence.Check(in)
	if len(got.Findings) != 1 || got.Findings[0].Key != "x.stage" {
		t.Fatalf("findings = %+v, want one for x.stage", got.Findings)
	}
}

func TestEnforcedKeys_ReturnsTheSortedKeysAtEnforce(t *testing.T) {
	t.Parallel()
	got := stageevidence.EnforcedKeys(map[string]string{"z": "enforce", "a": "enforce", "m": "shadow", "o": "off"})
	if !slices.Equal(got, []string{"a", "z"}) {
		t.Errorf("EnforcedKeys = %q", got)
	}
}

func TestFlipAndGenesisRoundTripThroughTheirMessage(t *testing.T) {
	t.Parallel()
	flip := grandfather("a.stage")
	back, err := stageevidence.ParseFlip(flip.Encode())
	if err != nil || back != flip {
		t.Errorf("flip round trip = %+v, %v", back, err)
	}
	genesis := genesisOf("a.stage", "b.stage")
	got, err := stageevidence.ParseGenesis(genesis.Encode())
	if err != nil || !slices.Equal(got.Keys, genesis.Keys) || got.Count != 2 || got.PolicyDigest != "d" {
		t.Errorf("genesis round trip = %+v, %v", got, err)
	}
}

func TestParseFlipAndGenesis_RefuseMalformedRecords(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		parse func() error
	}{
		"flip not json":             {func() error { _, err := stageevidence.ParseFlip("{"); return err }},
		"flip wrong version":        {func() error { _, err := stageevidence.ParseFlip(`{"v":2,"key":"a","value":"enforce"}`); return err }},
		"flip without key":          {func() error { _, err := stageevidence.ParseFlip(`{"v":1,"value":"enforce"}`); return err }},
		"genesis not json":          {func() error { _, err := stageevidence.ParseGenesis("{"); return err }},
		"genesis wrong version":     {func() error { _, err := stageevidence.ParseGenesis(`{"v":2,"keys":[],"count":0}`); return err }},
		"genesis count disagrees":   {func() error { _, err := stageevidence.ParseGenesis(`{"v":1,"keys":["a"],"count":2}`); return err }},
		"genesis duplicate keys":    {func() error { _, err := stageevidence.ParseGenesis(`{"v":1,"keys":["a","a"],"count":2}`); return err }},
		"genesis unsorted keys ok?": {func() error { _, err := stageevidence.ParseGenesis(`{"v":1,"keys":["b","a"],"count":2}`); return err }},
	}
	for name, c := range cases {
		err := c.parse()
		if name == "genesis unsorted keys ok?" {
			if err != nil {
				t.Errorf("%s: %v", name, err)
			}
			continue
		}
		if !errors.Is(err, stageevidence.ErrMalformedFlipRecord) {
			t.Errorf("%s: err = %v, want ErrMalformedFlipRecord", name, err)
		}
	}
}

func TestFlipErrorsAreTyped(t *testing.T) {
	t.Parallel()
	if errors.Is(stageevidence.ErrGenesisRequired, stageevidence.ErrUnresolvedStageKey) {
		t.Error("the two flip errors must differ")
	}
}

type recordsFake struct{ set stageevidence.FlipSet }

func (r recordsFake) FlipRecords(context.Context) (stageevidence.FlipSet, error) { return r.set, nil }

func TestAPICover_NamesTheFlipRecordExports(t *testing.T) {
	t.Parallel()
	var port stageevidence.FlipRecords = recordsFake{set: stageevidence.FlipSet{Flips: []stageevidence.Flip{grandfather("a.stage")}}}
	set, err := port.FlipRecords(context.Background())
	if err != nil || len(set.Flips) != 1 {
		t.Fatalf("set %+v err %v", set, err)
	}
	flip := set.Flips[0]
	if flip.V != stageevidence.FlipVersion {
		t.Errorf("record version %d, want %d", flip.V, stageevidence.FlipVersion)
	}
	if stageevidence.FlipKind != "stage-flip" || stageevidence.GenesisKind != "stage-genesis" {
		t.Errorf("record kinds = %q %q", stageevidence.FlipKind, stageevidence.GenesisKind)
	}
	var result stageevidence.CheckResult = stageevidence.Check(stageevidence.CheckInput{Effective: map[string]string{"a.stage": "enforce"}})
	var finding stageevidence.Finding = result.Findings[0]
	if finding.Key != "a.stage" || finding.Reason == "" {
		t.Errorf("finding = %+v", finding)
	}
}
