package stageevidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
)

const (
	FlipKind          = "stage-flip"
	GenesisKind       = "stage-genesis"
	OriginGrandfather = "grandfather"
	OriginVerified    = "verified"
	FlipVersion       = 1
	GenesisFindingKey = "genesis"
	valueEnforce      = "enforce"
)

var (
	ErrGenesisRequired     = errors.New("genesis_required is true and no stage-genesis record exists (D18: restore it with evolve stage migrate --restore-genesis)")
	ErrUnresolvedStageKey  = errors.New("stage key has no effective stage")
	ErrMalformedFlipRecord = errors.New("malformed stage flip record")
)

type Flip struct {
	V            int    `json:"v"`
	Key          string `json:"key"`
	Value        string `json:"value"`
	Origin       string `json:"origin"`
	PolicyDigest string `json:"policy_digest"`
	Reason       string `json:"reason"`
}

type Genesis struct {
	V            int      `json:"v"`
	Keys         []string `json:"keys"`
	PolicyDigest string   `json:"policy_digest"`
	Count        int      `json:"count"`
	Reason       string   `json:"reason,omitempty"`
}

type FlipSet struct {
	Flips   []Flip
	Genesis *Genesis
}

type FlipRecords interface {
	FlipRecords(ctx context.Context) (FlipSet, error)
}

type Mode string

const (
	ModeInactive   Mode = "inactive"
	ModeReportOnly Mode = "report-only"
	ModeActive     Mode = "active"
	ModeRefuse     Mode = "refuse"
)

type Finding struct {
	Key    string
	Reason string
}

type CheckInput struct {
	Effective       map[string]string
	Records         FlipSet
	GenesisRequired bool
}

type CheckResult struct {
	Mode     Mode
	Findings []Finding
}

func (f Flip) Encode() string {
	raw, _ := json.Marshal(f)
	return string(raw)
}

func (g Genesis) Encode() string {
	raw, _ := json.Marshal(g)
	return string(raw)
}

func ParseFlip(message string) (Flip, error) {
	var f Flip
	if err := json.Unmarshal([]byte(message), &f); err != nil {
		return Flip{}, fmt.Errorf("%w: %v", ErrMalformedFlipRecord, err)
	}
	if f.V != FlipVersion || f.Key == "" {
		return Flip{}, fmt.Errorf("%w: version %d key %q", ErrMalformedFlipRecord, f.V, f.Key)
	}
	return f, nil
}

func ParseGenesis(message string) (Genesis, error) {
	var g Genesis
	if err := json.Unmarshal([]byte(message), &g); err != nil {
		return Genesis{}, fmt.Errorf("%w: %v", ErrMalformedFlipRecord, err)
	}
	if g.V != FlipVersion || g.Count != len(g.Keys) {
		return Genesis{}, fmt.Errorf("%w: version %d count %d keys %d", ErrMalformedFlipRecord, g.V, g.Count, len(g.Keys))
	}
	sorted := slices.Clone(g.Keys)
	sort.Strings(sorted)
	if len(slices.Compact(sorted)) != len(sorted) {
		return Genesis{}, fmt.Errorf("%w: duplicate key", ErrMalformedFlipRecord)
	}
	return g, nil
}

func EnforcedKeys(effective map[string]string) []string {
	var keys []string
	for key, value := range effective {
		if value == valueEnforce {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func Check(in CheckInput) CheckResult {
	genesis := in.Records.Genesis
	findings := unrecorded(in)
	switch {
	case genesis == nil && in.GenesisRequired:
		return CheckResult{Mode: ModeRefuse, Findings: append([]Finding{{Key: GenesisFindingKey, Reason: "no stage-genesis record while genesis_required is true"}}, findings...)}
	case genesis == nil:
		return CheckResult{Mode: ModeInactive, Findings: findings}
	case in.GenesisRequired:
		return CheckResult{Mode: ModeActive, Findings: findings}
	}
	return CheckResult{Mode: ModeReportOnly, Findings: findings}
}

func unrecorded(in CheckInput) []Finding {
	var out []Finding
	for _, key := range EnforcedKeys(in.Effective) {
		if !recorded(key, in.Records) {
			out = append(out, Finding{Key: key, Reason: "stage is enforce with no accepted flip record"})
		}
	}
	return out
}

func recorded(key string, set FlipSet) bool {
	for _, f := range set.Flips {
		if f.Key != key || f.Value != valueEnforce {
			continue
		}
		if f.Origin == OriginVerified || (set.Genesis != nil && slices.Contains(set.Genesis.Keys, key)) {
			return true
		}
	}
	return false
}
