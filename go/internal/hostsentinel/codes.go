package hostsentinel

import "github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"

const (
	CodeDiskLow      signalcenter.Code = "HOST_DISK_LOW"
	CodeCacheHigh    signalcenter.Code = "HOST_CACHE_HIGH"
	CodeTmuxHigh     signalcenter.Code = "HOST_TMUX_HIGH"
	CodeOrphansHigh  signalcenter.Code = "HOST_ORPHANS_HIGH"
	CodeLoadHigh     signalcenter.Code = "HOST_LOAD_HIGH"
	CodeFloorHold    signalcenter.Code = "HOST_FLOOR_HOLD"
	CodeSampleFailed signalcenter.Code = "HOST_SAMPLE_FAILED"
)

func init() {
	const tail = "; fields.value and fields.threshold give the reading and the limit, fields.stage is shadow (signal and a gc preview only) or enforce (the mid-wave-safe gc scopes run), fields.released names each scope as would, applied, failed or lease_held; one signal per code per interval"
	signalcenter.RegisterCode(signalcenter.ModuleHost, CodeDiskLow, "the host sentinel read disk free under host_sentinel.disk_low_gib during a wave; the release is the gc cache scope"+tail)
	signalcenter.RegisterCode(signalcenter.ModuleHost, CodeCacheHigh, "the host sentinel read a go build cache over the gc.go_cache_max_gb cap during a wave; the release is the gc cache scope"+tail)
	signalcenter.RegisterCode(signalcenter.ModuleHost, CodeTmuxHigh, "the host sentinel counted more tmux bridge sessions than host_sentinel.tmux_sessions_max during a wave; a session is boundary-only, so there is no mid-wave release"+tail)
	signalcenter.RegisterCode(signalcenter.ModuleHost, CodeOrphansHigh, "the host sentinel counted more finished-cycle orphan processes (ppid 1 in a closed cycle tree) than host_sentinel.orphans_max during a wave; the release is the gc orphans scope"+tail)
	signalcenter.RegisterCode(signalcenter.ModuleHost, CodeLoadHigh, "the host sentinel read a one-minute load per cpu over host_sentinel.load_per_cpu_max during a wave; there is no release, the signal is for the operator"+tail)
	signalcenter.RegisterCode(signalcenter.ModuleHost, CodeFloorHold, "the host sentinel read disk free under host_sentinel.floor_gib; at enforce no new lane starts (the launch defers) and the running lanes continue, at shadow fields.held=false names the hold it would make; fields.value, threshold, stage")
	signalcenter.RegisterCode(signalcenter.ModuleHost, CodeSampleFailed, "one or more host readings failed (fields.metrics); the sentinel warns, checks no threshold for those metrics and the loop continues; one signal per interval")
}
