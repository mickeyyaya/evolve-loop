package phasecmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestObserverPolicy_MalformedPolicyEmitsRegisteredObserverWarn(t *testing.T) {
	if m, ok := signalcenter.IsRegistered(CodePolicyLoadFailed); !ok || m != signalcenter.ModuleObserver {
		t.Fatalf("IsRegistered(%s) = %q, %v; want observer, true", CodePolicyLoadFailed, m, ok)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", writeObserverPolicy(t, `{`))
	c := signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	pol := observerPolicy(observerArgs{pos: []string{"ws", "1", "7", "build", "builder"}, cycle: 7}, c)
	c.Flush()
	if *pol.ObserverConfig().StallS != 600 {
		t.Errorf("StallS = %d, want the compiled default 600", *pol.ObserverConfig().StallS)
	}
	if len(got) != 1 || got[0].Code != CodePolicyLoadFailed || got[0].Severity != signalcenter.SeverityWarn ||
		got[0].Module != signalcenter.ModuleObserver || got[0].Cycle != 7 || got[0].Phase != "build" ||
		!strings.HasSuffix(got[0].Fields["path"], "policy.json") {
		t.Fatalf("events = %+v, want one observer WARN %s naming policy.json", got, CodePolicyLoadFailed)
	}
}

func TestObserverPolicy_AbsentPolicyIsSilent(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	c := signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	observerPolicy(observerArgs{pos: []string{"ws", "1", "7", "build", "builder"}, cycle: 7}, c)
	c.Flush()
	if len(got) != 0 {
		t.Fatalf("events = %+v, want none for an absent policy.json", got)
	}
}

func TestObserverEnvConfig_Defaults(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	c := observerEnvConfig(mustLoadPolicy(t))
	if c.PollS != 5 || c.StallS != 600 || c.EOFGraceS != 0 {
		t.Errorf("PollS/StallS/EOFGraceS = %d/%d/%d, want 5/600/0", c.PollS, c.StallS, c.EOFGraceS)
	}
	if c.NudgeS != 300 {
		t.Errorf("NudgeS default = %d, want 300 (a flip to 0 disables the nudge)", c.NudgeS)
	}
	if c.NudgeBody != "" {
		t.Errorf("NudgeBody default = %q, want empty", c.NudgeBody)
	}
}

func TestObserverEnvConfig_Parsing(t *testing.T) {
	root := writeObserverPolicy(t, `{"observer":{"poll_s":7,"stall_s":20,"nudge_s":0,"nudge_body":"wake up","eof_grace_s":3}}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	c := observerEnvConfig(mustLoadPolicy(t))
	if c.PollS != 7 || c.StallS != 20 || c.NudgeS != 0 || c.EOFGraceS != 3 || c.NudgeBody != "wake up" {
		t.Errorf("got PollS=%d StallS=%d NudgeS=%d EOFGraceS=%d NudgeBody=%q", c.PollS, c.StallS, c.NudgeS, c.EOFGraceS, c.NudgeBody)
	}
}

func mustLoadPolicy(t *testing.T) policy.Policy {
	t.Helper()
	pol, _, err := loadPolicy()
	if err != nil {
		t.Fatal(err)
	}
	return pol
}

func writeObserverPolicy(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
