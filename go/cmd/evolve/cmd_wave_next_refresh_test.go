package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const waveFuturePolicy = `{"cli_routing": {"clis": ["claude"], "default": ["claude"], "share_pct": 40}}`

func TestWaveNext_APolicyKeyTheBinaryDoesNotKnowRebuildsAndReExecsBeforeTheParse(t *testing.T) {
	p := newWavePlane(t)
	p.seedWave81()
	p.write(".evolve/policy.json", waveFuturePolicy)
	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	swapExecTargetOK(t)
	swapFleetLaneCheck(t, false, nil)
	h := newWaveHarness()
	h.refresh = bootBinaryRefresh

	h.run("next", "--json", "--project-root", p.root)

	if strings.Join(spy.orderLog, ",") != "rebuild,exec" {
		t.Fatalf("wave next did %v before the policy parse, want rebuild then exec; stderr:\n%s", spy.orderLog, h.stderr.String())
	}
	if len(h.refreshed) != 1 || h.refreshed[0].ProjectRoot != p.root || h.refreshed[0].EvolveDir != p.evolveDir() {
		t.Fatalf("refresh calls = %+v, want one for the plane %s", h.refreshed, p.root)
	}
	if h.verbs.calls != nil {
		t.Fatalf("a boundary step ran before the binary was fresh: %+v", h.verbs.calls)
	}
}

func TestWaveNext_AFreshBinaryLaunchesWithNoRebuild(t *testing.T) {
	p := newWavePlane(t)
	p.seedWave81()
	swapBinaryCommit(t, "eeee23def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	h := newWaveHarness()
	h.refresh = bootBinaryRefresh

	if rc := h.run("next", "--json", "--project-root", p.root); rc != 0 || spy.rebuildCalls != 0 || spy.execCalls != 0 {
		t.Fatalf("rc = %d, spy %+v; want a launch with no rebuild; stderr:\n%s", rc, spy, h.stderr.String())
	}
}

func TestWaveNext_AnUnknownPolicyKeyNamesTheRebuildRemedy(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave81()
	p.write(".evolve/policy.json", waveFuturePolicy)
	h := newWaveHarness()

	rc := h.run("next", "--json", "--project-root", p.root)

	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	remedy := "make -C " + filepath.Join(p.root, "go") + " build"
	if rc != exitIO || !strings.Contains(e.Error, "share_pct") || !strings.Contains(e.Error, remedy) || !strings.Contains(e.Error, "wave next") {
		t.Fatalf("rc = %d, error %q; want exit %d with the unknown key and the remedy %q", rc, e.Error, exitIO, remedy)
	}
}

func TestWaveNext_ADryRunNeverRebuildsTheBinary(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()

	if rc := h.run("next", "--dry-run", "--project-root", p.root); rc != 0 || len(h.refreshed) != 0 {
		t.Fatalf("rc = %d, refresh calls %d; want a dry run with no rebuild", rc, len(h.refreshed))
	}
}

func TestWaveNext_AnotherPolicyErrorKeepsItsOwnText(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave81()
	p.write(".evolve/policy.json", `{"cli_routing": `)
	h := newWaveHarness()

	rc := h.run("next", "--json", "--project-root", p.root)

	if e := decodeWaveEnvelope(t, h.stdout.Bytes()); rc != exitIO || strings.Contains(e.Error, "make -C") {
		t.Fatalf("rc = %d, error %q; want the parse error with no rebuild remedy", rc, e.Error)
	}
}

func TestDefaultWaveEnv_RefreshesThroughTheBootRefreshOfTheLoop(t *testing.T) {
	t.Parallel()
	env := defaultWaveEnv()

	if env.refreshBinary == nil || reflect.ValueOf(env.refreshBinary).Pointer() != reflect.ValueOf(bootBinaryRefresh).Pointer() {
		t.Fatal("the production wave env must heal a stale binary with bootBinaryRefresh, the one self-heal of the loop")
	}
}

func TestWaveNext_TheRefreshOptOutHoldsWhenThePolicyHasAnUnknownKey(t *testing.T) {
	p := newWavePlane(t)
	p.seedWave81()
	p.write(".evolve/policy.json", `{"boot": {"binary_refresh": "off"}, "cli_routing": {"clis": ["claude"], "default": ["claude"], "share_pct": 40}}`)
	swapBinaryCommit(t, "abc123def456")
	spy := swapRefreshSeams(t, "eeee23def456fffffffffffffffffffffffffff0", nil, true, nil, nil, nil)
	swapExecTargetOK(t)
	h := newWaveHarness()
	h.refresh = bootBinaryRefresh

	h.run("next", "--json", "--project-root", p.root)

	if spy.rebuildCalls != 0 || !strings.Contains(h.stderr.String(), "boot.binary_refresh=off") {
		t.Fatalf("rebuilds = %d; want none, because boot.binary_refresh=off; stderr:\n%s", spy.rebuildCalls, h.stderr.String())
	}
}

func TestBootBinaryRefresh_AnUnreadablePolicyKeepsTheDefaultAuto(t *testing.T) {
	t.Parallel()
	for name, setup := range map[string]func(t *testing.T, evolveDir string){
		"invalid json": func(t *testing.T, evolveDir string) {
			writeFileOrFail(t, filepath.Join(evolveDir, "policy.json"), `{"boot": `)
		},
		"a directory": func(t *testing.T, evolveDir string) {
			if err := os.MkdirAll(filepath.Join(evolveDir, "policy.json"), 0o755); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			evolveDir := t.TempDir()
			setup(t, evolveDir)

			if bootRefreshOff(evolveDir) {
				t.Fatalf("bootRefreshOff = true for an unreadable policy (%s), want the default auto", name)
			}
		})
	}
}

func writeFileOrFail(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWaveNext_TheRefreshRunsBeforeThePolicyParseSoTheRebuiltBinaryLaunches(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave81()
	p.write(".evolve/policy.json", waveFuturePolicy)
	h := newWaveHarness()
	h.refresh = func(loopConfig, io.Writer) bootBinaryRefreshResult {
		p.write(".evolve/policy.json", `{"cli_routing": {"clis": ["claude"], "default": ["claude"]}}`)
		return bootBinaryRefreshResult{Stale: true, Rebuilt: true}
	}

	rc := h.run("next", "--json", "--project-root", p.root)

	if e := decodeWaveEnvelope(t, h.stdout.Bytes()); rc != 0 || !e.Launched || !e.Recorded {
		t.Fatalf("rc = %d, envelope %+v; want a launch: the refresh, which stands in for the rebuilt binary that knows the key, must run before the parse; stderr:\n%s", rc, e, h.stderr.String())
	}
}

func TestWaveNext_TheRemedyRepeatsTheOriginalArguments(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave81()
	p.write(".evolve/policy.json", waveFuturePolicy)
	h := newWaveHarness()

	h.run("next", "--merge", "1101,1102", "--note", "two words", "--json", "--project-root", p.root)

	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	want := filepath.Join(p.root, "go", "bin", "evolve") + ` wave next again with the arguments ["--merge" "1101,1102" "--note" "two words" "--json" "--project-root" "` + p.root + `"]`
	if !strings.Contains(e.Error, want) {
		t.Fatalf("error %q; want the rerun command %q", e.Error, want)
	}
}
