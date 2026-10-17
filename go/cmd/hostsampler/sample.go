package main

import "time"

const (
	percent      = 100.0
	msPerSecond  = 1000.0
	maxDiskBusy  = 100.0
	timestampFmt = time.RFC3339Nano
)

type Counters struct {
	At          time.Time
	HasCPU      bool
	CPUBusy     uint64
	CPUTotal    uint64
	HasRunQueue bool
	RunQueue    int
	HasMem      bool
	MemUsedMB   float64
	HasSwap     bool
	SwapUsedMB  float64
	DiskTicksMs uint64
	DiskCount   int
}

type Sample struct {
	Timestamp   string   `json:"ts"`
	CPUBusyPct  *float64 `json:"cpu_busy_pct,omitempty"`
	RunQueue    *int     `json:"runqueue,omitempty"`
	MemUsedMB   *float64 `json:"mem_used_mb,omitempty"`
	SwapUsedMB  *float64 `json:"swap_used_mb,omitempty"`
	DiskBusyPct *float64 `json:"disk_busy_pct,omitempty"`
}

func compute(prev, cur Counters) Sample {
	s := Sample{
		Timestamp:  cur.At.UTC().Format(timestampFmt),
		CPUBusyPct: cpuBusyPct(prev, cur),
		MemUsedMB:  present(cur.HasMem, cur.MemUsedMB),
		SwapUsedMB: present(cur.HasSwap, cur.SwapUsedMB),
	}
	if cur.HasRunQueue {
		rq := cur.RunQueue
		s.RunQueue = &rq
	}
	s.DiskBusyPct = diskBusyPct(prev, cur)
	return s
}

func present(has bool, v float64) *float64 {
	if !has {
		return nil
	}
	return &v
}

func cpuBusyPct(prev, cur Counters) *float64 {
	if !prev.HasCPU || !cur.HasCPU {
		return nil
	}
	if cur.CPUTotal <= prev.CPUTotal || cur.CPUBusy < prev.CPUBusy {
		return present(true, 0)
	}
	return present(true, percent*float64(cur.CPUBusy-prev.CPUBusy)/float64(cur.CPUTotal-prev.CPUTotal))
}

func diskBusyPct(prev, cur Counters) *float64 {
	elapsedMs := cur.At.Sub(prev.At).Seconds() * msPerSecond
	if cur.DiskCount == 0 || elapsedMs <= 0 {
		return nil
	}
	var busy float64
	if cur.DiskTicksMs > prev.DiskTicksMs {
		busy = percent * float64(cur.DiskTicksMs-prev.DiskTicksMs) / (elapsedMs * float64(cur.DiskCount))
	}
	if busy > maxDiskBusy {
		busy = maxDiskBusy
	}
	return &busy
}
