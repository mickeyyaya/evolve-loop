package core

import "context"

// Phase is the typed identity of an orchestrator lifecycle stage.
// Stringly-backed for JSON portability.
type Phase string

const (
	PhaseStart  Phase = "start"
	PhaseIntent Phase = "intent"
	PhaseScout  Phase = "scout"
	PhaseTriage Phase = "triage"
	PhaseTDD    Phase = "tdd"
	PhaseBuild  Phase = "build"
	PhaseAudit  Phase = "audit"
	PhaseShip   Phase = "ship"
	PhaseRetro  Phase = "retro"
	PhaseEnd    Phase = "end"
)

// String implements fmt.Stringer.
func (p Phase) String() string { return string(p) }

// IsValid reports whether p is one of the known phase constants.
func (p Phase) IsValid() bool {
	switch p {
	case PhaseStart, PhaseIntent, PhaseScout, PhaseTriage,
		PhaseTDD, PhaseBuild, PhaseAudit, PhaseShip, PhaseRetro, PhaseEnd:
		return true
	}
	return false
}

// Verdict constants — the four outcomes a phase may emit. These match
// the EGPS gate vocabulary (CLAUDE.md env-var table: WARN removed at
// v10.0.0 but still accepted by Audit for the pre-EGPS soft-start
// boundary; SKIPPED used when a phase opted out, e.g. EVOLVE_TRIAGE_DISABLE).
const (
	VerdictPASS    = "PASS"
	VerdictFAIL    = "FAIL"
	VerdictWARN    = "WARN"
	VerdictSKIPPED = "SKIPPED"
)

// IsVerdict reports whether s is one of the canonical verdict strings.
// Case- and whitespace-sensitive — guards against silent typos.
func IsVerdict(s string) bool {
	switch s {
	case VerdictPASS, VerdictFAIL, VerdictWARN, VerdictSKIPPED:
		return true
	}
	return false
}

// BudgetEnvelope is the per-call budget envelope passed to each phase.
type BudgetEnvelope struct {
	MaxUSD      float64 `json:"max_usd"`
	BatchCapUSD float64 `json:"batch_cap_usd"`
}

// TokenUsage records the LLM token counts attributed to a phase run.
type TokenUsage struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cache_read"`
	CacheWrite int `json:"cache_write"`
}

// Diagnostic is a single warning/error attached to a PhaseResponse.
type Diagnostic struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// PhaseRequest is the input envelope to PhaseRunner.Run. JSON-tagged
// so the subprocess override path (pkg/phaseproto) can serialise the
// same struct over stdin/stdout.
type PhaseRequest struct {
	Cycle         int               `json:"cycle"`
	ProjectRoot   string            `json:"project_root"`
	Workspace     string            `json:"workspace"`
	Worktree      string            `json:"worktree"`
	GoalHash      string            `json:"goal_hash"`
	Context       map[string]string `json:"context,omitempty"`
	Budget        BudgetEnvelope    `json:"budget"`
	PreviousPhase string            `json:"previous_phase,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
}

// PhaseResponse is the output envelope from PhaseRunner.Run.
type PhaseResponse struct {
	Phase        string       `json:"phase"`
	Verdict      string       `json:"verdict"`
	ArtifactsDir string       `json:"artifacts_dir"`
	NextPhase    string       `json:"next_phase,omitempty"`
	CostUSD      float64      `json:"cost_usd"`
	Tokens       TokenUsage   `json:"tokens"`
	DurationMS   int64        `json:"duration_ms"`
	Diagnostics  []Diagnostic `json:"diagnostics,omitempty"`
}

// PhaseRunner runs a single phase. The orchestrator never knows which
// runner impl is in play (in-process vs subprocess) — that's the
// independence guarantee for Approach C.
type PhaseRunner interface {
	Name() string
	Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error)
}
