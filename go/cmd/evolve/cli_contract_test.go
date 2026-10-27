package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

type decodedEnvelope struct {
	Schema string          `json:"schema"`
	OK     bool            `json:"ok"`
	Data   json.RawMessage `json:"data"`
	Error  *struct {
		Code    int    `json:"code"`
		Kind    string `json:"kind"`
		Message string `json:"message"`
	} `json:"error"`
	Warnings []string `json:"warnings"`
}

func decodeEnvelopes(t *testing.T, out string) []decodedEnvelope {
	t.Helper()
	var envs []decodedEnvelope
	dec := json.NewDecoder(strings.NewReader(out))
	for {
		var raw map[string]json.RawMessage
		if err := dec.Decode(&raw); errors.Is(err, io.EOF) {
			return envs
		} else if err != nil {
			t.Fatalf("stdout is not a sequence of JSON objects: %v\n%s", err, out)
		}
		for _, key := range []string{"schema", "ok", "data", "error", "warnings"} {
			if _, ok := raw[key]; !ok {
				t.Fatalf("envelope has no %q field: %s", key, out)
			}
		}
		whole, _ := json.Marshal(raw)
		var e decodedEnvelope
		if err := json.Unmarshal(whole, &e); err != nil {
			t.Fatal(err)
		}
		if e.Warnings == nil {
			t.Fatalf("warnings is null, want a list: %s", out)
		}
		envs = append(envs, e)
	}
}

func oneEnvelope(t *testing.T, out string) decodedEnvelope {
	t.Helper()
	envs := decodeEnvelopes(t, out)
	if len(envs) != 1 {
		t.Fatalf("want exactly one envelope, got %d:\n%s", len(envs), out)
	}
	return envs[0]
}

func runFake(rc int, out, errText string) func(stdout, stderr io.Writer) int {
	return func(stdout, stderr io.Writer) int {
		fmt.Fprint(stdout, out)
		fmt.Fprint(stderr, errText)
		return rc
	}
}

func TestRunEnveloped_WrapsTheResultOfAVerbInOneEnvelope(t *testing.T) {
	t.Parallel()
	inboxList, _ := lookupCoreVerb("inbox list")
	agentRun, _ := lookupCoreVerb("agent run")
	cases := []struct {
		name                 string
		verb                 coreVerb
		rc                   int
		out, errText         string
		wantData, wantKind   string
		wantMessage, wantErr string
		wantWarnings         int
	}{
		{"success", inboxList, 0, "[{\"id\":\"a\"}]\n", "", `[{"id":"a"}]`, "", "", "", 0},
		{"usage", inboxList, 10, "", "\ninbox list: unknown arg \"x\"\n  -json\nusage: evolve inbox list\n", "null", "usage", `inbox list: unknown arg "x"`, "usage: evolve inbox list", 0},
		{"the error line after progress and warnings", inboxList, 2, "", "step 1/3\nevolve inbox list: WARN slow\nevolve inbox list: read failed\nthe tail of the step:\n  | tail\nUsage of x:", "null", "io", "evolve inbox list: read failed", "step 1/3", 0},
		{"refusal", inboxList, 3, "", "refused: a lane is live\n", "null", "refused", "refused: a lane is live", "", 0},
		{"two JSON values become a list", inboxList, 0, "{\"a\":1}\n{\"b\":2}\n", "", `[{"a":1},{"b":2}]`, "", "", "", 1},
		{"text on stdout moves to stderr", inboxList, 1, "3 item(s)\n", "", "null", "failed", "evolve inbox list exited 1 (failed)", "3 item(s)", 1},
		{"a verb code", agentRun, 21, "", "dispatch failed\n", "null", "dispatch", "dispatch failed", "", 0},
		{"a code outside the table", inboxList, 7, "", "odd\n", "null", "unlisted", "odd", "", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			rc := runEnveloped(c.verb, runFake(c.rc, c.out, c.errText), &stdout, &stderr)
			e := oneEnvelope(t, stdout.String())
			if rc != c.rc || e.Schema != c.verb.schema() || e.OK != (c.rc == 0) || string(e.Data) != c.wantData || len(e.Warnings) != c.wantWarnings {
				t.Fatalf("rc %d envelope %+v data %s; want rc %d data %s", rc, e, e.Data, c.rc, c.wantData)
			}
			if c.rc == 0 {
				if e.Error != nil {
					t.Fatalf("error = %+v on success", e.Error)
				}
				return
			}
			if e.Error == nil || e.Error.Code != c.rc || e.Error.Kind != c.wantKind || e.Error.Message != c.wantMessage {
				t.Fatalf("error = %+v; want code %d kind %q message %q", e.Error, c.rc, c.wantKind, c.wantMessage)
			}
			if !strings.Contains(stderr.String(), c.wantErr) {
				t.Fatalf("stderr %q lacks %q", stderr.String(), c.wantErr)
			}
		})
	}
}

func TestRunEnveloped_AStreamPrintsOneEnvelopeForEachLine(t *testing.T) {
	t.Parallel()
	watch, _ := lookupCoreVerb("wave watch")
	cases := []struct {
		name      string
		rc        int
		out       string
		wantData  []string
		wantError bool
	}{
		{"lines", 0, "{\"n\":1}\nnot json\n\n{\"n\":2}", []string{`{"n":1}`, `{"n":2}`}, false},
		{"no line", 0, "", []string{"null"}, false},
		{"lines then a failure", 2, "{\"n\":1}\n", []string{`{"n":1}`, "null"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			rc := runEnveloped(watch, func(o, e io.Writer) int {
				for _, part := range strings.SplitAfter(c.out, "\n") {
					fmt.Fprint(o, part)
				}
				fmt.Fprint(e, "watch ended\n")
				return c.rc
			}, &stdout, &stderr)
			envs := decodeEnvelopes(t, stdout.String())
			if rc != c.rc || len(envs) != len(c.wantData) {
				t.Fatalf("rc %d, %d envelopes; want rc %d, %d envelopes:\n%s", rc, len(envs), c.rc, len(c.wantData), stdout.String())
			}
			for i, e := range envs {
				if string(e.Data) != c.wantData[i] || e.Schema != "evolve.wave.watch/1" {
					t.Errorf("envelope %d = %+v data %s; want data %s", i, e, e.Data, c.wantData[i])
				}
			}
			last := envs[len(envs)-1]
			if (last.Error != nil) != c.wantError {
				t.Errorf("last error = %+v, want error %t", last.Error, c.wantError)
			}
			if strings.Contains(c.out, "not json") && !strings.Contains(stderr.String(), "not json") {
				t.Errorf("stderr %q lacks the text line", stderr.String())
			}
		})
	}
}

func TestRunEnveloped_AnUnwritableStdoutIsAnIOFailure(t *testing.T) {
	t.Parallel()
	gc, _ := lookupCoreVerb("gc")
	for _, rc := range []int{0, 10} {
		var stderr bytes.Buffer
		got := runEnveloped(gc, runFake(rc, "{}", ""), &failingWriter{}, &stderr)
		if got != max(rc, cliExitIO) || !strings.Contains(stderr.String(), "evolve gc: write the JSON envelope: broken pipe") {
			t.Errorf("rc %d: got %d stderr %q", rc, got, stderr.String())
		}
	}
}

func TestJSONRequested_ReadsTheFlagBeforeTheTerminator(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"list --json":           true,
		"list -json":            true,
		"list --json=true":      true,
		"list -json=true":       true,
		"list --json=false":     false,
		"list":                  false,
		"run -- --json":         false,
		"show id --json --root": true,
	}
	for args, want := range cases {
		if got := jsonRequested(strings.Fields(args)); got != want {
			t.Errorf("jsonRequested(%q) = %t, want %t", args, got, want)
		}
	}
}

func TestCoreVerbOf_FindsTheVerbPathOfACall(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, rest, want string
	}{
		{"loop", "status --json", "loop status"},
		{"loop", "--project-root /p status", "loop status"},
		{"loop", "--max-cycles 2 fix the parser", "loop"},
		{"inbox", "list --json", "inbox list"},
		{"inbox", "claims --json", ""},
		{"gc", "--dry-run", "gc"},
		{"cli", "update --dry-run", "cli update"},
		{"cli", "", ""},
		{"wave", "note add x", "wave note"},
	}
	for _, c := range cases {
		v, ok := coreVerbOf(c.name, strings.Fields(c.rest))
		if v.path != c.want || ok != (c.want != "") {
			t.Errorf("coreVerbOf(%s %s) = %q %t, want %q", c.name, c.rest, v.path, ok, c.want)
		}
	}
}

func TestCoreVerb_SchemaAndExitKinds(t *testing.T) {
	t.Parallel()
	next, _ := lookupCoreVerb("wave next")
	if got := next.schema(); got != "evolve.wave.next/2" {
		t.Errorf("wave next schema = %q", got)
	}
	run, _ := lookupCoreVerb("agent run")
	for rc, want := range map[int]string{3: "refused", 4: "queued", 21: "dispatch"} {
		if kind, ok := run.exitKind(rc); !ok || kind != want {
			t.Errorf("exitKind(%d) = %q %t, want %q", rc, kind, ok, want)
		}
	}
	for _, rc := range []int{5, 22, 100} {
		if _, ok := run.exitKind(rc); ok {
			t.Errorf("exitKind(%d) is known, want unknown", rc)
		}
	}
	if _, ok := lookupCoreVerb("inbox claims"); ok {
		t.Error("inbox claims is not a core verb")
	}
}

func TestRunEnveloped_AStreamStopsAtAnUnwritableStdout(t *testing.T) {
	t.Parallel()
	watch, _ := lookupCoreVerb("wave watch")
	var stderr bytes.Buffer
	var writeErr error
	rc := runEnveloped(watch, func(o, _ io.Writer) int {
		_, writeErr = fmt.Fprint(o, "{\"n\":1}\n{\"n\":2}\n")
		fmt.Fprint(o, "{\"n\":3}")
		return cliExitOK
	}, &failingWriter{}, &stderr)
	if rc != cliExitIO || !errors.Is(writeErr, errEnvelopeStdout) || strings.Count(stderr.String(), "write the JSON envelope") != 1 {
		t.Fatalf("rc %d write error %v stderr %q, want exit 2, one write error and a stopped stream", rc, writeErr, stderr.String())
	}
}

func TestRunEnveloped_TheMessageIsTheFailureOfABoundaryStep(t *testing.T) {
	t.Parallel()
	run, _ := lookupCoreVerb("boundary run")
	verbs := &fakeBoundaryVerbs{failAt: 3, code: cliExitRefused}
	var stdout, stderr bytes.Buffer
	rc := runEnveloped(run, func(o, e io.Writer) int {
		return runBoundaryWith(verbs.dispatch, []string{"run", "--json", "--goal-text-file", fakeBoundaryGoalFile(t), "--project-root", t.TempDir()}, o, e)
	}, &stdout, &stderr)
	e := oneEnvelope(t, stdout.String())
	if rc != cliExitRefused || e.Error == nil || !strings.HasPrefix(e.Error.Message, "evolve boundary run: step 3/10 ") || !strings.Contains(e.Error.Message, "failed rc=3") {
		t.Fatalf("rc %d error %+v, want the failed step line as the message", rc, e.Error)
	}
}
