package opscmd

import (
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
)

var doctorSeatInspect = func() seatlogin.Health {
	home, _ := os.UserHomeDir()
	return seatlogin.Inspect(home, seatlogin.OS())
}

func runDoctorSeat(stdout, stderr io.Writer) int {
	h := doctorSeatInspect()
	switch h.State.Severity() {
	case seatlogin.SeverityOK:
		fmt.Fprintf(stdout, "OK: %s\n", h.Message)
		return 0
	case seatlogin.SeverityWarn:
		fmt.Fprintf(stderr, "WARN: %s\n", h.Message)
		return 0
	case seatlogin.SeverityHalt:
		fmt.Fprintf(stderr, "WARN: %s\n", h.Message)
		return 1
	}
	fmt.Fprintf(stderr, "WARN: unknown seat state: %s\n", h.Message)
	return 1
}
