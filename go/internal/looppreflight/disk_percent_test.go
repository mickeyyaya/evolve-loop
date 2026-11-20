package looppreflight

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDiskGuard_DefaultConfigAndThresholds(t *testing.T) {
	cfg := DefaultDiskGuardConfig()
	if cfg.WarnPercent != 10 || cfg.PausePercent != 5 {
		t.Fatalf("DefaultDiskGuardConfig = %+v, want WarnPercent=10 PausePercent=5", cfg)
	}

	const total = uint64(100) << 30
	pct := 20.0
	var warn bytes.Buffer
	var g *DiskGuard = NewDiskGuard(".", DiskGuardConfig{}, func(string) (DiskUsage, error) {
		return DiskUsage{FreeBytes: uint64(pct * float64(total) / 100), TotalBytes: total}, nil
	}, &warn)

	if g.Check() {
		t.Fatal("Check() at 20% = true, want false")
	}
	if warn.Len() != 0 {
		t.Fatalf("warn at 20%% = %q, want empty", warn.String())
	}

	pct = 9.0
	if g.Check() {
		t.Fatal("Check() at 9% = true, want false")
	}
	if !strings.Contains(warn.String(), "WARN") {
		t.Fatalf("warn at 9%% = %q, want WARN line", warn.String())
	}

	warn.Reset()
	pct = 4.0
	if !g.Check() {
		t.Fatal("Check() at 4% = false, want true (paused)")
	}
	if !strings.Contains(warn.String(), "WARN") {
		t.Fatalf("warn at 4%% = %q, want WARN line", warn.String())
	}

	warn.Reset()
	pct = 5.0
	if g.Check() {
		t.Fatal("Check() at 5% = true, want false (resumed at threshold)")
	}
	if !strings.Contains(warn.String(), "WARN") {
		t.Fatalf("warn at 5%% = %q, want WARN line below 10%%", warn.String())
	}
}

func TestDiskGuard_ReadFailureAndZeroTotal(t *testing.T) {
	var warn bytes.Buffer
	readErr := errors.New("probe failed")
	g := NewDiskGuard(".", DefaultDiskGuardConfig(), func(string) (DiskUsage, error) {
		return DiskUsage{}, readErr
	}, &warn)
	if g.Check() {
		t.Fatal("Check() on error = true, want false")
	}
	if !strings.Contains(warn.String(), "WARN") || !strings.Contains(warn.String(), "probe failed") {
		t.Fatalf("warn on error = %q, want WARN naming error", warn.String())
	}

	warn.Reset()
	gZero := NewDiskGuard(".", DefaultDiskGuardConfig(), func(string) (DiskUsage, error) {
		return DiskUsage{FreeBytes: 0, TotalBytes: 0}, nil
	}, &warn)
	if gZero.Check() {
		t.Fatal("Check() on zero total = true, want false")
	}
	if !strings.Contains(warn.String(), "WARN") {
		t.Fatalf("warn on zero total = %q, want WARN", warn.String())
	}
}

func TestDiskGuard_RunCycleStartTrimOrderAndCancel(t *testing.T) {
	const total = uint64(100) << 30
	seq := []float64{4, 6}
	idx := 0
	var warn bytes.Buffer
	var order []string
	guard := NewDiskGuard(".", DefaultDiskGuardConfig(), func(string) (DiskUsage, error) {
		order = append(order, "check")
		return DiskUsage{FreeBytes: uint64(seq[idx] * float64(total) / 100), TotalBytes: total}, nil
	}, &warn)

	err := RunCycleStart(context.Background(), CycleStartOptions{
		Trim: func() error {
			order = append(order, "trim")
			return errors.New("trim oops")
		},
		Guard: guard,
		Wait: func(context.Context) error {
			order = append(order, "wait")
			idx++
			return nil
		},
		Warn: &warn,
	})
	if err != nil {
		t.Fatalf("RunCycleStart = %v, want nil", err)
	}
	if len(order) < 2 || order[0] != "trim" {
		t.Fatalf("order = %v, want trim first", order)
	}
	if !strings.Contains(warn.String(), "trim oops") {
		t.Fatalf("warn = %q, want trim oops", warn.String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	idx = 0
	err = RunCycleStart(ctx, CycleStartOptions{Guard: guard})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunCycleStart with cancelled ctx and nil Wait = %v, want context.Canceled", err)
	}
}

func TestDiskUsageOf_MeasuresRealPathAndRejectsMissing(t *testing.T) {
	u, err := DiskUsageOf(t.TempDir())
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		if err == nil {
			t.Fatal("expected unsupported error off darwin/linux")
		}
		return
	}
	if err != nil {
		t.Fatalf("DiskUsageOf(temp dir) = %v", err)
	}
	if u.TotalBytes == 0 || u.FreeBytes > u.TotalBytes {
		t.Fatalf("DiskUsageOf = %+v, want non-zero TotalBytes and FreeBytes <= TotalBytes", u)
	}
	if _, err := DiskUsageOf(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("DiskUsageOf(missing) = nil, want error")
	}
}

func TestDiskGuard_NilDefaultsAndCycleStartNilGuardAndWait(t *testing.T) {
	var g *DiskGuard = NewDiskGuard(t.TempDir(), DefaultDiskGuardConfig(), nil, nil)
	_ = g.Check()

	trimmed := false
	if err := RunCycleStart(context.Background(), CycleStartOptions{
		Trim: func() error {
			trimmed = true
			return nil
		},
	}); err != nil {
		t.Fatalf("RunCycleStart with nil Guard = %v, want nil", err)
	}
	if !trimmed {
		t.Fatal("RunCycleStart with nil Guard did not run Trim")
	}

	if err := waitForInterval(context.Background(), time.Millisecond); err != nil {
		t.Fatalf("waitForInterval = %v, want nil", err)
	}
}

func TestDiskStatus_MeasureReportsPausedFreePctThresholdAndPath(t *testing.T) {
	const total = uint64(200) << 30
	free := total / 25
	g := NewDiskGuard("/vol/work", DiskGuardConfig{}, func(string) (DiskUsage, error) {
		return DiskUsage{FreeBytes: free, TotalBytes: total}, nil
	}, nil)

	st := g.Measure()
	want := DiskStatus{Paused: true, FreePct: 4, PausePct: 5, Path: "/vol/work"}
	if st != want {
		t.Fatalf("Measure() at 4%% free = %+v, want %+v", st, want)
	}

	free = total / 2
	want = DiskStatus{Paused: false, FreePct: 50, PausePct: 5, Path: "/vol/work"}
	if st = g.Measure(); st != want {
		t.Fatalf("Measure() at 50%% free = %+v, want %+v", st, want)
	}
}
