//go:build linux

package proctree

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const bootAndStat = "cpu 1 2 3\nbtime 1700000000\n"

func statOf(pid int, comm string, ppid, pgrp, ticks int) string {
	return strings.Join([]string{strconv.Itoa(pid), "(" + comm + ")", "S", strconv.Itoa(ppid), strconv.Itoa(pgrp), "4242 0 -1 4194560 100 0 0 0 1 2 0 0 20 0 1 0", strconv.Itoa(ticks), "1000 10\n"}, " ")
}

func statusOf(real, effective int) string {
	return "Name:\tx\nUid:\t" + strconv.Itoa(real) + "\t" + strconv.Itoa(effective) + "\t0\t0\n"
}

func TestReadTableAt_SkipsAPidThatVanishesAndKeepsACommWithSpacesAndParens(t *testing.T) {
	t.Parallel()
	root := procRoot(t, map[string]string{
		"stat":      bootAndStat,
		"11/stat":   statOf(11, "a b) c", 1, 11, 500),
		"11/status": statusOf(5, 5),
		"12/stat":   statOf(12, "gone", 1, 12, 1),
		"12/status": statusOf(5, 5),
		"13/stat":   statOf(13, "esrch", 1, 13, 1),
		"13/status": statusOf(5, 5),
		"14/stat":   statOf(14, "denied", 1, 14, 1),
		"14/status": statusOf(5, 5),
	})
	denied := map[string]error{"12": fs.ErrNotExist, "13": syscall.ESRCH, "14": syscall.EACCES}
	read := func(path string) ([]byte, error) {
		if err, ok := denied[filepath.Base(filepath.Dir(path))]; ok {
			return nil, err
		}
		return os.ReadFile(path)
	}

	got, err := readTableAt(root, read, 5)

	want := Process{Pid: 11, Ppid: 1, Pgid: 11, Started: time.Unix(1700000000+5, 0), Comm: "a b) c"}
	if err != nil || len(got) != 1 || got[0].Pid != want.Pid || got[0].Comm != want.Comm || got[0].Ppid != 1 || got[0].Pgid != 11 || !got[0].Started.Equal(want.Started) {
		t.Errorf("readTableAt = %+v, %v, want only %+v", got, err, want)
	}
}

func TestReadTableAt_AMalformedStatOrStatusIsAnError(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]map[string]string{
		"stat":   {"stat": bootAndStat, "7/stat": "7 (x) S 1\n", "7/status": statusOf(5, 5)},
		"status": {"stat": bootAndStat, "7/stat": statOf(7, "x", 1, 7, 1), "7/status": "Name:\tx\n"},
		"boot":   {"stat": "cpu 1\n", "7/stat": statOf(7, "x", 1, 7, 1), "7/status": statusOf(5, 5)},
	} {
		got, err := readTableAt(procRoot(t, files), os.ReadFile, 5)

		if err == nil || got != nil {
			t.Errorf("%s: readTableAt = %v, %v, want an error and no rows", name, got, err)
		}
	}
}

func TestReadTableAt_KeepsOnlyTheRowsWhoseEffectiveUIDIsTheGivenUID(t *testing.T) {
	t.Parallel()
	root := procRoot(t, map[string]string{
		"stat":      bootAndStat,
		"21/stat":   statOf(21, "mine", 1, 21, 1),
		"21/status": statusOf(9, 5),
		"22/stat":   statOf(22, "setuid", 1, 22, 1),
		"22/status": statusOf(5, 9),
		"23/stat":   statOf(23, "foreign", 1, 23, 1),
		"23/status": statusOf(9, 9),
	})

	got, err := readTableAt(root, os.ReadFile, 5)

	if err != nil || len(got) != 1 || got[0].Pid != 21 {
		t.Errorf("readTableAt = %+v, %v, want only pid 21: the effective uid decides, as for ps", got, err)
	}
}

func TestCommOfAt_ReadsStatCommAndAGonePidIsESRCH(t *testing.T) {
	t.Parallel()
	root := procRoot(t, map[string]string{"31/stat": statOf(31, "a (b) c", 1, 31, 1)})

	got, err := commOfAt(root, 31)
	_, goneErr := commOfAt(root, 32)

	if err != nil || got != "a (b) c" || !errors.Is(goneErr, syscall.ESRCH) {
		t.Errorf("commOfAt = %q, %v and gone = %v, want the stat comm and ESRCH", got, err, goneErr)
	}
}

func TestCommFrom_IsTheStatComm(t *testing.T) {
	t.Parallel()
	if got := commFrom(Process{Comm: "sleep", Args: []string{"renamedproc"}}); got != "sleep" {
		t.Errorf("commFrom = %q, want the stat comm: Linux ps prints it", got)
	}
}

func TestReadTableAt_AReadErrorThatIsNotAVanishedPidIsReturned(t *testing.T) {
	t.Parallel()
	root := procRoot(t, map[string]string{"stat": bootAndStat, "8/stat": statOf(8, "x", 1, 8, 1), "8/status": statusOf(5, 5)})
	for _, failing := range []string{"status", "stat"} {
		read := func(path string) ([]byte, error) {
			if filepath.Base(path) == failing {
				return nil, syscall.EIO
			}
			return os.ReadFile(path)
		}

		got, err := readTableAt(root, read, 5)

		if !errors.Is(err, syscall.EIO) || got != nil {
			t.Errorf("%s unreadable: readTableAt = %v, %v, want EIO and no rows", failing, got, err)
		}
	}
}

func TestReadTableAt_ARootThatCannotBeListedIsAnError(t *testing.T) {
	t.Parallel()

	got, err := readTableAt(filepath.Join(t.TempDir(), "gone"), os.ReadFile, 5)

	if !errors.Is(err, fs.ErrNotExist) || got != nil {
		t.Errorf("readTableAt = %v, %v, want a not-exist error", got, err)
	}
}

func TestReadTableAt_NonNumericFieldsAreMalformed(t *testing.T) {
	t.Parallel()
	good := statOf(9, "x", 1, 9, 1)
	for name, files := range map[string]map[string]string{
		"ppid":  {"9/stat": strings.Replace(good, ") S 1 9", ") S x 9", 1), "9/status": statusOf(5, 5)},
		"ticks": {"9/stat": strings.Replace(good, "20 0 1 0 1 1000", "20 0 1 0 x 1000", 1), "9/status": statusOf(5, 5)},
		"uid":   {"9/stat": good, "9/status": "Uid:\tx\ty\t0\t0\n"},
		"short": {"9/stat": good, "9/status": "Uid:\t5\n"},
	} {
		files["stat"] = bootAndStat

		got, err := readTableAt(procRoot(t, files), os.ReadFile, 5)

		if err == nil || got != nil {
			t.Errorf("%s: readTableAt = %v, %v, want an error and no rows", name, got, err)
		}
	}
}

func TestCommOfAt_AnUnreadableOrMalformedStatIsAnError(t *testing.T) {
	t.Parallel()
	root := procRoot(t, map[string]string{"41/stat/x": "", "42/stat": "42 (x) S 1\n"})

	_, unreadable := commOfAt(root, 41)
	_, malformed := commOfAt(root, 42)

	if unreadable == nil || errors.Is(unreadable, syscall.ESRCH) || !errors.Is(malformed, errProcStatMalformed) {
		t.Errorf("errors = %v, %v, want a read error (not ESRCH) and a malformed-stat error", unreadable, malformed)
	}
}

func TestParseStat_ANonNumericPpidOrPgrpIsMalformed(t *testing.T) {
	t.Parallel()
	for _, stat := range []string{"1 (x) S p 1 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 5 0 0", "1 (x) S 1 q 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 5 0 0", "1 x) S 1 1", "1 )x( S 1 1"} {
		if _, err := parseStat(stat); !errors.Is(err, errProcStatMalformed) {
			t.Errorf("parseStat(%q) = %v, want the malformed-stat error", stat, err)
		}
	}
}
