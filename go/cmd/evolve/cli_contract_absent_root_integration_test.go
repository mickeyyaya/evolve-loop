//go:build integration

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

const (
	absentRootItem = "absent-root-reads-as-success"
	absentRootWait = 5 * time.Second
)

var contractAbsentRootOpen = map[string]string{
	"wave supply":         "open: inbox item " + absentRootItem,
	"inbox rank":          "open: inbox item " + absentRootItem,
	"inbox lint":          "open: inbox item " + absentRootItem,
	"inbox batches":       "open: inbox item " + absentRootItem,
	"signals tail":        "open: inbox item " + absentRootItem,
	"events channels":     "open: inbox item " + absentRootItem,
	"dossier verify":      "open: inbox item " + absentRootItem,
	"cli-routing check":   "open: inbox item " + absentRootItem,
	"cli-routing explain": "open: inbox item " + absentRootItem,
	"routing explain":     "open: inbox item " + absentRootItem,
	"clihealth list":      "open: inbox item " + absentRootItem,
	"cloud list":          "open: inbox item " + absentRootItem,
	"events kinds":        "the vocabulary is compiled into the binary; no root is read",
	"agent capabilities":  "the verb prints a table compiled into the binary; no root is read",
	"quality show":        "the show exits 0 on each reading and signals QUALITY_SOURCE_ABSENT, as contractNoNegative states",
}

func contractCleanPool(t *testing.T) map[string]string {
	t.Helper()
	vars := contractPlane(t)
	rec := integration.WaveRecord{Pool: 5, Epoch: 2, ShadowBase: statusSha, Ended: true, LastCycle: 8,
		Members:   map[int]integration.MemberState{6: {Status: integration.MemberComposed}},
		Decisions: []integration.FlushRecord{{Trigger: integration.TriggerWaveEnd, At: time.Unix(0, 0).UTC()}}}
	if err := (integration.FileWaveStore{Root: vars["root"]}).Write(rec); err != nil {
		t.Fatal(err)
	}
	return vars
}

func runOnAbsentRoot(t *testing.T, c contractCall) (int, string, bool) {
	t.Helper()
	contractScratch(t, "HOME")
	contractScratch(t, "TMPDIR")
	contractWorkdir(t)
	absent := filepath.Join(t.TempDir(), "nowhere")
	args := contractArgs(c.args, map[string]string{"root": absent})
	gate := contractGate(t, c)
	type result struct {
		rc  int
		out string
	}
	done := make(chan result, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		rc := dispatchWith(gate, args, strings.NewReader(c.stdin), &stdout, &stderr)
		done <- result{rc, stdout.String() + stderr.String()}
	}()
	select {
	case r := <-done:
		return r.rc, r.out, false
	case <-time.After(absentRootWait):
		return 0, "", true
	}
}

var contractAbsentRootWaits = map[string]string{
	"events watch": "open: inbox item " + absentRootItem + "; the verb waits and cannot be stopped, so a run of it outlives the test and breaks the temporary directory cleanup",
}

func TestCLIContract_AnAbsentRootIsNeverASilentSuccess(t *testing.T) {
	contractEnvironment(t)
	for _, vc := range contractCases {
		v, ok := lookupCoreVerb(vc.verb)
		_, waits := contractAbsentRootWaits[vc.verb]
		if !ok || v.class != availAlways || waits {
			continue
		}
		for _, c := range vc.calls {
			if c.kind != contractSuccess {
				continue
			}
			t.Run(vc.verb, func(t *testing.T) {
				rc, out, hung := runOnAbsentRoot(t, c)
				reason, listed := contractAbsentRootOpen[vc.verb]
				silent := hung || rc == cliExitOK || rc == cliExitUsage
				switch {
				case silent && !listed:
					t.Errorf("%s exits %d (hung %t) on an absent root, so it reads as success, gives a usage error or never answers; fix the verb or list it in contractAbsentRootOpen with a reason\n%s", vc.verb, rc, hung, out)
				case !silent && listed:
					t.Errorf("%s exits %d on an absent root now; remove it from contractAbsentRootOpen (%s)", vc.verb, rc, reason)
				}
			})
		}
	}
	for verb, reason := range contractAbsentRootWaits {
		if v, ok := lookupCoreVerb(verb); !ok || !v.stream || reason == "" {
			t.Errorf("%s: a row of contractAbsentRootWaits needs a reason and a stream verb", verb)
		}
	}
	for verb, reason := range contractAbsentRootOpen {
		if reason == "" {
			t.Errorf("%s: a row of contractAbsentRootOpen needs a reason", verb)
		}
		if v, ok := lookupCoreVerb(verb); !ok || v.class != availAlways {
			t.Errorf("%s: not an always verb, so the test never runs this row", verb)
		}
	}
}
