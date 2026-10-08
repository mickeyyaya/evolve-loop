package gcpolicy

import (
	"path/filepath"
	"regexp"
	"time"
)

const (
	LogCategoryDispatch = "dispatch"
	LogCategoryLoop     = "loop"
	LogCategoryConsole  = "console"
	LogsDir             = "logs"
	LogsCurrentLink     = "current"
	LoopLogName         = "loop.log"
	LogWriterPIDSuffix  = ".writer-pid"
	LogRunDirGlob       = "[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]T[0-9][0-9][0-9][0-9][0-9][0-9]Z*"

	logRunIDLayout         = "20060102T150405Z"
	pollutedArchiveInfix   = ".polluted-"
	pollutedArchiveLayout  = "20060102T150405.000000000"
	bytesPerMB             = int64(1) << 20
	defaultDispatchCapMB   = 512
	defaultLoopCapMB       = 2048
	defaultConsoleCapMB    = 1024
	shipRepoContractScan   = "ship-repocontract-scan.log"
	integrationTierLogName = "integration-tier.log"
)

var logRunIDRe = regexp.MustCompile(`^\d{8}T\d{6}Z$`)

var pollutedArchiveRe = regexp.MustCompile(`^cycle-\d+\.polluted-\d{8}T\d{6}\.\d{9}$`)

type LogRetention struct {
	TTLDays    int `json:"ttl_days,omitempty"`
	MaxTotalMB int `json:"max_total_mb,omitempty"`
}

type LogsPolicy struct {
	Dispatch LogRetention `json:"dispatch,omitempty"`
	Loop     LogRetention `json:"loop,omitempty"`
	Console  LogRetention `json:"console,omitempty"`
}

type LogHome struct {
	Dir   string
	Glob  string
	IsDir bool
}

type LogCategory struct {
	Name          string
	Homes         []LogHome
	TTLDays       int
	MaxTotalBytes int64
}

func (p Policy) LogCatalog() []LogCategory {
	ttl := p.WithDefaults().LogsTTLDays
	return []LogCategory{
		resolveCategory(LogCategoryDispatch, p.Logs.Dispatch, ttl, defaultDispatchCapMB,
			LogHome{Dir: "dispatch-logs", Glob: "*.log"}),
		resolveCategory(LogCategoryLoop, p.Logs.Loop, ttl, defaultLoopCapMB,
			LogHome{Dir: LogsDir, Glob: LogRunDirGlob, IsDir: true}),
		resolveCategory(LogCategoryConsole, p.Logs.Console, ttl, defaultConsoleCapMB,
			LogHome{Dir: ".", Glob: "loop-*.log"}, LogHome{Dir: ".", Glob: "wave*.log"}, LogHome{Dir: LogsDir, Glob: "batch-*.log"}),
	}
}

func resolveCategory(name string, r LogRetention, defaultTTL, defaultCapMB int, homes ...LogHome) LogCategory {
	ttl, capMB := defaultTTL, defaultCapMB
	if r.TTLDays > 0 {
		ttl = r.TTLDays
	}
	if r.MaxTotalMB > 0 {
		capMB = r.MaxTotalMB
	}
	return LogCategory{Name: name, Homes: homes, TTLDays: ttl, MaxTotalBytes: int64(capMB) * bytesPerMB}
}

func ToolOutputFiles() []string {
	return []string{shipRepoContractScan, integrationTierLogName}
}

func LogRunID(at time.Time) string { return at.UTC().Format(logRunIDLayout) }

func IsLogRunDir(name string) bool {
	ok, err := filepath.Match(LogRunDirGlob, name)
	return err == nil && ok
}

func PollutedArchiveName(at time.Time) string {
	return pollutedArchiveInfix + at.UTC().Format(pollutedArchiveLayout)
}

func IsPollutedArchive(name string) bool { return pollutedArchiveRe.MatchString(name) }

func IsLogRunID(name string) bool { return logRunIDRe.MatchString(name) }
