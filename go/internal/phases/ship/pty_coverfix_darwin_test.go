//go:build darwin

package ship

import (
	"os"
	"syscall"
	"testing"
	"unsafe"
)

const (
	coverfixTIOCPTYGRANT = 0x20007454
	coverfixTIOCPTYUNLK  = 0x20007452
	coverfixTIOCPTYGNAME = 0x40807453
)

func coverfixIoctl(t *testing.T, fd uintptr, req, arg uintptr) {
	t.Helper()
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg); errno != 0 {
		t.Fatalf("ioctl %#x: %v", req, errno)
	}
}

func openPTYForCoverfix(t *testing.T) (master, slave *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no pty available: %v", err)
	}
	t.Cleanup(func() { _ = master.Close() })
	coverfixIoctl(t, master.Fd(), coverfixTIOCPTYGRANT, 0)
	coverfixIoctl(t, master.Fd(), coverfixTIOCPTYUNLK, 0)
	var name [128]byte
	coverfixIoctl(t, master.Fd(), coverfixTIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0])))
	n := 0
	for n < len(name) && name[n] != 0 {
		n++
	}
	slave, err = os.OpenFile(string(name[:n]), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open pty slave: %v", err)
	}
	t.Cleanup(func() { _ = slave.Close() })
	return master, slave
}
