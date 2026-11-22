package stageevidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const SealVersion = 1

const (
	CauseWriteFailed = "write-failed"
	CauseOpenSession = "open-session"
	CauseGap         = "gap"
	CauseTornTail    = "torn-tail"
)

type Seal struct {
	V          int            `json:"v"`
	Count      int            `json:"count"`
	Last       string         `json:"last"`
	Incomplete bool           `json:"incomplete,omitempty"`
	Cause      string         `json:"cause,omitempty"`
	Shipped    bool           `json:"shipped,omitempty"`
	Exit       string         `json:"exit,omitempty"`
	Counters   map[string]int `json:"counters,omitempty"`
}

type SealInput struct {
	Prior   *Seal
	Shipped bool
	Exit    string
}

func (s Seal) Encode() string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func ParseSeal(message string) (Seal, error) {
	var s Seal
	if err := json.Unmarshal([]byte(message), &s); err != nil {
		return Seal{}, fmt.Errorf("stageevidence: parse seal: %w", err)
	}
	if s.V != SealVersion {
		return Seal{}, fmt.Errorf("stageevidence: seal version %d is not %d", s.V, SealVersion)
	}
	return s, nil
}

func (s Seal) IsPrefixOf(records []Record) bool {
	return s.Count >= 1 && s.Count <= len(records) && records[s.Count-1].Hash == s.Last
}

func (s Seal) Outcome(scope ScopeKey) Outcome {
	return Outcome{Scope: scope.Kind, ID: scope.ID, Shipped: s.Shipped, Incomplete: s.Incomplete}
}

func BuildSeal(scope ScopeKey, working []byte, in SealInput) (Seal, []Record, error) {
	records, torn := parseWorking(working)
	if len(records) == 0 {
		return Seal{}, nil, ErrEmptyChain
	}
	for _, r := range records {
		if r.ScopeKey() != scope {
			return Seal{}, nil, &BrokenChainError{Scope: scope, Seq: r.Seq}
		}
	}
	if err := VerifyChain(Chain{Records: records}); err != nil {
		return Seal{}, nil, err
	}
	if p := in.Prior; p != nil && !p.IsPrefixOf(records) {
		return Seal{}, nil, &BrokenChainError{Scope: scope, Seq: p.Count}
	}
	seal := Seal{V: SealVersion, Count: len(records), Last: records[len(records)-1].Hash, Shipped: in.Shipped, Exit: in.Exit, Counters: decisionCounters(records)}
	seal.Cause = incompleteCause(records, torn)
	seal.Incomplete = seal.Cause != ""
	return seal, records, nil
}

func VerifyStored(scope ScopeKey, stored []byte, seal Seal) ([]Record, error) {
	rebuilt, records, err := BuildSeal(scope, stored, SealInput{Shipped: seal.Shipped, Exit: seal.Exit})
	if err != nil {
		return nil, err
	}
	if !sameSeal(rebuilt, seal) {
		return nil, &BrokenChainError{Scope: scope, Seq: rebuilt.Count}
	}
	return records, nil
}

func sameSeal(a, b Seal) bool {
	return a.Encode() == b.Encode()
}

func decisionCounters(records []Record) map[string]int {
	counters := map[string]int{}
	for _, r := range records {
		if r.Kind == KindDecision {
			counters[r.Key]++
		}
	}
	return counters
}

type sessionTally struct {
	closed    bool
	faulted   bool
	decisions map[string]int
	counters  map[string]int
}

func incompleteCause(records []Record, torn int) string {
	sessions := tallySessions(records)
	ids := make([]string, 0, len(sessions))
	for id := range sessions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	cause := ""
	for _, id := range ids {
		cause = strongerCause(cause, sessionCause(sessions[id]))
	}
	if torn > 0 {
		cause = strongerCause(cause, CauseTornTail)
	}
	return cause
}

var causeOrder = []string{CauseWriteFailed, CauseOpenSession, CauseGap, CauseTornTail}

func strongerCause(a, b string) string {
	if a == "" || (b != "" && slices.Index(causeOrder, b) < slices.Index(causeOrder, a)) {
		return b
	}
	return a
}

func sessionCause(t *sessionTally) string {
	switch {
	case t.faulted:
		return CauseWriteFailed
	case !t.closed:
		return CauseOpenSession
	case !sameCounts(t.decisions, t.counters):
		return CauseGap
	}
	return ""
}

func sameCounts(a, b map[string]int) bool {
	for k, n := range a {
		if b[k] != n {
			return false
		}
	}
	for k, n := range b {
		if a[k] != n {
			return false
		}
	}
	return true
}

func tallySessions(records []Record) map[string]*sessionTally {
	sessions := map[string]*sessionTally{}
	for _, r := range records {
		t := sessions[r.Session]
		if t == nil {
			t = &sessionTally{decisions: map[string]int{}, counters: map[string]int{}}
			sessions[r.Session] = t
		}
		switch r.Kind {
		case KindDecision:
			t.decisions[r.Key]++
		case KindClose:
			t.closed = true
			t.faulted = r.Fields[faultField] != ""
			readCounters(t, r.Fields)
		}
	}
	return sessions
}

func readCounters(t *sessionTally, fields map[string]string) {
	for name, value := range fields {
		key, ok := strings.CutPrefix(name, counterPrefix)
		if !ok {
			continue
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			n = -1
		}
		t.counters[key] = n
	}
}

func parseWorking(raw []byte) (records []Record, torn int) {
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			torn++
			continue
		}
		records = append(records, r)
	}
	return records, torn
}

func lastRecord(raw []byte) Record {
	parts := bytes.Split(raw, []byte("\n"))
	for i := len(parts) - 1; i >= 0; i-- {
		var r Record
		if len(parts[i]) > 0 && json.Unmarshal(parts[i], &r) == nil {
			return r
		}
	}
	return Record{}
}

func appendLine(dst []byte, r Record) []byte {
	line, _ := json.Marshal(r)
	return append(append(dst, line...), '\n')
}
