// Command testrusage is a go test -exec wrapper that runs one test binary and appends its resource record to TESTRUSAGE_OUT.
// See docs/architecture/packages/cmd-testrusage.md.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	outEnv            = "TESTRUSAGE_OUT"
	exitStartFailure  = 127
	exitWaitFailure   = 1
	usageNote         = "rusage: unavailable"
	exitUsage         = 2
	signalExitOffset  = 128
	bytesPerMB        = 1 << 20
	kilobytesPerMB    = 1 << 10
	secondsPerMicro   = 1e-6
	modeAppendCreate  = os.O_WRONLY | os.O_APPEND | os.O_CREATE
	recordPermissions = 0o644
)

var (
	exitProcess = os.Exit
	waitCmd     = (*exec.Cmd).Wait
	openAppend  = func(path string) (io.WriteCloser, error) {
		return os.OpenFile(path, modeAppendCreate, recordPermissions)
	}
)

type Record struct {
	Package        string  `json:"package"`
	WallS          float64 `json:"wall_s"`
	UserS          float64 `json:"user_s"`
	SystemS        float64 `json:"system_s"`
	PeakRSSMB      float64 `json:"peak_rss_mb"`
	BlockIn        int64   `json:"block_in"`
	BlockOut       int64   `json:"block_out"`
	CtxVoluntary   int64   `json:"ctx_voluntary"`
	CtxInvoluntary int64   `json:"ctx_involuntary"`
	ExitCode       int     `json:"exit_code"`
	Note           string  `json:"note,omitempty"`
}

type usage struct {
	UserS, SystemS, PeakRSSMB                       float64
	BlockIn, BlockOut, CtxVoluntary, CtxInvoluntary int64
}

func main() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	exitProcess(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, sig))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, sig <-chan os.Signal) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: testrusage <test-binary> [args...]")
		return exitUsage
	}
	cmd := sysexec.Command(context.Background(), args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	start := time.Now()
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(stderr, "testrusage: %v\n", err)
		return exitStartFailure
	}
	done := make(chan struct{})
	go forward(cmd.Process, sig, done)
	waitErr := waitCmd(cmd)
	close(done)
	wall := time.Since(start)
	if cmd.ProcessState == nil {
		fmt.Fprintf(stderr, "testrusage: %v\n", waitErr)
		return exitWaitFailure
	}
	code := exitCodeOf(cmd.ProcessState)
	if path := os.Getenv(outEnv); path != "" {
		if err := appendRecord(path, buildRecord(cmd.ProcessState, wall, code)); err != nil {
			fmt.Fprintf(stderr, "testrusage: %v\n", err)
		}
	}
	if _, isExit := waitErr.(*exec.ExitError); waitErr != nil && !isExit {
		fmt.Fprintf(stderr, "testrusage: %v\n", waitErr)
	}
	return code
}

func forward(p *os.Process, sig <-chan os.Signal, done <-chan struct{}) {
	for {
		select {
		case s := <-sig:
			_ = p.Signal(s)
		case <-done:
			return
		}
	}
}

func exitCodeOf(ps *os.ProcessState) int {
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return signalExitOffset + int(ws.Signal())
	}
	return ps.ExitCode()
}

func buildRecord(ps *os.ProcessState, wall time.Duration, code int) Record {
	cwd, _ := os.Getwd()
	r := Record{Package: packageName(cwd), WallS: wall.Seconds(), ExitCode: code}
	u, ok := usageOf(ps, runtime.GOOS)
	if !ok {
		r.Note = usageNote
		return r
	}
	r.UserS, r.SystemS, r.PeakRSSMB = u.UserS, u.SystemS, u.PeakRSSMB
	r.BlockIn, r.BlockOut, r.CtxVoluntary, r.CtxInvoluntary = u.BlockIn, u.BlockOut, u.CtxVoluntary, u.CtxInvoluntary
	return r
}

func peakRSSMB(raw int64, goos string) float64 {
	if goos == "darwin" {
		return float64(raw) / bytesPerMB
	}
	return float64(raw) / kilobytesPerMB
}

func packageName(dir string) string {
	for cur := dir; ; cur = filepath.Dir(cur) {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			rel, _ := filepath.Rel(cur, dir)
			return filepath.ToSlash(rel)
		}
		if filepath.Dir(cur) == cur {
			return filepath.ToSlash(dir)
		}
	}
}

func appendRecord(path string, r Record) error {
	line, _ := json.Marshal(r)
	f, err := openAppend(path)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
