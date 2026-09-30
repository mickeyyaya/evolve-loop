package dashboard

import "time"

const (
	StateRunning    = "running"
	StatePass       = "pass"
	StateWarn       = "warn"
	StateFail       = "fail"
	StateHalted     = "halted"
	StateIncomplete = "incomplete"
)

type Snapshot struct {
	GeneratedAt  time.Time         `json:"generated_at"`
	Root         string            `json:"root"`
	Loop         LoopStatus        `json:"loop"`
	Queue        QueueSummary      `json:"queue"`
	Cycles       []CycleSummary    `json:"cycles"`
	Trend        Trend             `json:"trend"`
	Fingerprints []FingerprintStat `json:"fingerprints"`
	Warnings     []string          `json:"warnings,omitempty"`
}

type LoopStatus struct {
	Running        bool      `json:"running"`
	BrakeEngaged   bool      `json:"brake_engaged"`
	CycleID        int       `json:"cycle_id"`
	Phase          string    `json:"phase"`
	PhaseStartedAt time.Time `json:"phase_started_at"`
	LeaseHeartbeat time.Time `json:"lease_heartbeat"`
	ActiveWorktree string    `json:"active_worktree,omitempty"`
	CLI            string    `json:"cli,omitempty"`
	Model          string    `json:"model,omitempty"`
	AuditRounds    int       `json:"audit_rounds"`
	Checkpointed   bool      `json:"checkpointed,omitempty"`
}

type QueueSummary struct {
	Pending    []QueueItem `json:"pending"`
	Consumed   int         `json:"consumed"`
	Processing int         `json:"processing"`
	Retry      int         `json:"retry"`
	Processed  int         `json:"processed"`
}

type QueueItem struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Kind     string  `json:"kind,omitempty"`
	Class    string  `json:"class,omitempty"`
	Route    string  `json:"route,omitempty"`
	Priority string  `json:"priority,omitempty"`
	Weight   float64 `json:"weight"`
}

type CycleSummary struct {
	ID           int        `json:"id"`
	State        string     `json:"state"`
	StateName    string     `json:"state_name"`
	Verdict      string     `json:"verdict,omitempty"`
	CommitSHA    string     `json:"commit_sha,omitempty"`
	Goal         string     `json:"goal,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      time.Time  `json:"ended_at"`
	Phases       []PhaseRun `json:"phases"`
	AuditRounds  int        `json:"audit_rounds"`
	Tasks        []string   `json:"tasks,omitempty"`
	Failure      *Failure   `json:"failure,omitempty"`
	Tokens       int        `json:"tokens"`
	HasWorkspace bool       `json:"has_workspace"`
	HasDossier   bool       `json:"has_dossier"`
	CurrentPhase string     `json:"current_phase,omitempty"`
	Plan         *PhasePlan `json:"plan,omitempty"`
}

type PhasePlan struct {
	Mandatory         []string   `json:"mandatory"`
	Steps             []PlanStep `json:"steps"`
	Required          int        `json:"required"`
	PassedRequired    int        `json:"passed_required"`
	Total             int        `json:"total"`
	Passed            int        `json:"passed"`
	Ongoing           string     `json:"ongoing,omitempty"`
	OngoingSince      time.Time  `json:"ongoing_since"`
	Remaining         []string   `json:"remaining,omitempty"`
	AdvisorProposed   []string   `json:"advisor_proposed,omitempty"`
	AdvisorSkips      []string   `json:"advisor_skips,omitempty"`
	AdvisorOverridden []string   `json:"advisor_overridden,omitempty"`
}

type PlanStep struct {
	Phase        string `json:"phase"`
	Status       string `json:"status"`
	Optional     bool   `json:"optional,omitempty"`
	Conditional  bool   `json:"conditional,omitempty"`
	GateVerified bool   `json:"gate_verified,omitempty"`
	DurationMS   int64  `json:"duration_ms,omitempty"`
	Rounds       int    `json:"rounds,omitempty"`
}

type PhaseRun struct {
	Phase      string    `json:"phase"`
	Verdict    string    `json:"verdict"`
	Archetype  string    `json:"archetype,omitempty"`
	CLI        string    `json:"cli,omitempty"`
	Model      string    `json:"model,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
	DurationMS int64     `json:"duration_ms"`
	Attempt    int       `json:"attempt"`
	Round      int       `json:"round"`
	Tokens     int       `json:"tokens"`
}

type Failure struct {
	Category    string       `json:"category,omitempty"`
	Level       string       `json:"level,omitempty"`
	Action      string       `json:"action,omitempty"`
	FixType     string       `json:"fix_type,omitempty"`
	Fingerprint string       `json:"fingerprint,omitempty"`
	PreClass    string       `json:"pre_class,omitempty"`
	Legitimacy  string       `json:"legitimacy,omitempty"`
	Layer       string       `json:"layer,omitempty"`
	RootCause   string       `json:"root_cause,omitempty"`
	Salvage     string       `json:"salvage,omitempty"`
	Urgency     string       `json:"urgency,omitempty"`
	GateReasons []string     `json:"gate_reasons,omitempty"`
	Findings    []Finding    `json:"findings,omitempty"`
	Rounds      []AuditRound `json:"rounds,omitempty"`
}

type AuditRound struct {
	Round    int       `json:"round"`
	Verdict  string    `json:"verdict"`
	Findings []Finding `json:"findings"`
	Resolved int       `json:"resolved"`
	New      int       `json:"new"`
	Carried  int       `json:"carried"`
}

type Trend struct {
	Points         []TrendPoint  `json:"points"`
	Closed         int           `json:"closed"`
	Shipped        int           `json:"shipped"`
	ShipRateLast20 float64       `json:"ship_rate_last_20"`
	ShipRateLast50 float64       `json:"ship_rate_last_50"`
	ShipRateAll    float64       `json:"ship_rate_all"`
	RoundHistogram []RoundBucket `json:"round_histogram"`
}

type TrendPoint struct {
	Cycle   int    `json:"cycle"`
	Verdict string `json:"verdict"`
	Shipped bool   `json:"shipped"`
}

type RoundBucket struct {
	Rounds  int `json:"rounds"`
	Cycles  int `json:"cycles"`
	Shipped int `json:"shipped"`
}

type FingerprintStat struct {
	Fingerprint string `json:"fingerprint"`
	PreClass    string `json:"pre_class,omitempty"`
	Count       int    `json:"count"`
	FirstCycle  int    `json:"first_cycle"`
	LastCycle   int    `json:"last_cycle"`
	Regressed   bool   `json:"regressed"`
	Reason      string `json:"reason,omitempty"`
}
