// Package gcpolicy is the zero-dependency leaf holding the
// `.evolve/policy.json` `gc` block — the retention engine's CONFIG
// vocabulary, split from the engine itself (internal/gc).
package gcpolicy

import "time"

type Policy struct {
	Mode            string     `json:"mode,omitempty"`
	Runs            RunsPolicy `json:"runs,omitempty"`
	SalvageTTLDays  int        `json:"salvage_ttl_days,omitempty"`
	LogsTTLDays     int        `json:"logs_ttl_days,omitempty"`
	TrackerTTLDays  int        `json:"tracker_ttl_days,omitempty"`
	GoCacheTTLHours int        `json:"go_cache_ttl_hours,omitempty"`
	GoCacheMaxGB    int        `json:"go_cache_max_gb,omitempty"`
	CacheTrimFreeGB int        `json:"cache_trim_free_gb,omitempty"`
	TempTTLHours    int        `json:"temp_ttl_hours,omitempty"`

	TestCacheTTLDays int `json:"testcache_ttl_days,omitempty"`
	TestCacheMaxMB   int `json:"testcache_max_mb,omitempty"`

	GuardsLogMaxMB          int    `json:"guards_log_max_mb,omitempty"`
	VerdictCacheKeepEntries int    `json:"verdict_cache_keep,omitempty"`
	LedgerSealKeepLines     int    `json:"ledger_seal_keep,omitempty"`
	LedgerSeal              string `json:"ledger_seal,omitempty"`

	Logs         LogsPolicy         `json:"logs,omitempty"`
	Worktrees    WorktreesPolicy    `json:"worktrees,omitempty"`
	DevWorktrees DevWorktreesPolicy `json:"dev_worktrees,omitempty"`
}

type RunsPolicy struct {
	KeepFull         int `json:"keep_full,omitempty"`
	ArchiveAfterDays int `json:"archive_after_days,omitempty"`
	DeleteAfterDays  int `json:"delete_after_days,omitempty"`
}

type WorktreesPolicy struct {
	KeepRecent        int `json:"keep_recent,omitempty"`
	MinAgeMinutes     int `json:"min_age_minutes,omitempty"`
	SalvageAfterHours int `json:"salvage_after_hours,omitempty"`
	DevQuietMinutes   int `json:"dev_quiet_minutes,omitempty"`
}

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

const (
	defaultGoCacheMaxGB = 20
	bytesPerGB          = int64(1e9)
)

func (p Policy) GoCacheMaxBytes() int64 {
	if p.GoCacheMaxGB <= 0 {
		return defaultGoCacheMaxGB * bytesPerGB
	}
	return int64(p.GoCacheMaxGB) * bytesPerGB
}

func (p Policy) CacheTrimFreeBytes() uint64 {
	if p.CacheTrimFreeGB <= 0 {
		return 0
	}
	return uint64(p.CacheTrimFreeGB) * uint64(bytesPerGB)
}

const (
	defaultTestCacheTTLDays = 14
	defaultTestCacheMaxMB   = 256
)

func (p Policy) TestCacheTTL() time.Duration {
	if p.TestCacheTTLDays <= 0 {
		return defaultTestCacheTTLDays * 24 * time.Hour
	}
	return time.Duration(p.TestCacheTTLDays) * 24 * time.Hour
}

func (p Policy) TestCacheMaxBytes() int64 {
	if p.TestCacheMaxMB <= 0 {
		return defaultTestCacheMaxMB * bytesPerMB
	}
	return int64(p.TestCacheMaxMB) * bytesPerMB
}

const (
	defaultDevQuietMinutes = 120
	minDevQuietMinutes     = 30
)

func (w WorktreesPolicy) DevQuietPeriod() time.Duration {
	switch {
	case w.DevQuietMinutes <= 0:
		return defaultDevQuietMinutes * time.Minute
	case w.DevQuietMinutes < minDevQuietMinutes:
		return minDevQuietMinutes * time.Minute
	}
	return time.Duration(w.DevQuietMinutes) * time.Minute
}

const (
	defaultDevArchiveAfterDays = 14
	defaultDevSweepBudgetSecs  = 300
)

type DevWorktreesPolicy struct {
	ArchiveAfterDays   int `json:"archive_after_days,omitempty"`
	SweepBudgetSeconds int `json:"sweep_budget_seconds,omitempty"`
}

func (d DevWorktreesPolicy) SweepBudget() time.Duration {
	secs := d.SweepBudgetSeconds
	if secs <= 0 {
		secs = defaultDevSweepBudgetSecs
	}
	return time.Duration(secs) * time.Second
}

func (d DevWorktreesPolicy) ArchiveAfter() time.Duration {
	days := d.ArchiveAfterDays
	if days <= 0 {
		days = defaultDevArchiveAfterDays
	}
	return time.Duration(days) * 24 * time.Hour
}
