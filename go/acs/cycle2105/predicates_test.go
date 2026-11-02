//go:build acs

package cycle2105

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/supersede"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	liveCycle    = 2105
	consoleCycle = 9001
	itemID       = "claimed-item"
)

func seedClaimedItem(t *testing.T, id string, cycle int) (root, inboxDir string) {
	t.Helper()
	root = t.TempDir()
	inboxDir = filepath.Join(root, ".evolve", "inbox")
	dir := inboxbatch.ProcessingCycleDir(inboxDir, cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(`{"id":"`+id+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, inboxDir
}

func mustCheck(t *testing.T, read supersede.Reader, id string, cycle int) supersede.Result {
	t.Helper()
	res, err := supersede.Check(read, id, cycle)
	if err != nil {
		t.Fatalf("Check(%q, cycle %d) error: %v", id, cycle, err)
	}
	return res
}

func TestC2105_001_ConsoleClaimBetweenTwoReadsReturnsSupersededWithTheClaim(t *testing.T) {
	_, inboxDir := seedClaimedItem(t, itemID, liveCycle)
	read := supersede.InboxReader(inboxDir)

	if first := mustCheck(t, read, itemID, liveCycle); first.Verdict != supersede.None {
		t.Fatalf("first read before any console action = %+v, want none", first)
	}
	from := filepath.Join(inboxbatch.ProcessingCycleDir(inboxDir, liveCycle), itemID+".json")
	toDir := inboxbatch.ProcessingCycleDir(inboxDir, consoleCycle)
	if err := os.MkdirAll(toDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(from, filepath.Join(toDir, itemID+".json")); err != nil {
		t.Fatal(err)
	}

	second := mustCheck(t, read, itemID, liveCycle)

	if second.Verdict != supersede.Superseded || second.Claim.Cycle != consoleCycle {
		t.Errorf("second read after the console claim = %+v, want superseded with claim cycle %d", second, consoleCycle)
	}
}

func TestC2105_002_ConsoleRouteBetweenTwoReadsReturnsSupersededWithTheRoute(t *testing.T) {
	root, inboxDir := seedClaimedItem(t, itemID, liveCycle)
	read := supersede.InboxReader(inboxDir)

	if first := mustCheck(t, read, itemID, liveCycle); first.Verdict != supersede.None {
		t.Fatalf("first read before the route = %+v, want none", first)
	}
	if _, err := inboxmover.RouteConsole(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, itemID, "console took the item", liveCycle); err != nil {
		t.Fatalf("RouteConsole: %v", err)
	}

	second := mustCheck(t, read, itemID, liveCycle)

	if second.Verdict != supersede.Superseded || second.Claim.Route != "console-manual" {
		t.Errorf("second read after the console route = %+v, want superseded with route console-manual", second)
	}
}

func TestC2105_003_AnUntouchedItemReturnsNoneOnEveryRead(t *testing.T) {
	_, inboxDir := seedClaimedItem(t, itemID, liveCycle)
	read := supersede.InboxReader(inboxDir)
	otherDir := inboxbatch.ProcessingCycleDir(inboxDir, consoleCycle)
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "other-item.json"), []byte(`{"id":"other-item"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		if res := mustCheck(t, read, itemID, liveCycle); res != (supersede.Result{Verdict: supersede.None}) {
			t.Errorf("read %d of an untouched item = %+v, want none with no claim", i+1, res)
		}
	}
}

func TestC2105_004_OnlyAClaimByAnotherHolderOrAConsoleRouteSupersedes(t *testing.T) {
	cases := []struct {
		name string
		got  supersede.Reading
		want supersede.Verdict
	}{
		{"own claim", supersede.Reading{Cycle: liveCycle}, supersede.None},
		{"own claim with a lane route", supersede.Reading{Cycle: liveCycle, Route: "lane"}, supersede.None},
		{"another holder", supersede.Reading{Cycle: consoleCycle}, supersede.Superseded},
		{"own claim routed to the console", supersede.Reading{Cycle: liveCycle, Route: "console-manual"}, supersede.Superseded},
		{"released to the root and routed to the console", supersede.Reading{Route: "console-manual"}, supersede.Superseded},
		{"released to the root and not routed", supersede.Reading{}, supersede.None},
	}
	for _, tc := range cases {
		read := func(string) (supersede.Reading, error) { return tc.got, nil }
		if res := mustCheck(t, read, itemID, liveCycle); res.Verdict != tc.want {
			t.Errorf("%s: verdict = %q, want %q", tc.name, res.Verdict, tc.want)
		}
	}
}

func TestC2105_005_TheCheckReadsOncePerAskAndNeverBetween(t *testing.T) {
	reads := 0
	read := func(id string) (supersede.Reading, error) {
		reads++
		if id != itemID {
			t.Errorf("read for %q, want %q", id, itemID)
		}
		return supersede.Reading{Cycle: liveCycle}, nil
	}

	if reads != 0 {
		t.Fatalf("reads before any ask = %d, want 0", reads)
	}
	for ask := 1; ask <= 3; ask++ {
		mustCheck(t, read, itemID, liveCycle)
		if reads != ask {
			t.Fatalf("reads after ask %d = %d, want exactly %d", ask, reads, ask)
		}
	}
}

func TestC2105_006_AFailedReadIsAnErrorAndNeverASupersede(t *testing.T) {
	boom := errors.New("inbox unreadable")
	read := func(string) (supersede.Reading, error) { return supersede.Reading{Cycle: consoleCycle}, boom }

	res, err := supersede.Check(read, itemID, liveCycle)

	if !errors.Is(err, boom) {
		t.Errorf("error = %v, want it to wrap the read failure", err)
	}
	if res.Verdict == supersede.Superseded {
		t.Errorf("a failed read stopped the cycle: %+v", res)
	}
}

func TestC2105_007_TheSupersedeStageDefaultsToShadow(t *testing.T) {
	for _, body := range []string{`{}`, `{"workflow": {}}`, `{"workflow": {"supersede": {}}}`} {
		p, err := policy.Parse([]byte(body))
		if err != nil || p.WorkflowConfig().Supersede.Stage != config.StageShadow {
			t.Errorf("%s: stage = %v, err = %v, want shadow", body, p.WorkflowConfig().Supersede.Stage, err)
		}
	}
}

func TestC2105_008_TheSupersedeStageReadsOffShadowAndEnforce(t *testing.T) {
	for word, want := range map[string]config.Stage{"0": config.StageOff, "off": config.StageOff, "shadow": config.StageShadow, "enforce": config.StageEnforce} {
		p, err := policy.Parse([]byte(`{"workflow": {"supersede": {"stage": "` + word + `"}}}`))
		if got := p.WorkflowConfig().Supersede.Stage; err != nil || got != want {
			t.Errorf("stage %q = %v, err = %v, want %v", word, got, err, want)
		}
	}
}

func TestC2105_009_TheSupersedeStageIsStrict(t *testing.T) {
	for _, body := range []string{
		`{"workflow": {"supersede": {"stage": "advisory"}}}`,
		`{"workflow": {"supersede": {"stage": "enforced"}}}`,
		`{"workflow": {"supersede": {"stge": "enforce"}}}`,
		`{"workflow": {"supersede": {"stage": "enforce", "poll_seconds": 5}}}`,
	} {
		if _, err := policy.Parse([]byte(body)); err == nil || !strings.Contains(err.Error(), "supersede") {
			t.Errorf("%s: err = %v, want a refusal that names supersede", body, err)
		}
	}
}

func TestC2105_010_TheNewPackageIsEnrolledInTheAPICoverGateAndNamesItsAPI(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enrolled, err := os.ReadFile(filepath.Join(root, "go", ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(string(enrolled), "\n") {
		if strings.TrimSpace(line) == "./internal/supersede" {
			found = true
		}
	}
	if !found {
		t.Errorf("go/.apicover-enforce has no ./internal/supersede line")
	}
	if !acsassert.FileExists(t, filepath.Join(root, "go", "internal", "supersede", "apicover_named_test.go")) {
		t.Errorf("go/internal/supersede/apicover_named_test.go is missing")
	}
	_, stderr, code, runErr := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./internal/supersede")
	if code != 0 {
		t.Errorf("go test ./internal/supersede exit %d: %v\n%s", code, runErr, acsassert.OutputTail(stderr, 30))
	}
}
