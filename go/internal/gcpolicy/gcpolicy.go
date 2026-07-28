// Package gcpolicy is the zero-dependency leaf holding the
// `.evolve/policy.json` `gc` block — the retention engine's CONFIG vocabulary,
// split from the engine itself (internal/gc).
//
// Why the split (cycle-1141, decoupling campaign — same shape as cyclestate
// and shiperr): internal/policy, the config SSOT, held `GC *gc.Policy`, i.e.
// config depended on an ENGINE. That inverted edge closed a real cycle the
// moment gc needed the phasecontract registry
// (policy → gc → phasecontract → phasespec → policy). Config now points at a
// leaf that points at nothing, and the engine re-exports these types as
// aliases so every existing `gc.Policy` call site is untouched.
package gcpolicy

// Policy is the `.evolve/policy.json` `gc` block. The zero value means
// "defaults": see WithDefaults. A zero ArchiveAfterDays/DeleteAfterDays
// disables that action entirely — retention never escalates by default.
type Policy struct {
	// Mode controls whether the GC hook runs. off = disabled; shadow
	// (the default an ABSENT mode resolves to in runGCHook, workspace-hygiene
	// S5) = discover+plan+log manifest without mutations; enforce =
	// shadow+apply, always an explicit operator decision.
	Mode string     `json:"mode,omitempty"`
	Runs RunsPolicy `json:"runs,omitempty"`
	// SalvageTTLDays prunes <evolve>/operator-salvage entries. Default 30.
	SalvageTTLDays int `json:"salvage_ttl_days,omitempty"`
	// LogsTTLDays prunes <evolve>/dispatch-logs/*.log. Default 30.
	LogsTTLDays int `json:"logs_ttl_days,omitempty"`
	// TrackerTTLDays prunes <run-dir>/.ephemeral subtrees of KEPT runs.
	// Default 7 (mirrors pruneephemeral).
	TrackerTTLDays int `json:"tracker_ttl_days,omitempty"`
	// Worktrees is the retention grace for the worktree+branch backlog sweep
	// (S4); consumed by PlanWorktrees. Zero value = no KeepRecent/MinAge grace.
	Worktrees WorktreesPolicy `json:"worktrees,omitempty"`
}

// RunsPolicy is the retention ladder for run directories.
type RunsPolicy struct {
	// KeepFull: the newest N run dirs (by mtime, live or not — live runs are
	// protected independently of this count) are always kept in full,
	// however old. Default 10.
	KeepFull int `json:"keep_full,omitempty"`
	// ArchiveAfterDays: beyond KeepFull, a dead run STRICTLY older than this
	// is moved under <evolve>/archive/runs/. 0 = never archive.
	ArchiveAfterDays int `json:"archive_after_days,omitempty"`
	// DeleteAfterDays: beyond KeepFull, a dead run STRICTLY older than this
	// is deleted. 0 = never delete. Delete wins over archive when both match.
	DeleteAfterDays int `json:"delete_after_days,omitempty"`
}

// WorktreesPolicy is the retention grace on top of the merged/clean/dead gate,
// consumed by the worktree backlog sweep.
type WorktreesPolicy struct {
	// KeepRecent: among fully-eligible (merged, clean, dead) candidates, the
	// newest N by mtime are always kept, mirroring RunsPolicy.KeepFull.
	KeepRecent int `json:"keep_recent,omitempty"`
	// MinAgeMinutes: a candidate younger than this is never touched — the grace
	// window that covers the create -> lease-write race.
	MinAgeMinutes int `json:"min_age_minutes,omitempty"`
}

// WithDefaults returns a copy of p with every zero-value retention knob
// replaced by its built-in default. Deliberately does NOT default the
// Archive/Delete day counts: zero there means "never", and a retention engine
// must never invent a deletion horizon the operator did not ask for.
func (p Policy) WithDefaults() Policy {
	if p.Runs.KeepFull == 0 {
		p.Runs.KeepFull = 10
	}
	if p.SalvageTTLDays == 0 {
		p.SalvageTTLDays = 30
	}
	if p.LogsTTLDays == 0 {
		p.LogsTTLDays = 30
	}
	if p.TrackerTTLDays == 0 {
		p.TrackerTTLDays = 7
	}
	return p
}
