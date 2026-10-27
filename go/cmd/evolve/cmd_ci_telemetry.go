package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/citelemetry"
)

const ciTelemetryPrefix = "evolve ci telemetry: "

var errTelemetryUsage = errors.New("want --jobs <file> and no operand")

func runCITelemetry(args []string, stdout, stderr io.Writer) int {
	var jobsFile string
	var bindings []string
	var asJSON bool
	operands, err := cliFlags{
		bools:  map[string]*bool{"--json": &asJSON},
		values: map[string]*string{"--jobs": &jobsFile},
		lists:  map[string]*[]string{"--job-artifacts": &bindings},
	}.parse(args)
	if err == nil && (jobsFile == "" || len(operands) != 0) {
		err = errTelemetryUsage
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", ciTelemetryPrefix, err, ciUsage)
		return exitUsage
	}
	rep, err := citelemetry.Run(jobsFile, bindings)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciTelemetryPrefix, err)
		return exitIO
	}
	if err := writeTelemetryReport(stdout, rep, asJSON); err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciTelemetryPrefix, err)
		return exitIO
	}
	return 0
}

func writeTelemetryReport(w io.Writer, rep citelemetry.Report, asJSON bool) error {
	if !asJSON {
		_, err := io.WriteString(w, rep.Markdown())
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}
