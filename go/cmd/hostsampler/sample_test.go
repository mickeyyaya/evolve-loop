package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCompute_CPUBusyPercentFromDeltas(t *testing.T) {
	t0 := time.Unix(100, 0)
	prev := Counters{At: t0, HasCPU: true, CPUBusy: 100, CPUTotal: 1000}
	cur := Counters{At: t0.Add(time.Second), HasCPU: true, CPUBusy: 150, CPUTotal: 1100, HasMem: true, MemUsedMB: 512.5, HasSwap: true, SwapUsedMB: 3}
	got := compute(prev, cur)
	if got.CPUBusyPct == nil || *got.CPUBusyPct != 50 || *got.MemUsedMB != 512.5 || *got.SwapUsedMB != 3 {
		t.Fatalf("got %+v", got)
	}
	if got.Timestamp != cur.At.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("ts=%q", got.Timestamp)
	}
	if got.RunQueue != nil || got.DiskBusyPct != nil {
		t.Fatalf("optional fields must be absent: %+v", got)
	}
}

func TestCompute_NoCPUProgress_IsZero(t *testing.T) {
	prev := Counters{HasCPU: true, CPUBusy: 10, CPUTotal: 100}
	for _, cur := range []Counters{{HasCPU: true, CPUBusy: 10, CPUTotal: 100}, {HasCPU: true, CPUBusy: 1, CPUTotal: 50}} {
		if got := compute(prev, cur); got.CPUBusyPct == nil || *got.CPUBusyPct != 0 {
			t.Errorf("cur=%+v pct=%v", cur, got.CPUBusyPct)
		}
	}
}

func TestCompute_FieldsTheHostCannotGiveAreAbsent(t *testing.T) {
	has := Counters{HasCPU: true, CPUTotal: 1}
	for _, c := range [][2]Counters{{{}, {}}, {has, {}}, {{}, has}} {
		got := compute(c[0], c[1])
		if got.CPUBusyPct != nil || got.MemUsedMB != nil || got.SwapUsedMB != nil {
			t.Errorf("want absent: %+v", got)
		}
	}
}

func TestCompute_RunQueueAndDisk(t *testing.T) {
	t0 := time.Unix(0, 0)
	prev := Counters{At: t0, DiskTicksMs: 1000, DiskCount: 2}
	cur := Counters{At: t0.Add(time.Second), HasRunQueue: true, RunQueue: 4, DiskTicksMs: 2000, DiskCount: 2}
	got := compute(prev, cur)
	if got.RunQueue == nil || *got.RunQueue != 4 {
		t.Fatalf("runqueue=%v", got.RunQueue)
	}
	if got.DiskBusyPct == nil || *got.DiskBusyPct != 50 {
		t.Fatalf("disk=%v", got.DiskBusyPct)
	}
}

func TestCompute_DiskClampedAndAbsentWithoutDisks(t *testing.T) {
	t0 := time.Unix(0, 0)
	over := compute(Counters{At: t0, DiskCount: 1}, Counters{At: t0.Add(time.Second), DiskTicksMs: 5000, DiskCount: 1})
	if over.DiskBusyPct == nil || *over.DiskBusyPct != 100 {
		t.Fatalf("clamp: %v", over.DiskBusyPct)
	}
	none := compute(Counters{At: t0}, Counters{At: t0.Add(time.Second), DiskTicksMs: 5000})
	if none.DiskBusyPct != nil {
		t.Fatalf("no disks: %v", *none.DiskBusyPct)
	}
	stalled := compute(Counters{At: t0, DiskCount: 1}, Counters{At: t0, DiskTicksMs: 5, DiskCount: 1})
	if stalled.DiskBusyPct != nil {
		t.Fatalf("zero elapsed: %v", *stalled.DiskBusyPct)
	}
	regress := compute(Counters{At: t0, DiskTicksMs: 9, DiskCount: 1}, Counters{At: t0.Add(time.Second), DiskTicksMs: 5, DiskCount: 1})
	if regress.DiskBusyPct == nil || *regress.DiskBusyPct != 0 {
		t.Fatalf("regress: %v", regress.DiskBusyPct)
	}
}

func TestSample_JSONFieldNames(t *testing.T) {
	rq := 2
	d := 10.0
	one, two, three := 1.0, 2.0, 3.0
	raw, err := json.Marshal(Sample{Timestamp: "t", CPUBusyPct: &one, RunQueue: &rq, MemUsedMB: &two, SwapUsedMB: &three, DiskBusyPct: &d})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ts":"t","cpu_busy_pct":1,"runqueue":2,"mem_used_mb":2,"swap_used_mb":3,"disk_busy_pct":10}`
	if string(raw) != want {
		t.Fatalf("got %s", raw)
	}
	bare, _ := json.Marshal(Sample{})
	if string(bare) != `{"ts":""}` {
		t.Fatalf("bare %s", bare)
	}
}
