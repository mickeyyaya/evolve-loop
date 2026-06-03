package observer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writePIDFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x.bridge-pid")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProcessCPUProbe_AdvancingCPUIsAlive(t *testing.T) {
	pidFile := writePIDFile(t, "4242")
	samples := []string{"0:01.00", "0:01.00", "0:02.50"}
	i := 0
	run := func(pid int) (string, error) {
		if pid != 4242 {
			t.Fatalf("probe read pid %d, want 4242", pid)
		}
		s := samples[i]
		i++
		return "  " + s + "\n", nil // ps pads + newlines; probe must trim
	}
	probe := newProcessCPUProbe(pidFile, run)

	if !probe() {
		t.Error("first sighting must grant a window (true)")
	}
	if probe() {
		t.Error("unchanged CPU time → not alive (false)")
	}
	if !probe() {
		t.Error("advanced CPU time → alive (true)")
	}
}

func TestProcessCPUProbe_FailOpenPaths(t *testing.T) {
	t.Run("missing pidfile → false", func(t *testing.T) {
		probe := newProcessCPUProbe(filepath.Join(t.TempDir(), "absent"), func(int) (string, error) {
			t.Fatal("ps must not run when the pidfile is absent")
			return "", nil
		})
		if probe() {
			t.Error("absent pidfile must yield false")
		}
	})

	t.Run("unparseable pid → false", func(t *testing.T) {
		probe := newProcessCPUProbe(writePIDFile(t, "not-a-pid"), func(int) (string, error) { return "0:01", nil })
		if probe() {
			t.Error("unparseable pid must yield false")
		}
	})

	t.Run("ps error → false", func(t *testing.T) {
		probe := newProcessCPUProbe(writePIDFile(t, "5"), func(int) (string, error) { return "", errors.New("no such process") })
		if probe() {
			t.Error("ps error must yield false")
		}
	})

	t.Run("empty ps output → false", func(t *testing.T) {
		probe := newProcessCPUProbe(writePIDFile(t, "5"), func(int) (string, error) { return "   \n", nil })
		if probe() {
			t.Error("empty ps output must yield false")
		}
	})
}

func TestReadPID(t *testing.T) {
	if _, ok := readPID(""); ok {
		t.Error("empty path → not ok")
	}
	if pid, ok := readPID(writePIDFile(t, "  91234\n")); !ok || pid != 91234 {
		t.Errorf("readPID = (%d,%v), want (91234,true)", pid, ok)
	}
	if _, ok := readPID(writePIDFile(t, "0")); ok {
		t.Error("pid 0 → not ok")
	}
	if _, ok := readPID(writePIDFile(t, "-3")); ok {
		t.Error("negative pid → not ok")
	}
}

func TestAnyProbe(t *testing.T) {
	tru := func() bool { return true }
	fls := func() bool { return false }

	if anyProbe()() {
		t.Error("no probes → false")
	}
	if anyProbe(nil, nil)() {
		t.Error("all-nil → false")
	}
	if anyProbe(fls, fls)() {
		t.Error("all-false → false")
	}
	if !anyProbe(fls, tru)() {
		t.Error("any true → true")
	}

	// All sub-probes are consulted every call (no short-circuit), so stateful
	// probes keep consistent internal state.
	calls := 0
	counting := func() bool { calls++; return false }
	combined := anyProbe(tru, counting) // tru first would short-circuit a lazy OR
	combined()
	if calls != 1 {
		t.Errorf("counting probe consulted %d times, want 1 (no short-circuit)", calls)
	}
}
