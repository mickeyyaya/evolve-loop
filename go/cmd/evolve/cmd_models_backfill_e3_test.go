package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/setup"
)

func TestRunModelsRefresh_RefusesBadArgsBeforeDetecting(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"unknown flag", []string{"--nope"}, "flag provided but not defined"},
		{"unknown source", []string{"--source", "bogus"}, `unknown --source "bogus" (live|detect)`},
		{"unicode source", []string{"--source", "ライブ"}, `unknown --source "ライブ"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), ".evolve")
			code, out, errb := runModelsCLI(append([]string{"refresh", "--evolve-dir", dir}, tc.args...)...)
			if code != 10 {
				t.Fatalf("code = %d, want 10 (bad args)", code)
			}
			if !strings.Contains(errb, tc.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", errb, tc.wantStderr)
			}
			if out != "" {
				t.Errorf("stdout = %q, want nothing on refused args", out)
			}
			if _, err := os.Stat(filepath.Join(dir, "model-catalog.json")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("a refused refresh wrote the catalog: stat err = %v", err)
			}
		})
	}
}

func TestCommitAndReport_AnUnwritableEvolveDirFailsWithoutAReport(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := refreshResult(map[string]modelcatalog.CLIEntry{
		"codex": {TierModels: map[string]string{"deep": "gpt-5.6"}, Source: modelcatalog.SourceLive},
	})
	for _, asJSON := range []bool{false, true} {
		var out, errb bytes.Buffer

		code := commitAndReport(modelsOpts{EvolveDir: blocker, AsJSON: asJSON}, cat, "live", &out, &errb)

		if code != 1 {
			t.Fatalf("json=%v: code = %d, want 1 when the catalog cannot be written", asJSON, code)
		}
		if !strings.Contains(errb.String(), "\nevolve models refresh: modelcatalog: mkdir ") {
			t.Errorf("json=%v: stderr = %q, want the refresh error", asJSON, errb.String())
		}
		if out.Len() != 0 {
			t.Errorf("json=%v: stdout = %q, want no report for a catalog that did not land", asJSON, out.String())
		}
	}
}

func TestCommitAndReport_HumanReportNamesTheSourceAndCount(t *testing.T) {
	dir := t.TempDir()
	cat := refreshResult(map[string]modelcatalog.CLIEntry{
		"codex":  {TierModels: map[string]string{"deep": "gpt-5.6"}, Source: modelcatalog.SourceLive},
		"claude": {TierModels: map[string]string{"fast": "haiku"}, Source: modelcatalog.SourceLive},
	})
	var out, errb bytes.Buffer

	code := commitAndReport(modelsOpts{EvolveDir: dir}, cat, "setup detect", &out, &errb)

	if code != 0 {
		t.Fatalf("code = %d, stderr %q", code, errb.String())
	}
	if !strings.HasPrefix(out.String(), "Refreshed model catalog (source: setup detect) → 2 CLI(s):\n") {
		t.Errorf("report header = %q", out.String())
	}
	onDisk, err := modelcatalog.Read(dir)
	if err != nil || onDisk.CLIs["codex"].TierModels["deep"] != "gpt-5.6" {
		t.Errorf("committed catalog = %+v (err %v), want codex deep gpt-5.6", onDisk, err)
	}
}

func TestPrintCatalogHuman_PrintsEffortsOnlyForACLIThatOffersThem(t *testing.T) {
	cat := refreshResult(map[string]modelcatalog.CLIEntry{
		"codex":  {TierModels: map[string]string{"deep": "gpt-5.6"}, Source: modelcatalog.SourceLive, Efforts: []string{"low", "xhigh"}},
		"claude": {TierModels: map[string]string{"fast": "haiku"}, Source: modelcatalog.SourceLive},
		"agy":    {TierModels: map[string]string{"fast": "flash"}, Source: modelcatalog.SourceLive, Efforts: []string{"ünïcode"}},
	})
	var out bytes.Buffer

	printCatalogHuman(&out, cat)

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	want := []string{
		"  agy      fast=flash            balanced=-                deep=-",
		"           source: live",
		"           efforts: ünïcode",
		"  claude   fast=haiku            balanced=-                deep=-",
		"           source: live",
		"  codex    fast=-                balanced=-                deep=gpt-5.6",
		"           source: live",
		"           efforts: low, xhigh",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("printCatalogHuman lines =\n%q\nwant\n%q", lines, want)
	}
}

func TestPrintCatalogHuman_EmptyCatalogPrintsNothing(t *testing.T) {
	var out bytes.Buffer

	printCatalogHuman(&out, modelcatalog.Catalog{})

	if out.Len() != 0 {
		t.Errorf("an empty catalog printed %q", out.String())
	}
}

func TestBridgePromptDispatcher_ALaunchFailureNamesTheDriver(t *testing.T) {
	boom := errors.New("pane died")
	d := bridgePromptDispatcher{workspace: t.TempDir(), projectRoot: t.TempDir(),
		launch: func(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
			return core.BridgeResponse{Stdout: "partial"}, boom
		},
	}

	reply, err := d.DispatchPrompt(context.Background(), "claude", "PROMPT")

	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "launch claude-p") {
		t.Fatalf("err = %v, want the wrapped launch error naming driver claude-p", err)
	}
	if reply != "" {
		t.Errorf("reply = %q, want empty on a failed launch", reply)
	}
}

func TestBridgePromptDispatcher_ReturnsTheLaunchStdout(t *testing.T) {
	var got core.BridgeRequest
	d := bridgePromptDispatcher{workspace: t.TempDir(), projectRoot: t.TempDir(),
		launch: func(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
			got = req
			return core.BridgeResponse{Stdout: `{"fast":"m"}`}, nil
		},
	}

	reply, err := d.DispatchPrompt(context.Background(), "codex", "PROMPT")

	if err != nil || reply != `{"fast":"m"}` {
		t.Fatalf("reply %q err %v", reply, err)
	}
	if got.CLI != "codex" || got.Agent != "model-classifier" {
		t.Errorf("request = %+v, want codex driver and model-classifier agent", got)
	}
}

func TestBridgePromptDispatcher_LauncherDefaultsToTheBridgeEngine(t *testing.T) {
	injected := func(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
		return core.BridgeResponse{Stdout: "injected"}, nil
	}
	withSeam := bridgePromptDispatcher{launch: injected}
	if resp, _ := withSeam.launcher()(context.Background(), core.BridgeRequest{}); resp.Stdout != "injected" {
		t.Errorf("an injected launch was not used: %q", resp.Stdout)
	}
	var zero bridgePromptDispatcher
	if zero.launcher() == nil {
		t.Fatal("no launch seam must fall back to the bridge engine, got nil")
	}
}

func TestLiveRefreshRoots(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, project, evolve, wantProject, wantEvolve string
	}{
		{"both empty", "", "", cwd, filepath.Join(cwd, ".evolve")},
		{"project only", "/p", "", "/p", "/p/.evolve"},
		{"both set", "/p", "/elsewhere/.e", "/p", "/elsewhere/.e"},
		{"evolve only", "", "/e", cwd, "/e"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, e, err := liveRefreshRoots(tc.project, tc.evolve)
			if err != nil || p != tc.wantProject || e != tc.wantEvolve {
				t.Errorf("liveRefreshRoots(%q,%q) = %q,%q,%v want %q,%q", tc.project, tc.evolve, p, e, err, tc.wantProject, tc.wantEvolve)
			}
		})
	}
}

func TestReadyCLIsWithDetectMaps(t *testing.T) {
	codexMap := map[string]string{"deep": "gpt-5.6"}
	cases := []struct {
		name      string
		clis      []setup.CLIStatus
		wantReady []string
		wantMaps  map[string]map[string]string
	}{
		{"no CLIs", nil, nil, map[string]map[string]string{}},
		{"only blocked", []setup.CLIStatus{{CLI: "codex", Verdict: "blocked", TierModels: codexMap}}, nil, map[string]map[string]string{}},
		{"warning is not ready", []setup.CLIStatus{{CLI: "claude", Verdict: "warning"}}, nil, map[string]map[string]string{}},
		{"ready keeps order and drops empty maps", []setup.CLIStatus{
			{CLI: "codex", Verdict: "ready", TierModels: codexMap},
			{CLI: "agy", Verdict: "blocked", TierModels: map[string]string{"fast": "f"}},
			{CLI: "ollama", Verdict: "ready"},
		}, []string{"codex", "ollama"}, map[string]map[string]string{"codex": codexMap}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ready, maps := readyCLIsWithDetectMaps(setup.DetectReport{CLIs: tc.clis})
			if !reflect.DeepEqual(ready, tc.wantReady) || !reflect.DeepEqual(maps, tc.wantMaps) {
				t.Errorf("got %v %v, want %v %v", ready, maps, tc.wantReady, tc.wantMaps)
			}
		})
	}
}

func TestPolicyAllowedFamilies(t *testing.T) {
	writePolicy := func(t *testing.T, body string) string {
		t.Helper()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	t.Run("malformed policy yields no filter", func(t *testing.T) {
		if got := policyAllowedFamilies(writePolicy(t, "{not json")); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("declared families are returned", func(t *testing.T) {
		root := writePolicy(t, `{"catalog":{"allowed_families":{"agy":["Gemini"]}}}`)
		got := policyAllowedFamilies(root)
		if !reflect.DeepEqual(got, map[string][]string{"agy": {"Gemini"}}) {
			t.Errorf("got %v, want agy -> [Gemini]", got)
		}
	})
}

func TestClassifierPreference_NoTableUsesTheLegacyOrder(t *testing.T) {
	got, err := classifierPreference(t.TempDir())
	if err != nil {
		t.Fatalf("classifierPreference: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"codex", "claude", "agy"}) {
		t.Errorf("got %v, want the legacy codex > claude > agy", got)
	}
}

func TestClassifierFamilies_ATableWithNoDefaultRefusesTheClassifier(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Work: map[string][]string{"build": {"agy", "claude"}}}

	families, err := classifierFamilies(classifierRouter(t, policy.Policy{CLIRouting: &block}))

	if err == nil || !strings.Contains(err.Error(), "no route for the model classifier") {
		t.Fatalf("err = %v, want the no-route refusal", err)
	}
	if families != nil {
		t.Errorf("families = %v, want nil on a refusal", families)
	}
}

func TestClassifierFamilies_DedupesDriversOfOneFamily(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"claude", "agy"}, Default: []string{"claude", "agy", "claude"}}
	families, err := classifierFamilies(classifierRouter(t, policy.Policy{CLIRouting: &block}))
	if err != nil {
		t.Fatalf("classifierFamilies: %v", err)
	}
	if !reflect.DeepEqual(families, []string{"claude", "agy"}) {
		t.Errorf("families = %v, want claude, agy once each", families)
	}
}

func e3ReadyReport(clis ...string) setup.DetectReport {
	var rep setup.DetectReport
	for _, c := range clis {
		rep.CLIs = append(rep.CLIs, setup.CLIStatus{CLI: c, Verdict: "ready", TierModels: map[string]string{"balanced": c + "-model"}})
	}
	return rep
}

func TestLiveRefresh_NoReadyCLIIsAnError(t *testing.T) {
	cat, err := liveRefresh(context.Background(), setup.DetectReport{CLIs: []setup.CLIStatus{{CLI: "codex", Verdict: "blocked"}}}, t.TempDir(), "", modelcatalog.Catalog{}, io.Discard)

	if err == nil || err.Error() != "no ready CLIs to query (run: evolve setup detect)" {
		t.Fatalf("err = %v", err)
	}
	if !cat.Empty() {
		t.Errorf("catalog = %+v, want empty", cat)
	}
}

func TestLiveRefresh_AnUncreatableScratchDirIsAnError(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing", "tmp"))

	_, err := liveRefresh(context.Background(), e3ReadyReport("ollama"), t.TempDir(), "", modelcatalog.Catalog{}, io.Discard)

	if err == nil || !strings.HasPrefix(err.Error(), "liveRefresh: scratch workspace: ") {
		t.Fatalf("err = %v, want the scratch workspace error", err)
	}
}

func TestLiveRefresh_ARefusedRoutingTableFailsBeforeProbing(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"cli_routing":{"clis":["agy"],"default":["agy"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := liveRefresh(context.Background(), e3ReadyReport("zzcli"), root, "", modelcatalog.Catalog{}, io.Discard)

	if err == nil || !strings.Contains(err.Error(), "refuses to route the model classifier") {
		t.Fatalf("err = %v, want the routing refusal", err)
	}
	if _, serr := os.Stat(filepath.Join(evolveDir, "model-catalog.json")); !errors.Is(serr, os.ErrNotExist) {
		t.Errorf("a refused refresh wrote a catalog: %v", serr)
	}
}

func TestLiveRefresh_AnUnlistableCLIFallsBackToItsDetectMap(t *testing.T) {
	root := t.TempDir()
	var log bytes.Buffer

	cat, err := liveRefresh(context.Background(), e3ReadyReport("zzcli"), root, "", modelcatalog.Catalog{}, &log)

	if err != nil {
		t.Fatalf("liveRefresh: %v", err)
	}
	entry := cat.CLIs["zzcli"]
	if entry.Source != modelcatalog.SourceDetect || entry.TierModels["balanced"] != "zzcli-model" || entry.FallbackReason == "" {
		t.Errorf("entry = %+v, want the detect fallback with a reason", entry)
	}
	if !strings.Contains(log.String(), "zzcli") {
		t.Errorf("the fallback was not logged: %q", log.String())
	}
}

func TestLiveRefreshRoots_AnUnresolvableWorkingDirectoryIsAnError(t *testing.T) {
	prev := processGetwd
	t.Cleanup(func() { processGetwd = prev })
	processGetwd = func() (string, error) { return "", errors.New("getwd: no such file or directory") }

	p, e, err := liveRefreshRoots("", "/explicit/.evolve")
	_, lerr := liveRefresh(context.Background(), e3ReadyReport("ollama"), "", "", modelcatalog.Catalog{}, io.Discard)
	pp, ee, perr := liveRefreshRoots("/p", "")

	if err == nil || err.Error() != "liveRefresh: resolve workspace: getwd: no such file or directory" || p != "" || e != "" {
		t.Fatalf("liveRefreshRoots = %q, %q, %v; want the resolve-workspace error", p, e, err)
	}
	if lerr == nil || !strings.HasPrefix(lerr.Error(), "liveRefresh: resolve workspace: ") {
		t.Fatalf("liveRefresh err = %v, want the resolve-workspace error", lerr)
	}
	if perr != nil || pp != "/p" || ee != "/p/.evolve" {
		t.Errorf("an explicit root must not need the working directory: %q %q %v", pp, ee, perr)
	}
}
