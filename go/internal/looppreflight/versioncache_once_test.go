package looppreflight

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestRun_VersionInventoryProbedOncePerRun(t *testing.T) {
	opts := goodPipelineOptions(t)
	calls := 0
	opts.VersionInventory = func() map[string]string {
		calls++
		return map[string]string{"claude": fmt.Sprintf("2.1.%d", calls)}
	}

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calls != 1 {
		t.Errorf("version inventory captured %d times in one Run, want exactly 1", calls)
	}
	if r.CLIVersions["claude"] != "2.1.1" {
		t.Errorf("Result.CLIVersions = %v, want the same snapshot the drift check saw (claude=2.1.1)", r.CLIVersions)
	}
}

func TestRun_ReportedVersionsMatchSavedCache(t *testing.T) {
	opts := goodPipelineOptions(t)
	calls := 0
	opts.VersionInventory = func() map[string]string {
		calls++
		return map[string]string{"claude": fmt.Sprintf("3.0.%d", calls)}
	}

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	saved, err := loadVersionCache(filepath.Join(opts.EvolveDir, cliVersionsFile))
	if err != nil {
		t.Fatalf("loadVersionCache: %v", err)
	}
	if saved["claude"] != r.CLIVersions["claude"] {
		t.Errorf("cache saved %v but Result reported %v: drift comparison and report diverged", saved, r.CLIVersions)
	}
}

func TestRun_DefaultInventoryProbesEachBinaryOnce(t *testing.T) {
	orig := execVersion
	t.Cleanup(func() { execVersion = orig })
	var mu sync.Mutex
	perBin := map[string]int{}
	execVersion = func(bin string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		perBin[bin]++
		return bin + " 1.2.3", nil
	}
	opts := goodPipelineOptions(t)

	if _, err := Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(perBin) == 0 {
		t.Fatalf("no --version probe ran: the default inventory path was not exercised")
	}
	for bin, n := range perBin {
		if n != 1 {
			t.Errorf("%s --version executed %d times in one Run, want 1", bin, n)
		}
	}
}

func TestSaveVersionCache_SurvivesStaticTempNameCollision(t *testing.T) {
	path := filepath.Join(t.TempDir(), cliVersionsFile)
	if err := os.Mkdir(path+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}

	if err := saveVersionCache(path, map[string]string{"claude": "2.1.1"}); err != nil {
		t.Fatalf("saveVersionCache failed when %s.tmp is occupied: a fixed temp name collides across concurrent boots: %v", cliVersionsFile, err)
	}
	got, err := loadVersionCache(path)
	if err != nil || got["claude"] != "2.1.1" {
		t.Errorf("loadVersionCache = %v, %v; want claude=2.1.1", got, err)
	}
}

func TestSaveVersionCache_ConcurrentWritersLeaveValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), cliVersionsFile)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- saveVersionCache(path, map[string]string{"claude": fmt.Sprintf("2.1.%d", i)})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent saveVersionCache: %v", err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil || len(m) != 1 {
		t.Errorf("cache after concurrent writes is not one valid map: %q, %v", data, err)
	}
}

func TestRun_CacheWrittenDespiteStaticTempOccupied(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.9"} }
	cache := filepath.Join(opts.EvolveDir, cliVersionsFile)
	if err := os.Mkdir(cache+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := loadVersionCache(cache)
	if err != nil || got["claude"] != "2.1.9" {
		t.Errorf("Run did not persist the version cache through the atomic writer: %v, %v", got, err)
	}
}

func TestGoodPipelineOptions_StubsEveryProcessSeam(t *testing.T) {
	opts := goodPipelineOptions(t)
	seams := map[string]bool{
		"VersionInventory":     opts.VersionInventory != nil,
		"CLIHealthActive":      opts.CLIHealthActive != nil,
		"PhaseRoutingWarnings": opts.PhaseRoutingWarnings != nil,
		"OrphanKill":           opts.OrphanKill != nil,
	}
	for name, stubbed := range seams {
		if !stubbed {
			t.Errorf("goodPipelineOptions leaves %s unstubbed: unit tests would shell out to real subprocesses", name)
		}
	}
}

func TestRun_GoodPipelineOptionsExecNoVersionProbe(t *testing.T) {
	orig := execVersion
	t.Cleanup(func() { execVersion = orig })
	probes := 0
	execVersion = func(string) (string, error) {
		probes++
		return "", fmt.Errorf("real exec attempted")
	}

	if _, err := Run(goodPipelineOptions(t)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if probes != 0 {
		t.Errorf("Run(goodPipelineOptions) executed %d real --version probes, want 0", probes)
	}
}
