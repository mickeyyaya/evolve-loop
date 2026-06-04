package envchain

// keys.go is the single registry of operational EVOLVE_* env-var names and
// their default values. Centralizing the string literals means a rename
// touches one line and call sites can't drift on a typo; centralizing the
// defaults gives the previously-inlined magic numbers (Smell D) one home with
// a documented rationale.
//
// This registry grows as read sites migrate onto the typed getters — it is
// deliberately NOT an exhaustive enumeration of every flag in CLAUDE.md.
// Single-use, never-duplicated reads (e.g. one-off path overrides) stay inline
// at their call site; a key earns a constant here when a typed getter reads it.

// Env-var name constants.
const (
	// KeyPhaseMaxAttempts bounds per-phase retries on a recoverable bridge
	// timeout. Range [1,5]; out-of-range or unparseable → DefPhaseMaxAttempts.
	KeyPhaseMaxAttempts = "EVOLVE_PHASE_MAX_ATTEMPTS"

	// KeyRetryBackoffBaseS is the base seconds for exponential backoff between
	// phase retry attempts. Negative → 0 (disabled).
	KeyRetryBackoffBaseS = "EVOLVE_RETRY_BACKOFF_BASE_S"

	// KeyPhaseLatencyCeilingS is the global per-phase latency ceiling (seconds)
	// beyond which cycle-health raises a phase_latency anomaly. Per-phase
	// overrides use PhaseEnvKey(phase, "LATENCY_CEILING_S").
	KeyPhaseLatencyCeilingS = "EVOLVE_PHASE_LATENCY_CEILING_S"

	// KeyContractCorrectionRetries bounds how many times the orchestrator
	// re-dispatches a phase with a correction directive after a deliverable
	// contract violation. Range [0,5]; 0 disables (immediate abort, the
	// pre-feature behavior). Out-of-range/unparseable → DefContractCorrectionRetries.
	KeyContractCorrectionRetries = "EVOLVE_CONTRACT_CORRECTION_RETRIES"
)

// Default values for the knobs above.
const (
	// DefPhaseMaxAttempts: 2 = one relaunch after the first recoverable
	// timeout; a deterministic timeout still aborts after the cap.
	DefPhaseMaxAttempts = 2
	// MaxPhaseMaxAttempts is the hard ceiling on per-phase retries.
	MaxPhaseMaxAttempts = 5
	// DefRetryBackoffBaseS: 5s base for the exponential backoff ladder.
	DefRetryBackoffBaseS = 5
	// DefPhaseLatencyCeilingS: 15 minutes.
	DefPhaseLatencyCeilingS = 900
	// DefContractCorrectionRetries: 2 correction re-dispatches before abort.
	DefContractCorrectionRetries = 2
	// MaxContractCorrectionRetries is the hard ceiling on correction re-dispatches.
	MaxContractCorrectionRetries = 5
)
