package commitgate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/journeys"
)

const twoRows = `{"rows":[{"id":"kept","journey":"j","test":"p.TestA","issue":"i"},{"id":"dropped","journey":"j","test":"p.TestB","issue":"i"}]}`

func journeyRowsRun(t *testing.T, head string, rules []scriptRule, files ...string) (int, *Result) {
	t.Helper()
	root := t.TempDir()
	if head != "" {
		root = registryRoot(t, head)
	}
	o := baseOpts(root)
	o.Runner = (&scriptRunner{rules: rules}).run()
	res := &Result{}
	return o.refuseDroppedJourneys(context.Background(), files, res), res
}

func baseShows(body string) []scriptRule {
	return []scriptRule{{matchPrefix: "git show HEAD:" + journeys.RegistryPath, stdout: body}}
}

func TestRefuseDroppedJourneys_ARowGoesAwayOnlyWithRetiredAndAReason(t *testing.T) {
	t.Parallel()
	base := baseShows(twoRows)
	dropped := `{"rows":[{"id":"kept","journey":"j","test":"p.TestA","issue":"i"}]}`
	retired := `{"rows":[{"id":"kept","journey":"j","test":"p.TestA","issue":"i"},{"id":"dropped","journey":"j","test":"p.TestB","issue":"i","retired":{"reason":"the path is gone"}}]}`
	noReason := `{"rows":[{"id":"kept","journey":"j","test":"p.TestA","issue":"i"},{"id":"dropped","journey":"j","test":"p.TestB","issue":"i","retired":{"reason":""}}]}`

	if code, res := journeyRowsRun(t, dropped, base, journeys.RegistryPath); code != ExitFail || !strings.Contains(strings.Join(res.Logs, "\n"), "dropped") {
		t.Errorf("a dropped row: code %d logs %v, want ExitFail that names the row", code, res.Logs)
	}
	if code, res := journeyRowsRun(t, noReason, base, journeys.RegistryPath); code != ExitFail {
		t.Errorf("a retired row with no reason: code %d logs %v, want ExitFail", code, res.Logs)
	}
	if code, res := journeyRowsRun(t, retired, base, journeys.RegistryPath); code != ExitPass {
		t.Errorf("a retired row with a reason: code %d logs %v, want ExitPass", code, res.Logs)
	}
	if code, res := journeyRowsRun(t, "", base, journeys.RegistryPath); code != ExitFail {
		t.Errorf("a deleted registry: code %d logs %v, want ExitFail", code, res.Logs)
	}
}

func TestRefuseDroppedJourneys_PassesWhenTheRegistryIsNotChangedOrHasNoBase(t *testing.T) {
	t.Parallel()
	absent := []scriptRule{{matchPrefix: "git cat-file -e HEAD:", exit: 128}}
	if code, _ := journeyRowsRun(t, "", baseShows(twoRows), "README.md"); code != ExitPass {
		t.Errorf("an unchanged registry: code %d, want ExitPass", code)
	}
	if code, res := journeyRowsRun(t, twoRows, absent, journeys.RegistryPath); code != ExitPass || len(res.Logs) != 0 {
		t.Errorf("a registry new at this commit: code %d logs %v, want ExitPass and no log", code, res.Logs)
	}
	if code, res := journeyRowsRun(t, twoRows, baseShows("{"), journeys.RegistryPath); code != ExitPass || len(res.Logs) != 1 {
		t.Errorf("a malformed base: code %d logs %v, want ExitPass and one log line", code, res.Logs)
	}
}

func TestRefuseDroppedJourneys_AnUnreadableBaseIsAFault(t *testing.T) {
	t.Parallel()
	unreadable := []scriptRule{{matchPrefix: "git cat-file -e HEAD", exit: 128}}
	if code, _ := journeyRowsRun(t, twoRows, unreadable, journeys.RegistryPath); code != ExitGitFatal {
		t.Errorf("an unreadable HEAD: code %d, want ExitGitFatal", code)
	}
	broken := []scriptRule{{matchPrefix: "git show", err: errors.New("no git")}}
	if code, _ := journeyRowsRun(t, twoRows, broken, journeys.RegistryPath); code != ExitGitFatal {
		t.Errorf("git cannot run: code %d, want ExitGitFatal", code)
	}
}
