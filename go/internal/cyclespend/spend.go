package cyclespend

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

const StateFile = "cycle-state.json"

var (
	ErrLedger     = errors.New("llm-calls ledger")
	ErrCycleState = errors.New(StateFile)
)

type Spend struct {
	LLMMillis       int64 `json:"llm_ms"`
	FreshTokens     int64 `json:"fresh_tokens"`
	CacheReadTokens int64 `json:"cache_read_tokens"`
	UnmeasuredCalls int   `json:"unmeasured_calls"`
	AuditRounds     int   `json:"audit_rounds"`
}

func Sum(records []llmcalls.Record) Spend {
	var s Spend
	for _, r := range records {
		if r.DurationMS != nil {
			s.LLMMillis += *r.DurationMS
		}
		s.FreshTokens += int64(r.Tokens.Input + r.Tokens.Output + r.Tokens.CacheWrite)
		s.CacheReadTokens += int64(r.Tokens.CacheRead)
		if r.UsageStatus != llmcalls.UsageMeasured {
			s.UnmeasuredCalls++
		}
	}
	return s
}

func Read(workspace string) (Spend, error) {
	s, err := readLedger(workspace)
	rounds, stateErr := auditRounds(workspace)
	s.AuditRounds = rounds
	return s, errors.Join(err, stateErr)
}

func readLedger(workspace string) (Spend, error) {
	res, err := llmcalls.ReadWorkspace(workspace)
	if errors.Is(err, fs.ErrNotExist) {
		return Spend{}, nil
	}
	s := Sum(res.Records)
	s.UnmeasuredCalls += res.Skipped
	if err != nil {
		return s, fmt.Errorf("%w: %w", ErrLedger, err)
	}
	return s, nil
}

func auditRounds(workspace string) (int, error) {
	b, err := os.ReadFile(filepath.Join(workspace, StateFile))
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	var st cyclestate.CycleState
	if err == nil {
		err = json.Unmarshal(b, &st)
	}
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrCycleState, err)
	}
	return st.AuditDispatches, nil
}

func (s Spend) Add(o Spend) Spend {
	return Spend{
		LLMMillis:       s.LLMMillis + o.LLMMillis,
		FreshTokens:     s.FreshTokens + o.FreshTokens,
		CacheReadTokens: s.CacheReadTokens + o.CacheReadTokens,
		UnmeasuredCalls: s.UnmeasuredCalls + o.UnmeasuredCalls,
		AuditRounds:     s.AuditRounds + o.AuditRounds,
	}
}

func (s Spend) LLMMinutes() float64 { return float64(s.LLMMillis) / 60_000 }
