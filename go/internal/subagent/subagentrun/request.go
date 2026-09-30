package subagentrun

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

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

	Prompt io.Reader

	ModelTierHint          string
	AuditorTierOverride    string
	DiffComplexityDisabled bool
	AdversarialAudit       bool
	LegacyAgentDispatch    bool
	DispatchDepth          int
	ChallengeTokenOverride string
}

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

type Profile struct {
	CLI            string
	OutputArtifact string
	Overrides      func(cli string) (toolsJSON, extraFlagsJSON string)
}

type LLM struct {
	CLI       string
	ModelTier string
	Source    string
}

type Capability struct {
	BudgetNative      bool
	PermissionScoping bool
	Warns             []string
}

type TierRequest struct {
	ProfilePath            string
	Cycle                  int
	ProjectRoot            string
	WorktreePath           string
	ModelTierHint          string
	AuditorTierOverride    string
	DiffComplexityDisabled bool
}

type identity struct {
	agent  string
	role   string
	worker string
	runID  string
}

type plan struct {
	profilePath string
	profile     Profile
	cli         string
	source      string
	model       string
	adapterPath string
	cap         Capability
}

type provenance struct {
	artifactPath string
	token        string
	gitHead      string
	treeDiff     string
	prompt       string
}

type execution struct {
	exitCode   int
	execErr    error
	durationMS int64
}
