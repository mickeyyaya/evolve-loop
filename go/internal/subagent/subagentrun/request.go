package subagentrun

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// Request is the leaf's input — every value the execution path reads. The
// host projects its RunRequest onto it ONCE (the dead PluginRoot never rides
// in). Prompt MUST be non-nil: the run fails fast when no prompt source is
// configured.
type Request struct {
	Agent         string
	Cycle         int
	WorkspacePath string

	ProfilesDir   string
	AdaptersDir   string
	CapabilityDir string
	ProjectRoot   string
	WorktreePath  string
	LedgerPath    string

	// Prompt supplies the user task prompt (PROMPT_FILE_OVERRIDE, stdin or a
	// buffer).
	Prompt io.Reader

	ModelTierHint          string
	AuditorTierOverride    string
	DiffComplexityDisabled bool
	AdversarialAudit       bool
	LegacyAgentDispatch    bool
	DispatchDepth          int
	// ChallengeTokenOverride pins the challenge token instead of minting one —
	// a fan-out worker is dispatched with the parent-dictated token so its
	// artifact bears a token the parent can verify. Empty ⇒ mint.
	ChallengeTokenOverride string
}

// Outcome is what Dispatch produced: the host's RunResult minus the never-set
// stderr, plus the verification evidence the run used to drop (the ladder's
// diagnostics and the typed integrity rung). Warns is the capability
// inspector's warns followed by the worktree-fallback sentence, byte-identical
// to before.
type Outcome struct {
	Verdict        string
	CLI            string
	Model          string
	ArtifactPath   string
	ArtifactSHA256 string
	ChallengeToken string
	ExitCode       int
	DurationMS     int64
	Warns          []string
	Diagnostics    []cyclestate.Diagnostic
	Integrity      IntegrityReason
}

// Profile is the typed projection of an agent profile the path consumes: the
// cli field, the output_artifact template, and the adapter overrides for the
// cli resolved later (a closure over the profile body, because the cli is
// known only after resolution).
type Profile struct {
	CLI            string
	OutputArtifact string
	Overrides      func(cli string) (toolsJSON, extraFlagsJSON string)
}

// LLM is what the LLM router resolved for a role.
type LLM struct {
	CLI       string
	ModelTier string
	Source    string
}

// Capability is what the capability inspector found for a cli.
type Capability struct {
	BudgetNative      bool
	PermissionScoping bool
	Warns             []string
}

// TierRequest is what the adaptive model-tier resolver receives.
type TierRequest struct {
	ProfilePath            string
	Cycle                  int
	ProjectRoot            string
	WorktreePath           string
	ModelTierHint          string
	AuditorTierOverride    string
	DiffComplexityDisabled bool
}

// identity is what admission derived about the request: the full dispatched
// name, its parsed role and worker subtask, and — stamped as the gate's last
// act — the run id every later signal carries. Complete when admit returns;
// every later step takes it by value.
type identity struct {
	agent  string
	role   string
	worker string
	runID  string
}

// plan is what resolution produced (steps 2-7 of the path).
type plan struct {
	profilePath string
	profile     Profile
	cli         string
	source      string
	model       string
	adapterPath string
	cap         Capability
}

// provenance is what preparation produced (steps 8-11): where the artifact
// goes, the token it must bear, the git state the ledger stamps, and the
// composed prompt.
type provenance struct {
	artifactPath string
	token        string
	gitHead      string
	treeDiff     string
	prompt       string
}

// execution is what the adapter call produced (step 12).
type execution struct {
	exitCode   int
	execErr    error
	durationMS int64
}
