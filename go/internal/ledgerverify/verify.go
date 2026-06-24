// Package ledgerverify ports verify_cycle from legacy/scripts/dispatch/
// evolve-loop-dispatch.sh:489-546. It walks the ledger.jsonl chain once
// per cycle and asserts the pipeline ran end-to-end: scout + builder +
// auditor each have at least one exit_code-zero entry, plus intent if the
// cycle was init'd with intent_required=true, plus memo if the cycle's
// audit verdict was PASS.
//
// Ledger vocabulary (cycle-137 fix): two writers record per-phase entries
// with DIFFERENT shapes, and the verifier must accept both or it
// false-negatives an entire class of cycles:
//
//   - bash subagent-run.sh   → kind="agent_subprocess", role=AGENT name
//     ("scout", "builder", "auditor")
//   - Go native orchestrator → kind="phase",            role=PHASE name
//     ("scout", "build", "audit")
//
// The Go `evolve loop` path emits ONLY kind="phase" entries (no
// agent_subprocess), so a verifier that matched the bash shape alone
// reported "missing [scout builder auditor]" for every native cycle —
// the cycle-137 incident. canonicalRole + the kind allow-list below
// fold both vocabularies onto the same {scout,builder,auditor,intent,
// memo} buckets so bash and Go cycles verify identically.
//
// Pure: the package depends only on core.Ledger + small data types.
// Callers (cmd_loop) are responsible for sourcing IntentRequired from
// cycle-state.json (priority) or state.json (fallback) and CycleVerdict
// from .cycle-verdict — see LoadVerifyContext.
package ledgerverify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Ledger entry kinds that represent a completed phase/agent run. Both
// count toward verification (see package doc): the bash dispatcher wrote
// agent_subprocess, the Go orchestrator writes phase. Any other kind
// (agent_fanout, cycle_terminal, routing_decision, …) is bookkeeping and
// is ignored.
const (
	kindSubprocess = "agent_subprocess"
	kindPhase      = "phase"
)

// Roles required for every cycle (pre-intent, pre-memo gates). Canonical
// names; canonicalRole maps both ledger vocabularies onto these.
var requiredRoles = []string{"scout", "builder", "auditor"}

// canonicalRole folds the two ledger role vocabularies onto the canonical
// buckets the verifier counts. The bash dispatcher records AGENT names
// (builder, auditor); the Go orchestrator records PHASE names (build,
// audit). scout/intent/memo are spelled identically in both. An
// unrecognized role returns "" and is not counted.
func canonicalRole(role string) string {
	switch role {
	case "scout":
		return "scout"
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

// countsTowardVerify reports whether a ledger entry kind represents a real
// phase/agent run (vs bookkeeping entries that must not satisfy a role).
func countsTowardVerify(kind string) bool {
	return kind == kindSubprocess || kind == kindPhase
}

// Options pins the cycle-specific gates: which optional roles to also
// require. The caller fills these from cycle-state.json + .cycle-verdict.
type Options struct {
	// IntentRequired triggers the "intent agent_subprocess entry must
	// exist" check. Source: cycle-state.json:intent_required (priority)
	// > state.json:intent_required.
	IntentRequired bool
	// CycleVerdict is the audit verdict for this cycle (PASS / FAIL /
	// WARN / ""). When PASS, the memo agent_subprocess entry MUST exist
	// (Layer P / Layer E3 contract from v8.58.0). Other verdicts skip
	// the memo check.
	CycleVerdict string
}

// Result summarises what VerifyCycle found.
//
// OK is true iff Missing is empty. Counts always reflect what the ledger
// contains (exit_code==0 entries only). Missing names the roles whose
// count came back zero against the resolved requirement set.
type Result struct {
	Scout   int      `json:"scout"`
	Builder int      `json:"builder"`
	Auditor int      `json:"auditor"`
	Intent  int      `json:"intent"`
	Memo    int      `json:"memo"`
	Missing []string `json:"missing,omitempty"`
	OK      bool     `json:"ok"`
	// Required snapshots the role set Options resolved to. Useful for
	// diagnostics and for the cmd_loop log line that mirrors the bash
	// `log "ledger: cycle=$cycle scout=$s builder=$b ..."`.
	Required []string `json:"required"`
}

// VerifyCycle reads every ledger entry for `cycle` and asserts the
// required roles all have at least one exit_code==0 subprocess entry.
//
// Returns Result.OK=true and a nil error when the pipeline is complete.
// Returns Result.OK=false and a nil error when one or more required
// roles are missing — the caller decides whether to halt, classify, or
// continue per EVOLVE_DISPATCH_POLICY. A non-nil error is reserved for
// ledger I/O failures.
func VerifyCycle(ctx context.Context, ledger core.Ledger, cycle int, opts Options) (Result, error) {
	required := make([]string, 0, 5)
	required = append(required, requiredRoles...)
	if opts.IntentRequired {
		required = append(required, "intent")
	}
	if strings.EqualFold(opts.CycleVerdict, "PASS") {
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

// roleCount returns the counter field matching a canonical role name.
func roleCount(r Result, role string) int {
	switch role {
	case "scout":
		return r.Scout
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

// VerifyContext holds the booleans VerifyCycle needs. Caller-facing
// rather than baked into VerifyCycle so unit tests don't need to
// fixture-create cycle-state.json + .cycle-verdict files.
type VerifyContext struct {
	IntentRequired bool
	CycleVerdict   string
}

// LoadVerifyContext resolves IntentRequired + CycleVerdict from on-disk
// state per the bash precedence:
//
//	IntentRequired = cycle-state.json:intent_required (if file exists)
//	                 > state.json:intent_required (fallback)
//	                 > false
//	CycleVerdict   = contents of <workspace>/.cycle-verdict
//	                 (trimmed; "" if missing)
//
// Missing/unreadable files are non-fatal: the strictest behavior is
// "no extra requirement", consistent with the bash defaults.
func LoadVerifyContext(workspace, evolveDir string) VerifyContext {
	vc := VerifyContext{}
	// Per-cycle file takes precedence over the global state.
	if v, ok := readIntentRequired(filepath.Join(workspace, "cycle-state.json")); ok {
		vc.IntentRequired = v
	} else if evolveDir != "" {
		if v, ok := readIntentRequired(filepath.Join(evolveDir, "state.json")); ok {
			vc.IntentRequired = v
		}
	}
	// Verdict file is workspace-local.
	if data, err := os.ReadFile(filepath.Join(workspace, ".cycle-verdict")); err == nil {
		vc.CycleVerdict = strings.TrimSpace(string(data))
	}
	return vc
}

// readIntentRequired reads `intent_required` (top-level bool) from a JSON
// file. Returns (value, true) on success, (false, false) on any error
// (missing file, bad JSON, missing key). Matches the bash `jq -r
// '.intent_required // false'` semantics without pulling in a full
// schema decode.
func readIntentRequired(path string) (bool, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	// Avoid a full struct decode because cycle-state.json contains many
	// fields not modeled in core.CycleState; a `map[string]any` is the
	// safe portable option.
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
