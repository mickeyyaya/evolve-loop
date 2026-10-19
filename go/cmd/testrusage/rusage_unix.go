//go:build unix

package main

import (
	"os"
	"syscall"
)

func usageOf(ps *os.ProcessState, goos string) (usage, bool) {
	ru, ok := ps.SysUsage().(*syscall.Rusage)
	if !ok || ru == nil {
		return usage{}, false
	}
	return usage{
		UserS:          tvSeconds(ru.Utime),
		SystemS:        tvSeconds(ru.Stime),
		PeakRSSMB:      peakRSSMB(int64(ru.Maxrss), goos),
		BlockIn:        int64(ru.Inblock),
		BlockOut:       int64(ru.Oublock),
		CtxVoluntary:   int64(ru.Nvcsw),
		CtxInvoluntary: int64(ru.Nivcsw),
	}, true
}

func tvSeconds(tv syscall.Timeval) float64 {
	return float64(tv.Sec) + float64(tv.Usec)*secondsPerMicro
}
