// Package stageevidence records and seals shadow-stage evidence hash chains, computes decision version digests, and scores stage readiness.
package stageevidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

type Kind string

const (
	KindDecision Kind = "decision"
	KindLate     Kind = "late"
	KindOpen     Kind = "open"
	KindClose    Kind = "close"
)

type Class string

const (
	ClassNone      Class = "none"
	ClassFalseStop Class = "false_stop"
	ClassEscape    Class = "escape"
	ClassConflict  Class = "conflict"
)

var (
	ErrBrokenChain    = errors.New("broken chain")
	ErrEmptyChain     = errors.New("empty chain")
	ErrTrailingData   = errors.New("trailing data in policy JSON")
	ErrMissingPackage = errors.New("missing package in closure")
	ErrLedgerBroken   = errors.New("ledger broken")
	ErrCorruptStore   = errors.New("corrupt evidence store")
)

type BrokenChainError struct {
	Scope ScopeKey
	Seq   int
}

func (e *BrokenChainError) Error() string {
	return fmt.Sprintf("broken chain at %s %d seq %d", e.Scope.Kind, e.Scope.ID, e.Seq)
}

func (e *BrokenChainError) Unwrap() error {
	return ErrBrokenChain
}

func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrBrokenChain) {
		return "broken-chain"
	}
	if errors.Is(err, ErrEmptyChain) {
		return "empty-chain"
	}
	if errors.Is(err, ErrLedgerBroken) {
		return "ledger-broken"
	}
	if errors.Is(err, ErrCorruptStore) {
		return "corrupt-store"
	}
	return "error"
}

type Record struct {
	Scope   ScopeKind         `json:"scope,omitempty"`
	ID      int               `json:"id"`
	Session string            `json:"session,omitempty"`
	Key     string            `json:"key"`
	Version string            `json:"version"`
	Kind    Kind              `json:"kind"`
	Seq     int               `json:"seq"`
	Fields  map[string]string `json:"fields,omitempty"`
	Prev    string            `json:"prev,omitempty"`
	Hash    string            `json:"hash,omitempty"`
}

type Outcome struct {
	Scope      ScopeKind `json:"scope,omitempty"`
	ID         int       `json:"id"`
	Shipped    bool      `json:"shipped,omitempty"`
	Incomplete bool      `json:"incomplete,omitempty"`
	Failed     []string  `json:"failed,omitempty"`
	Routed     []string  `json:"routed,omitempty"`
}

type Chain struct {
	Records     []Record  `json:"records,omitempty"`
	Outcomes    []Outcome `json:"outcomes,omitempty"`
	Digest      string    `json:"-"`
	preverified bool
}

func PreverifiedChain(records []Record, outcomes []Outcome, digest string) Chain {
	return Chain{Records: records, Outcomes: outcomes, Digest: digest, preverified: true}
}

type Case struct {
	Class Class `json:"class"`
	Cycle int   `json:"cycle"`
	Seq   int   `json:"seq"`
}

type Result struct {
	Key              string         `json:"key"`
	Scorer           string         `json:"scorer"`
	Version          string         `json:"version"`
	ChainDigest      string         `json:"chain_digest,omitempty"`
	Error            string         `json:"error,omitempty"`
	Ready            bool           `json:"ready"`
	FirstCycle       int            `json:"first_cycle"`
	LastCycle        int            `json:"last_cycle"`
	Decisions        int            `json:"decisions"`
	FalseStops       int            `json:"false_stops"`
	Escapes          int            `json:"escapes"`
	Conflicts        int            `json:"conflicts"`
	IncompleteCycles []int          `json:"incomplete_cycles,omitempty"`
	Cases            []Case         `json:"cases,omitempty"`
	Late             []Record       `json:"late,omitempty"`
	OtherVersions    map[string]int `json:"other_versions,omitempty"`
}

type Scorer interface {
	Name() string
	Classify(Record, Outcome) Class
}

type defaultScorer struct{}

func (defaultScorer) Name() string {
	return "default"
}

func (defaultScorer) Classify(rec Record, out Outcome) Class {
	if rec.Fields["would_hold"] == "true" && rec.Fields["item"] != "" && slices.Contains(out.Routed, rec.Fields["item"]) {
		return ClassConflict
	}
	if rec.Fields["would_stop"] == "true" && out.Shipped {
		return ClassFalseStop
	}
	if rec.Fields["would_skip"] == "true" && rec.Fields["check"] != "" && slices.Contains(out.Failed, rec.Fields["check"]) {
		return ClassEscape
	}
	return ClassNone
}

func DefaultScorer() Scorer {
	return defaultScorer{}
}

func ScorerFor(_ string) Scorer {
	return DefaultScorer()
}

type hashedRecord struct {
	Scope   ScopeKind         `json:"scope,omitempty"`
	ID      int               `json:"id"`
	Session string            `json:"session,omitempty"`
	Key     string            `json:"key"`
	Version string            `json:"version"`
	Kind    Kind              `json:"kind"`
	Seq     int               `json:"seq"`
	Fields  map[string]string `json:"fields,omitempty"`
	Prev    string            `json:"prev,omitempty"`
}

func computeRecordHash(r Record) string {
	payload, _ := json.Marshal(hashedRecord{
		Scope:   r.Scope,
		ID:      r.ID,
		Session: r.Session,
		Key:     r.Key,
		Version: r.Version,
		Kind:    r.Kind,
		Seq:     r.Seq,
		Fields:  r.Fields,
		Prev:    r.Prev,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func Link(prev string, r Record) Record {
	r.Prev = prev
	r.Hash = computeRecordHash(r)
	return r
}

func VerifyChain(chain Chain) error {
	if len(chain.Records) == 0 {
		return ErrEmptyChain
	}
	prevByScope := map[ScopeKey]string{}
	for _, r := range chain.Records {
		scope := r.ScopeKey()
		if r.Prev != prevByScope[scope] || r.Hash == "" || r.Hash != computeRecordHash(r) {
			return &BrokenChainError{Scope: scope, Seq: r.Seq}
		}
		prevByScope[scope] = r.Hash
	}
	return nil
}

func indexOutcomes(list []Outcome) (map[ScopeKey]Outcome, []int) {
	byScope := make(map[ScopeKey]Outcome, len(list))
	var incomplete []int
	seenIncomplete := map[ScopeKey]bool{}
	for _, o := range list {
		scope := o.ScopeKey()
		byScope[scope] = o
		if o.Incomplete && !seenIncomplete[scope] {
			seenIncomplete[scope] = true
			incomplete = append(incomplete, o.ID)
		}
	}
	sort.Ints(incomplete)
	return byScope, incomplete
}

func tallyRecord(res *Result, r Record, out Outcome, scorer Scorer) {
	res.Decisions++
	if res.FirstCycle == 0 || r.ID < res.FirstCycle {
		res.FirstCycle = r.ID
	}
	if r.ID > res.LastCycle {
		res.LastCycle = r.ID
	}
	c := scorer.Classify(r, out)
	switch c {
	case ClassFalseStop:
		res.FalseStops++
	case ClassEscape:
		res.Escapes++
	case ClassConflict:
		res.Conflicts++
	default:
		return
	}
	res.Cases = append(res.Cases, Case{Class: c, Cycle: r.ID, Seq: r.Seq})
}

func Score(chain Chain, version string, promotion gatecontract.Promotion, scorer Scorer) Result {
	if scorer == nil {
		scorer = DefaultScorer()
	}
	outcomes, incomplete := indexOutcomes(chain.Outcomes)
	res := Result{
		Scorer:           scorer.Name(),
		Version:          version,
		IncompleteCycles: incomplete,
	}
	if len(chain.Records) > 0 {
		res.Key = chain.Records[0].Key
		res.ChainDigest = chain.chainDigest()
	}
	for _, r := range chain.Records {
		if r.Kind == KindLate {
			res.Late = append(res.Late, r)
			continue
		}
		if r.Version != version {
			if res.OtherVersions == nil {
				res.OtherVersions = map[string]int{}
			}
			res.OtherVersions[r.Version]++
			continue
		}
		out := outcomes[r.ScopeKey()]
		if out.Incomplete || r.Kind != KindDecision {
			continue
		}
		tallyRecord(&res, r, out, scorer)
	}
	minDecisions := max(promotion.MinDecisions, 1)
	res.Ready = res.Decisions >= minDecisions &&
		res.Escapes <= promotion.MaxEscapes &&
		res.FalseStops <= promotion.MaxFalseStops &&
		res.Conflicts == 0
	return res
}

func Verify(ctx context.Context, reader ChainReader, key, version string, promotion gatecontract.Promotion, scorer Scorer) (Result, error) {
	if scorer == nil {
		scorer = DefaultScorer()
	}
	base := Result{Key: key, Scorer: scorer.Name(), Version: version}
	if reader == nil {
		base.Error = ErrorCode(ErrEmptyChain)
		return base, ErrEmptyChain
	}
	chain, err := reader.ReadChain(ctx, key)
	if err != nil {
		base.Error = ErrorCode(err)
		return base, err
	}
	if !chain.preverified {
		if verr := VerifyChain(chain); verr != nil {
			base.Error = ErrorCode(verr)
			return base, verr
		}
	}
	res := Score(chain, version, promotion, scorer)
	res.Key = key
	return res, nil
}

func (c Chain) chainDigest() string {
	if c.Digest != "" {
		return c.Digest
	}
	return c.Records[len(c.Records)-1].Hash
}
