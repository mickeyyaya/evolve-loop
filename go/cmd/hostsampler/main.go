// Command hostsampler samples the host once each interval and appends one NDJSON record per tick until it gets SIGTERM.
// See docs/architecture/packages/cmd-hostsampler.md.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	exitFailure = 1
	exitUsage   = 2

	modeAppendCreate  = os.O_WRONLY | os.O_APPEND | os.O_CREATE
	recordPermissions = 0o644
)

type reader func() (Counters, error)

var (
	exitProcess = os.Exit
	openOut     = func(path string) (io.WriteCloser, error) {
		return os.OpenFile(path, modeAppendCreate, recordPermissions)
	}
)

func main() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	exitProcess(start(os.Args[1:], os.Stderr, stop, newSource))
}

var errUnsupportedPlatform = errors.New("unsupported platform: no host source")

func unsupportedSource() (reader, error) {
	return nil, errUnsupportedPlatform
}

func start(args []string, stderr io.Writer, stop <-chan os.Signal, source func() (reader, error)) int {
	read, err := source()
	if err != nil {
		fmt.Fprintf(stderr, "hostsampler: %v\n", err)
		return 0
	}
	return run(args, stderr, read, stop)
}

func run(args []string, stderr io.Writer, read reader, stop <-chan os.Signal) int {
	fs := flag.NewFlagSet("hostsampler", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "NDJSON file to append samples to (required)")
	interval := fs.Duration("interval", time.Second, "time between samples")
	duration := fs.Duration("duration", 0, "stop after this long (0 runs until SIGTERM)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if *out == "" || *interval <= 0 {
		fmt.Fprintln(stderr, "usage: hostsampler -out <file> [-interval 1s] [-duration 0]")
		return exitUsage
	}
	sink, err := openOut(*out)
	if err != nil {
		fmt.Fprintf(stderr, "hostsampler: %v\n", err)
		return exitFailure
	}
	code := sampleLoop(sink, stderr, read, stop, *interval, *duration)
	if err := sink.Close(); err != nil {
		fmt.Fprintf(stderr, "hostsampler: %v\n", err)
		return exitFailure
	}
	return code
}

func sampleLoop(sink io.Writer, stderr io.Writer, read reader, stop <-chan os.Signal, interval, duration time.Duration) int {
	prev, err := read()
	if err != nil {
		fmt.Fprintf(stderr, "hostsampler: %v\n", err)
		return exitFailure
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var deadline <-chan time.Time
	if duration > 0 {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		deadline = timer.C
	}
	for {
		select {
		case <-stop:
			return 0
		case <-deadline:
			return 0
		case <-ticker.C:
			cur, err := read()
			if err != nil {
				fmt.Fprintf(stderr, "hostsampler: %v\n", err)
				continue
			}
			line, _ := json.Marshal(compute(prev, cur))
			if _, err := sink.Write(append(line, '\n')); err != nil {
				fmt.Fprintf(stderr, "hostsampler: %v\n", err)
				return exitFailure
			}
			prev = cur
		}
	}
}
