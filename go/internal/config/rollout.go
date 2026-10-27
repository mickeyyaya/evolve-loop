package config

type RolloutStages struct {
	CommitEvidence   Stage
	ReviewGate       Stage
	EvalGate         Stage
	ContractGate     Stage
	TriageCapGate    Stage
	TopNGate         Stage
	PhaseRecovery    Stage
	SpineFloor       Stage
	FatalPane        Stage
	PhaseIO          Stage
	RouterReplan     Stage
	MergeGate        Stage
	ParallelEvaluate Stage
	ScoutDecompose   Stage
}
