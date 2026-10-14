//go:build linux

package ship

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

const (
	coverfixTIOCGPTN   = 0x80045430
	coverfixTIOCSPTLCK = 0x40045431
)

func openPTYForCoverfix(t *testing.T) (master, slave *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no pty available: %v", err)
	}
	t.Cleanup(func() { _ = master.Close() })
	var unlock int32
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), coverfixTIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); errno != 0 {
		t.Fatalf("unlock pty: %v", errno)
	}
	var num uint32
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), coverfixTIOCGPTN, uintptr(unsafe.Pointer(&num))); errno != 0 {
		t.Fatalf("pty number: %v", errno)
	}
	slave, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", num), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open pty slave: %v", err)
	}
	t.Cleanup(func() { _ = slave.Close() })
	return master, slave
}
