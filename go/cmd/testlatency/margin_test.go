package main

import (
	"strings"
	"testing"
)

const marginStream = `
{"Action":"run","Package":"x/slow","Test":"TestA"}
{"Action":"pass","Package":"x/slow","Elapsed":1500}
{"Action":"run","Package":"x/hung","Test":"TestB"}
`

func TestRun_TimeoutMarginFlagsLowAndNamesTheIncompletePackage(t *testing.T) {
	code, out, _ := runLatency(t, marginStream, "-timeout-margin", "30m")
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	for _, want := range []string{"## Margin against the go test timeout", "| x/slow | 1500.00 | 1800.00 | 300.00 | 16.7 | LOW |", "| x/hung |", "no terminal summary"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
}

func TestRun_TimeoutMarginOfAnEmptyStreamShowsNone(t *testing.T) {
	_, out, _ := runLatency(t, "", "-timeout-margin", "30m")
	if !strings.Contains(out, "| _(none)_ | | | | | |") {
		t.Errorf("empty stream has no placeholder row:\n%s", out)
	}
}

func TestRun_TimeoutMarginRejectsANonPositiveOrUnparsableValue(t *testing.T) {
	for _, bad := range []string{"soon", "0s", "-5m"} {
		code, _, errOut := runLatency(t, marginStream, "-timeout-margin", bad)
		if code != 2 || !strings.Contains(errOut, "-timeout-margin") {
			t.Errorf("%q: exit %d, stderr %q, want 2 naming the flag", bad, code, errOut)
		}
	}
}

func TestRun_NoTimeoutMarginLeavesTheReportWithoutTheSection(t *testing.T) {
	_, out, _ := runLatency(t, marginStream)
	if strings.Contains(out, "Margin against the go test timeout") {
		t.Errorf("section present without the flag:\n%s", out)
	}
}
