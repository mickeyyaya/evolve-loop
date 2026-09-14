package dashboard

import "time"

// State is the closed vocabulary of a cycle's state TYPE (Prefect-style: the
// type drives colour and sorting, the human-readable name rides beside it).
const (
	// StateRunning: the current cycle holds a fresh run lease.
	StateRunning = "running"
	// StatePass / StateWarn / StateFail: the committed dossier's final verdict.
	StatePass = "pass"
	StateWarn = "warn"
	StateFail = "fail"
	// StateHalted: the cycle's failure-decision.json declared a SYSTEM-level
	// failure — the loop stopped itself on this cycle.
	StateHalted = "halted"
	// StateIncomplete: no dossier and no fresh lease — paused, crashed, or
	// still checkpointed mid-cycle.
	StateIncomplete = "incomplete"
)

// Snapshot is the immutable, whole-picture value the page renders. It is
// rebuilt on every detected change and swapped by pointer; nothing mutates it.
type Snapshot struct {
	GeneratedAt time.Time `json:"generated_at"`
	// Root is the project root the snapshot was read from.
	Root string `json:"root"`
	// Loop is the live status of the loop process (lease, brake, current phase).
	Loop LoopStatus `json:"loop"`
	// Queue is the inbox: pending items plus lifecycle counts.
	Queue QueueSummary `json:"queue"`
	// Cycles are the most recent cycles, newest first — every cycle with a run
	// workspace on disk plus the newest dossier-only cycles up to the cap.
	Cycles []CycleSummary `json:"cycles"`
	// Trend is the ship-rate history computed from the committed dossiers.
	Trend Trend `json:"trend"`
	// Fingerprints are failure identities grouped Sentry-style, most recent first.
	Fingerprints []FingerprintStat `json:"fingerprints"`
	// Warnings lists artifacts that could not be read. Absence is normal for a
	// sparse or garbage-collected workspace; the list exists so a reader can
	// tell "absent" from "hidden".
	Warnings []string `json:"warnings,omitempty"`
}

// LoopStatus is what the loop process is doing right now.
type LoopStatus struct {
	// Running is true when the current cycle's run lease is fresh
	// (runlease.Fresh); a live PID is never treated as liveness.
	Running bool `json:"running"`
	// BrakeEngaged is true when `.evolve/loop-stop` exists.
	BrakeEngaged bool `json:"brake_engaged"`
	// CycleID / Phase / PhaseStartedAt mirror cycle-state.json.
	CycleID        int       `json:"cycle_id"`
	Phase          string    `json:"phase"`
	PhaseStartedAt time.Time `json:"phase_started_at"`
	// LeaseHeartbeat is the lease's last heartbeat (zero when no lease).
	LeaseHeartbeat time.Time `json:"lease_heartbeat"`
	ActiveWorktree string    `json:"active_worktree,omitempty"`
	// CLI / Model are the current phase's dispatch as recorded in llm-calls.ndjson.
	CLI   string `json:"cli,omitempty"`
	Model string `json:"model,omitempty"`
	// AuditRounds is the cycle's audit dispatch count so far.
	AuditRounds int `json:"audit_rounds"`
	// Checkpointed is true when cycle-state.json was absent (a cleanly stopped
	// loop removes it) and the fields above came from the newest run.json —
	// i.e. this is where the loop was when it stopped, not what it is doing.
	Checkpointed bool `json:"checkpointed,omitempty"`
}

// QueueSummary is the inbox at a glance.
type QueueSummary struct {
	Pending    []QueueItem `json:"pending"`
	Consumed   int         `json:"consumed"`
	Processing int         `json:"processing"`
	Retry      int         `json:"retry"`
	Processed  int         `json:"processed"`
}

// QueueItem is one pending inbox item (the fields inboxbatch models).
type QueueItem struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Kind     string  `json:"kind,omitempty"`
	Class    string  `json:"class,omitempty"`
	Route    string  `json:"route,omitempty"`
	Priority string  `json:"priority,omitempty"`
	Weight   float64 `json:"weight"`
}

// CycleSummary is one cycle's progress and outcome.
type CycleSummary struct {
	ID int `json:"id"`
	// State is one of the State* constants; StateName is the human label
	// shown beside it (e.g. "paused (brake)").
	State     string `json:"state"`
	StateName string `json:"state_name"`
	// Verdict is the dossier's final verdict ("" when no dossier yet).
	Verdict   string    `json:"verdict,omitempty"`
	CommitSHA string    `json:"commit_sha,omitempty"`
	Goal      string    `json:"goal,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	// Phases are the dispatches in the order they ran (from phase-timing.json).
	Phases []PhaseRun `json:"phases"`
	// AuditRounds is the number of audit dispatches (cycle-state / run.json).
	AuditRounds int `json:"audit_rounds"`
	// Tasks are the triage top_n slugs this cycle committed to.
	Tasks []string `json:"tasks,omitempty"`
	// Failure is nil on PASS cycles and when no failure artifact exists.
	Failure *Failure `json:"failure,omitempty"`
	// Tokens is the cycle's terminal token total (phasetiming.Rollup).
	Tokens int `json:"tokens"`
	// HasWorkspace / HasDossier say which sources fed this summary.
	HasWorkspace bool `json:"has_workspace"`
	HasDossier   bool `json:"has_dossier"`
	// CurrentPhase is set only on the running/incomplete cycle.
	CurrentPhase string `json:"current_phase,omitempty"`
	// Plan is the cycle's required sequence at a glance (nil without a run
	// workspace: a dossier-only cycle has nothing left to project).
	Plan *PhasePlan `json:"plan,omitempty"`
}

// PhasePlan is a cycle's phase sequence as the operator reads it: the
// registry's mandatory set, every phase the cycle ran, each step with a
// status, the counts, what is ongoing and what remains, and how the advisor's
// proposal fared against what ran.
type PhasePlan struct {
	// Mandatory is config.mandatory_phases — the set the router's floor
	// enforces — in order.
	Mandatory []string `json:"mandatory"`
	// Steps is the sequence in run order: executed phases, the ongoing one,
	// then the mandatory phases not yet reached.
	Steps []PlanStep `json:"steps"`
	// Required counts the mandatory steps plus the conditional-mandatory ones
	// that ran; PassedRequired how many of those passed ("passed 3/5 required").
	// Total is the sequence length and Passed how many of all steps passed.
	Required       int `json:"required"`
	PassedRequired int `json:"passed_required"`
	Total          int `json:"total"`
	Passed         int `json:"passed"`
	// Ongoing is the running phase (running cycles only) and OngoingSince its
	// start; Remaining lists the pending phases in order.
	Ongoing      string    `json:"ongoing,omitempty"`
	OngoingSince time.Time `json:"ongoing_since"`
	Remaining    []string  `json:"remaining,omitempty"`
	// AdvisorProposed are phases the advisor proposed to run that have not
	// run (not yet, on a running cycle; never, on a sealed one).
	// AdvisorSkips are phases it proposed to skip that did not run;
	// AdvisorOverridden the ones it proposed to skip that ran anyway — the
	// mandatory floor overrode the proposal.
	AdvisorProposed   []string `json:"advisor_proposed,omitempty"`
	AdvisorSkips      []string `json:"advisor_skips,omitempty"`
	AdvisorOverridden []string `json:"advisor_overridden,omitempty"`
}

// PlanStep is one phase in a PhasePlan.
type PlanStep struct {
	Phase string `json:"phase"`
	// Status is one of pass, warn, fail (the last run's verdict), ongoing,
	// pending (a mandatory phase a running cycle has not reached), unreached
	// (a mandatory phase a sealed cycle never ran) or skipped (a mandatory
	// phase the cycle went past without running).
	Status string `json:"status"`
	// Optional is true for a phase outside the mandatory and
	// conditional-mandatory sets; Conditional for a conditional-mandatory one.
	Optional    bool `json:"optional,omitempty"`
	Conditional bool `json:"conditional,omitempty"`
	// GateVerified is true when the contract gate recorded
	// GATE_CONTRACT_VERIFIED for the phase in the cycle's signal stream.
	GateVerified bool `json:"gate_verified,omitempty"`
	// DurationMS is the last run's wall clock; Rounds its occurrence count.
	DurationMS int64 `json:"duration_ms,omitempty"`
	Rounds     int   `json:"rounds,omitempty"`
}

// PhaseRun is one phase dispatch inside a cycle.
type PhaseRun struct {
	Phase     string    `json:"phase"`
	Verdict   string    `json:"verdict"`
	Archetype string    `json:"archetype,omitempty"`
	CLI       string    `json:"cli,omitempty"`
	Model     string    `json:"model,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	// DurationMS is wall-clock; Attempt is the attempt count recorded by the
	// dispatcher; Round is the 1-based occurrence of this phase name within the
	// cycle (audit round 2 = the second "audit" entry).
	DurationMS int64 `json:"duration_ms"`
	Attempt    int   `json:"attempt"`
	Round      int   `json:"round"`
	Tokens     int   `json:"tokens"`
}

// Failure is everything a human needs to act on a FAILed cycle in under thirty
// seconds: classification, identity, recurrence, root cause, the gate reasons,
// the auditor's findings, the repair-round history, and where the work is.
type Failure struct {
	// From failure-decision.json (retro-authored, policy-validated).
	Category string `json:"category,omitempty"`
	Level    string `json:"level,omitempty"`
	Action   string `json:"action,omitempty"`
	FixType  string `json:"fix_type,omitempty"`
	// From failure-digest.json / the dossier: the deterministic identity.
	Fingerprint string `json:"fingerprint,omitempty"`
	PreClass    string `json:"pre_class,omitempty"`
	// From disposition.json (retro-authored, digest-cross-checked).
	Legitimacy string `json:"legitimacy,omitempty"`
	Layer      string `json:"layer,omitempty"`
	RootCause  string `json:"root_cause,omitempty"`
	Salvage    string `json:"salvage,omitempty"`
	Urgency    string `json:"urgency,omitempty"`
	// GateReasons are the deterministic gate strings (audit-fail-reason.json).
	GateReasons []string `json:"gate_reasons,omitempty"`
	// Findings are the FINAL audit round's issues, highest severity first.
	Findings []Finding `json:"findings,omitempty"`
	// Rounds is the immutable repair-round history, round 1 first.
	Rounds []AuditRound `json:"rounds,omitempty"`
}

// AuditRound is one archived audit round and its delta against the previous.
type AuditRound struct {
	Round    int       `json:"round"`
	Verdict  string    `json:"verdict"`
	Findings []Finding `json:"findings"`
	// Resolved: previous-round findings no longer present. New: findings not in
	// the previous round. Carried: present in both (matched by normalised title).
	Resolved int `json:"resolved"`
	New      int `json:"new"`
	Carried  int `json:"carried"`
}

// Trend is the ship-rate history from the committed dossiers.
type Trend struct {
	// Points are the last N closed cycles, oldest first.
	Points []TrendPoint `json:"points"`
	// Closed / Shipped are all-time counts over every dossier read.
	Closed  int `json:"closed"`
	Shipped int `json:"shipped"`
	// Ship rates over the most recent 20 / 50 closed cycles and all-time.
	ShipRateLast20 float64 `json:"ship_rate_last_20"`
	ShipRateLast50 float64 `json:"ship_rate_last_50"`
	ShipRateAll    float64 `json:"ship_rate_all"`
	// RoundHistogram counts closed cycles by audit-round count (index = rounds).
	RoundHistogram []RoundBucket `json:"round_histogram"`
}

// TrendPoint is one closed cycle's outcome.
type TrendPoint struct {
	Cycle   int    `json:"cycle"`
	Verdict string `json:"verdict"`
	Shipped bool   `json:"shipped"`
}

// RoundBucket is "cycles that needed N audit rounds: how many, how many shipped".
type RoundBucket struct {
	Rounds  int `json:"rounds"`
	Cycles  int `json:"cycles"`
	Shipped int `json:"shipped"`
}

// FingerprintStat groups FAIL cycles by failure identity.
type FingerprintStat struct {
	Fingerprint string `json:"fingerprint"`
	PreClass    string `json:"pre_class,omitempty"`
	Count       int    `json:"count"`
	FirstCycle  int    `json:"first_cycle"`
	LastCycle   int    `json:"last_cycle"`
	// Regressed is true when the fingerprint reappeared after a PASS cycle
	// that came later than one of its earlier occurrences.
	Regressed bool `json:"regressed"`
	// Reason is the first bounded reason recorded with the latest occurrence.
	Reason string `json:"reason,omitempty"`
}
