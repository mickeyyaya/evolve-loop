// Package ledgerverify asserts from the ledger that a cycle ran its required roles end to end.
// See docs/architecture/packages/internal-ledgerverify.md.
package ledgerverify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const (
	kindSubprocess = "agent_subprocess"
	kindPhase      = "phase"
)

var requiredRoles = phasecontract.RequiredRoles()

func canonicalRole(role string) string {
	switch role {
	case "scout":
		return "scout"
	case "triage":
		return "triage"
	case "builder", "build":
		return "builder"
	case "auditor", "audit":
		return "auditor"
	case "intent":
		return "intent"
	case "memo":
		return "memo"
	default:
		return ""
	}
}

func countsTowardVerify(kind string) bool {
	return kind == kindSubprocess || kind == kindPhase
}

type Options struct {
	IntentRequired         bool
	CycleVerdict           string
	EmptyTriageTermination bool
}

type Result struct {
	Scout    int      `json:"scout"`
	Triage   int      `json:"triage"`
	Builder  int      `json:"builder"`
	Auditor  int      `json:"auditor"`
	Intent   int      `json:"intent"`
	Memo     int      `json:"memo"`
	Missing  []string `json:"missing,omitempty"`
	OK       bool     `json:"ok"`
	Required []string `json:"required"`
}

func VerifyCycle(ctx context.Context, ledger core.Ledger, cycle int, opts Options) (Result, error) {
	required := make([]string, 0, 5)
	if opts.EmptyTriageTermination {
		required = append(required, "scout", "triage")
	} else {
		required = append(required, requiredRoles...)
	}
	if opts.IntentRequired {
		required = append(required, "intent")
	}
	if strings.EqualFold(opts.CycleVerdict, "PASS") && !opts.EmptyTriageTermination {
		required = append(required, "memo")
	}

	r := Result{Required: required}

	it, err := ledger.Iter(ctx)
	if err != nil {
		return r, fmt.Errorf("ledger iter: %w", err)
	}
	defer func() { _ = it.Close() }()

	for {
		entry, ok, err := it.Next()
		if err != nil {
			return r, fmt.Errorf("ledger next: %w", err)
		}
		if !ok {
			break
		}
		if entry.Cycle != cycle {
			continue
		}
		if !countsTowardVerify(entry.Kind) {
			continue
		}
		if entry.ExitCode != 0 {
			continue
		}
		switch canonicalRole(entry.Role) {
		case "scout":
			r.Scout++
		case "triage":
			r.Triage++
		case "builder":
			r.Builder++
		case "auditor":
			r.Auditor++
		case "intent":
			r.Intent++
		case "memo":
			r.Memo++
		}
	}

	for _, role := range required {
		if roleCount(r, role) == 0 {
			r.Missing = append(r.Missing, role)
		}
	}
	r.OK = len(r.Missing) == 0
	return r, nil
}

func roleCount(r Result, role string) int {
	switch role {
	case "scout":
		return r.Scout
	case "triage":
		return r.Triage
	case "builder":
		return r.Builder
	case "auditor":
		return r.Auditor
	case "intent":
		return r.Intent
	case "memo":
		return r.Memo
	}
	return 0
}

type VerifyContext struct {
	IntentRequired         bool
	CycleVerdict           string
	EmptyTriageTermination bool
}

func LoadVerifyContext(workspace, evolveDir string) VerifyContext {
	vc := VerifyContext{}
	if v, ok := readIntentRequired(filepath.Join(workspace, "cycle-state.json")); ok {
		vc.IntentRequired = v
	} else if evolveDir != "" {
		if v, ok := readIntentRequired(filepath.Join(evolveDir, "state.json")); ok {
			vc.IntentRequired = v
		}
	}
	if data, err := os.ReadFile(filepath.Join(workspace, ".cycle-verdict")); err == nil {
		vc.CycleVerdict = strings.TrimSpace(string(data))
	}
	return vc
}

func readIntentRequired(path string) (bool, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, false
	}
	v, ok := raw["intent_required"]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	if !ok {
		return false, false
	}
	return b, true
}
