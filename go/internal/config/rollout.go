package config

// RolloutStages holds the subsystem rollout dials, embedded in RoutingConfig so each is promoted to a cfg field.
type RolloutStages struct {
	// CommitEvidence is the commit-as-evidence stage (off/shadow/enforce). The bridge subprocess reads
	// EVOLVE_COMMIT_EVIDENCE itself; this field is the orchestrator's view.
	CommitEvidence Stage
	// ReviewGate is the per-phase review gate (off/shadow/enforce); off keeps the no-op reviewer.
	ReviewGate Stage
	// EvalGate is the structural eval gate (off/shadow/enforce); it fails open on ambiguity.
	EvalGate Stage
	// ContractGate is the deliverable-contract gate (off/shadow/enforce); it fails open on ambiguity.
	ContractGate Stage
	// TriageCapGate is the triage capacity clamp (off/shadow/enforce); it fails open on ambiguity.
	TriageCapGate Stage
	// TopNGate is the build-to-audit top_n task-binding gate (off/shadow/enforce); it fails open on ambiguity.
	TopNGate Stage
	// SandboxMode is auto, on or off. The bridge subprocess reads EVOLVE_SANDBOX from its own env, so
	// setting this field without propagating the var into the bridge's env has no effect.
	SandboxMode string
	// PhaseRecovery is the recovery program dial (off/shadow/enforce). It arms several subsystems at
	// once; never flip it to get SpineFloor or FatalPane behavior.
	PhaseRecovery Stage
	// SpineFloor decides only whether a clean-absence handoff gap aborts (enforce) or warns (shadow).
	SpineFloor Stage
	// FatalPane decides only whether a persisted non-Busy fatal-pane match ends the stop-review wait.
	FatalPane Stage
	// PhaseIO is the unified phase-I/O envelope, on the full four-value ladder; off is the rollback.
	PhaseIO Stage
	// RouterReplan is the post-scout re-plan (off/shadow/advisory; enforce is unused).
	RouterReplan Stage
	// MergeGate is the merge-to-main gate dial, on the full ladder. Nothing reads it yet.
	MergeGate Stage
	// ParallelEvaluate runs the independent post-build evaluate phases concurrently at enforce.
	ParallelEvaluate Stage
	// ScoutDecompose is the scout map-reduce dial. Nothing reads it yet.
	ScoutDecompose Stage
}
