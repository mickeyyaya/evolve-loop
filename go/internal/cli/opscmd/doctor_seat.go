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
	switch h.State {
	case seatlogin.StateNone, seatlogin.StateIsolated:
		fmt.Fprintf(stdout, "OK: %s\n", h.Message)
		return 0
	case seatlogin.StateBad:
		fmt.Fprintf(stderr, "WARN: %s\n", h.Message)
		return 1
	}
	fmt.Fprintf(stderr, "WARN: %s\n", h.Message)
	return 0
}
