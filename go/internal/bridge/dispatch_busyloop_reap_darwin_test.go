//go:build darwin

package bridge

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

const (
	ctlKern         = 1
	kernProc        = 14
	kernProcAll     = 0
	kernProcArgs2   = 49
	kinfoProcBytes  = 648
	offsetStartTime = 0
	offsetPid       = 40
	offsetComm      = 243
	offsetUid       = 420
	offsetPpid      = 560
	offsetPgid      = 564
)

func darwinSysctlListProcs(uid int) ([]proctree.Process, error) {
	mib := []int32{ctlKern, kernProc, kernProcAll}
	var size uintptr
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)), 0, uintptr(unsafe.Pointer(&size)), 0, 0)
	if errno != 0 {
		return nil, errno
	}
	buf := make([]byte, size)
	_, _, errno = syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0, 0)
	if errno != 0 {
		return nil, errno
	}
	count := int(size) / kinfoProcBytes
	procs := make([]proctree.Process, 0, count)
	for i := 0; i < count; i++ {
		entry := buf[i*kinfoProcBytes : (i+1)*kinfoProcBytes]
		procUid := int(binary.LittleEndian.Uint32(entry[offsetUid : offsetUid+4]))
		if procUid != uid {
			continue
		}
		sec := int64(binary.LittleEndian.Uint64(entry[offsetStartTime : offsetStartTime+8]))
		usec := int64(binary.LittleEndian.Uint32(entry[offsetStartTime+8 : offsetStartTime+12]))
		started := time.Unix(sec, usec*1000)
		pid := int(int32(binary.LittleEndian.Uint32(entry[offsetPid : offsetPid+4])))
		ppid := int(int32(binary.LittleEndian.Uint32(entry[offsetPpid : offsetPpid+4])))
		pgid := int(int32(binary.LittleEndian.Uint32(entry[offsetPgid : offsetPgid+4])))
		commBytes := entry[offsetComm : offsetComm+16]
		if nullIdx := bytes.IndexByte(commBytes, 0); nullIdx != -1 {
			commBytes = commBytes[:nullIdx]
		}
		comm := string(commBytes)
		procs = append(procs, proctree.Process{
			Pid: pid, Ppid: ppid, Pgid: pgid, Started: started, Comm: comm,
		})
	}
	return procs, nil
}

func fallbackLister(l proctree.Lister) proctree.Lister {
	return func(ctx context.Context) ([]proctree.Process, error) {
		procs, err := l(ctx)
		if err == nil {
			return procs, nil
		}
		return darwinSysctlListProcs(os.Getuid())
	}
}

func darwinProcArgsListing() ([]byte, error) {
	procs, err := darwinSysctlListProcs(os.Getuid())
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	for _, p := range procs {
		mib := []int32{ctlKern, kernProcArgs2, int32(p.Pid)}
		var size uintptr
		if _, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)), 0, uintptr(unsafe.Pointer(&size)), 0, 0); errno != 0 || size == 0 {
			continue
		}
		buf := make([]byte, size)
		if _, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0, 0); errno != 0 {
			continue
		}
		out.Write(bytes.ReplaceAll(buf, []byte{0}, []byte{' '}))
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

func psArgsOrFallback() ([]byte, error) {
	out, err := exec.Command("ps", "-axww", "-o", "args").Output()
	if err == nil {
		return out, nil
	}
	return darwinProcArgsListing()
}
