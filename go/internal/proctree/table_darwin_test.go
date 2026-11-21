//go:build darwin

package proctree

import (
	"encoding/binary"
	"errors"
	"os"
	"slices"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

type fakeKern struct {
	table, single []byte
	args          []byte
	argsErr       error
	sizeErr       error
	enomem        int
	mibs          [][]int32
}

func (f *fakeKern) call(mib []int32, out *byte, size *uintptr) error {
	f.mibs = append(f.mibs, slices.Clone(mib))
	answer := f.answerFor(mib)
	if out == nil {
		*size = uintptr(len(answer))
		return f.sizeErr
	}
	if mib[1] == kernProcArgs2 && f.argsErr != nil {
		return f.argsErr
	}
	if f.enomem > 0 {
		f.enomem--
		return syscall.ENOMEM
	}
	if uintptr(len(answer)) > *size {
		return syscall.ENOMEM
	}
	*size = uintptr(copy(unsafe.Slice(out, *size), answer))
	return nil
}

func (f *fakeKern) answerFor(mib []int32) []byte {
	switch {
	case mib[1] == kernProcArgs2:
		return f.args
	case mib[2] == kernProcPID:
		return f.single
	}
	return f.table
}

func kinfoRow(pid, ppid, pgid int, start int64, comm string) []byte {
	raw := make([]byte, kinfoProcBytes)
	binary.LittleEndian.PutUint64(raw[0:8], uint64(start))
	binary.LittleEndian.PutUint32(raw[40:44], uint32(pid))
	copy(raw[243:243+16], comm)
	binary.LittleEndian.PutUint32(raw[560:564], uint32(ppid))
	binary.LittleEndian.PutUint32(raw[564:568], uint32(pgid))
	return raw
}

func selfRow(t *testing.T, comm string) []byte {
	t.Helper()
	started, err := StartTime(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	return kinfoRow(os.Getpid(), os.Getppid(), syscall.Getpgrp(), started.Unix(), comm)
}

func kernWithSelf(t *testing.T, others ...[]byte) *fakeKern {
	t.Helper()
	self := selfRow(t, "tester")
	return &fakeKern{table: slices.Concat(append([][]byte{self}, others...)...), single: self}
}

func TestKinfoLayout_MatchesThisProcess(t *testing.T) {
	t.Parallel()

	table, err := readTable(os.Getuid())

	if err != nil {
		t.Fatalf("readTable = %v, want the kinfo_proc layout of this macOS to match the offsets of the decoder", err)
	}
	self, found := rowOf(table, os.Getpid())
	if !found || self.Ppid != os.Getppid() || self.Pgid != syscall.Getpgrp() {
		t.Errorf("self row = %+v, %v, want ppid %d and pgid %d at offsets 560 and 564", self, found, os.Getppid(), syscall.Getpgrp())
	}
}

func TestReadTableVia_RefusesATableWhoseSelfRowDisagreesWithTheProcess(t *testing.T) {
	t.Parallel()
	started, err := StartTime(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	sec := started.Unix()
	cases := map[string][]byte{
		"ppid moved":      kinfoRow(os.Getpid(), os.Getppid()+1, syscall.Getpgrp(), sec, "x"),
		"pgid moved":      kinfoRow(os.Getpid(), os.Getppid(), syscall.Getpgrp()+1, sec, "x"),
		"start moved":     kinfoRow(os.Getpid(), os.Getppid(), syscall.Getpgrp(), sec+7, "x"),
		"self row absent": kinfoRow(os.Getpid()+1, os.Getppid(), syscall.Getpgrp(), sec, "x"),
	}
	for name, row := range cases {
		f := &fakeKern{table: row, single: kinfoRow(os.Getpid(), 0, 0, sec, "x")}

		got, err := readTableVia(f.call, os.Getuid())

		if !errors.Is(err, errKinfoLayout) || got != nil {
			t.Errorf("%s: readTableVia = %v, %v, want no rows and an error that names the kinfo_proc layout", name, got, err)
		}
	}
}

func TestReadTableVia_ASelfRecordWithAMovedPidOffsetIsRefused(t *testing.T) {
	t.Parallel()
	f := kernWithSelf(t)
	f.single = kinfoRow(os.Getpid()+1, 0, 0, 0, "x")

	got, err := readTableVia(f.call, os.Getuid())

	if !errors.Is(err, errKinfoLayout) || got != nil {
		t.Errorf("readTableVia = %v, %v, want the layout error: the self record must carry this pid", got, err)
	}
}

func TestReadTableVia_ASelfRecordTheKernelCannotGiveIsAnError(t *testing.T) {
	t.Parallel()
	f := kernWithSelf(t)
	f.single = nil

	got, err := readTableVia(f.call, os.Getuid())

	if !errors.Is(err, syscall.ESRCH) || got != nil {
		t.Errorf("readTableVia = %v, %v, want ESRCH for the missing self record", got, err)
	}
}

func TestReadTable_KeepsOnlyTheRowsOfTheGivenUID(t *testing.T) {
	t.Parallel()
	other := kinfoRow(777, 1, 777, 1760000000, "Google Chrome He")
	f := kernWithSelf(t, other)

	got, err := readTableVia(f.call, 4242)

	if err != nil {
		t.Fatal(err)
	}
	if want := []int32{ctlKern, kernProc, kernProcUID, 4242}; !slices.Equal(f.mibs[0], want) {
		t.Errorf("first sysctl mib = %v, want %v: the kernel filters by the effective uid", f.mibs[0], want)
	}
	row, found := rowOf(got, 777)
	if !found || row.Ppid != 1 || row.Pgid != 777 || row.Comm != "Google Chrome He" || !row.Started.Equal(time.Unix(1760000000, 0)) {
		t.Errorf("row 777 = %+v, %v, want the decoded pid, ppid, pgid, p_comm and the start second", row, found)
	}
}

func TestReadTableVia_RetriesWhenTheTableGrowsBetweenTheTwoCalls(t *testing.T) {
	t.Parallel()
	f := kernWithSelf(t)
	f.enomem = 2

	got, err := readTableVia(f.call, os.Getuid())

	if err != nil || len(got) != 1 {
		t.Errorf("readTableVia = %v, %v, want one row after two ENOMEM answers", got, err)
	}
}

func TestReadTableVia_APersistentENOMEMAndAFirstCallErrorFail(t *testing.T) {
	t.Parallel()
	growing := kernWithSelf(t)
	growing.enomem = 1 << 20
	refused := kernWithSelf(t)
	refused.sizeErr = syscall.EPERM

	if _, err := readTableVia(growing.call, os.Getuid()); !errors.Is(err, syscall.ENOMEM) {
		t.Errorf("a table that always grows = %v, want ENOMEM after the fixed count", err)
	}
	if _, err := readTableVia(refused.call, os.Getuid()); !errors.Is(err, syscall.EPERM) || len(refused.mibs) != 1 {
		t.Errorf("a refused sizing call = %v after %d calls, want EPERM at once", err, len(refused.mibs))
	}
}

func TestReadTableVia_AReadErrorOtherThanENOMEMReturnsAtOnce(t *testing.T) {
	t.Parallel()
	f := kernWithSelf(t)
	f.argsErr = nil
	broken := func(mib []int32, out *byte, size *uintptr) error {
		if out != nil {
			return syscall.EPERM
		}
		return f.call(mib, out, size)
	}

	got, err := readTableVia(broken, os.Getuid())

	if !errors.Is(err, syscall.EPERM) || got != nil {
		t.Errorf("readTableVia = %v, %v, want EPERM from the read call", got, err)
	}
}

func TestCommOfVia_FallsBackToPCommAndMapsAGonePid(t *testing.T) {
	t.Parallel()
	row := kinfoRow(4242, 1, 4242, 1760000000, "Google Chrome He")
	failing := &fakeKern{single: row, argsErr: syscall.EPERM}
	named := &fakeKern{single: row, args: []byte("\x01\x00\x00\x00/x/y\x00\x00renamedproc\x00")}
	blank := &fakeKern{single: row, args: []byte("\x01\x00\x00\x00/x/y\x00\x00\x00")}
	gone := &fakeKern{single: row[:kinfoProcBytes-1]}

	for name, c := range map[string]struct {
		kern *fakeKern
		want string
	}{"argument read fails": {failing, "Google Chrome He"}, "argv0 renamed": {named, "renamedproc"}, "argv0 empty": {blank, "Google Chrome He"}} {
		got, err := commOfVia(c.kern.call, 4242)

		if err != nil || got != c.want {
			t.Errorf("%s: commOfVia = %q, %v, want %q", name, got, err, c.want)
		}
	}
	if got, err := commOfVia(gone.call, 4242); !errors.Is(err, syscall.ESRCH) || got != "" {
		t.Errorf("a short record: commOfVia = %q, %v, want ESRCH", got, err)
	}
}

func TestCommFrom_IsArgv0AndFallsBackToPComm(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		p    Process
		want string
	}{
		"argv0":       {Process{Comm: "sleep", Args: []string{"renamedproc", "30"}}, "renamedproc"},
		"no args":     {Process{Comm: "Google Chrome He"}, "Google Chrome He"},
		"empty argv0": {Process{Comm: "sleep", Args: []string{"", "x"}}, "sleep"},
		"blank argv0": {Process{Comm: "sleep", Args: []string{"  ", "x"}}, "sleep"},
		"padded":      {Process{Comm: "postgres: wal", Args: []string{"postgres: walwriter   "}}, "postgres: walwriter"},
	}
	for name, c := range cases {
		if got := commFrom(c.p); got != c.want {
			t.Errorf("%s: commFrom = %q, want %q", name, got, c.want)
		}
	}
}

func TestReadTableVia_ATableThatGrowsBetweenSizingAndReadFitsTheFirstRead(t *testing.T) {
	t.Parallel()
	f := kernWithSelf(t, kinfoRow(901, 1, 901, 1760000000, "a"))
	reads := 0
	growing := func(mib []int32, out *byte, size *uintptr) error {
		err := f.call(mib, out, size)
		if out == nil && mib[2] == kernProcUID {
			*size -= kinfoProcBytes
		}
		if out != nil && mib[2] == kernProcUID {
			reads++
		}
		return err
	}

	got, err := readTableVia(growing, os.Getuid())

	if err != nil || len(got) != 2 || reads != 1 {
		t.Errorf("readTableVia = %d rows, %v after %d reads, want both rows on the first read", len(got), err, reads)
	}
}
