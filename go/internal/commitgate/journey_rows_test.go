package commitgate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/journeys"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	rowActive  = `{"id":"keep-me","journey":"A path works","test":"internal/x.TestA","issue":"#1"}`
	rowRetired = `{"id":"old-one","journey":"An old path","test":"internal/x.TestB","issue":"#2","retired":"the path was removed"}`
	rowRevived = `{"id":"old-one","journey":"An old path","test":"internal/x.TestB","issue":"#2"}`
	rowNew     = `{"id":"new-one","journey":"A new path","test":"internal/x.TestC","issue":"#3"}`
)

func registryFile(rows ...string) string { return "[" + strings.Join(rows, ",") + "]\n" }

func journeyRepo(t *testing.T, atHead string) (*gittest.Repo, Options) {
	t.Helper()
	repo := gittest.Fixture(t)
	if atHead != "" {
		mustWrite(t, filepath.Join(repo.Dir, journeys.RegistryPath), atHead)
	}
	mustWrite(t, filepath.Join(repo.Dir, "README.md"), "base\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	return repo, Options{RepoRoot: repo.Dir, ReviewDir: t.TempDir(), Runner: sysexec.DefaultRunner}
}

func TestRefuseRemovedJourneyRows_NamesEachRowThatLeftWithoutItsReason(t *testing.T) {
	head := registryFile(rowActive, rowRetired)
	tests := []struct {
		name   string
		after  string
		remove bool
		want   int
		named  string
	}{
		{"row deleted", registryFile(rowRetired), false, ExitFail, "keep-me"},
		{"retired row deleted", registryFile(rowActive), false, ExitFail, "old-one"},
		{"retired reason cleared", registryFile(rowActive, rowRevived), false, ExitFail, "old-one"},
		{"file deleted", "", true, ExitFail, "keep-me"},
		{"row added", registryFile(rowActive, rowRetired, rowNew), false, ExitPass, ""},
		{"row retired with a reason", registryFile(`{"id":"keep-me","journey":"A path works","test":"internal/x.TestA","issue":"#1","retired":"no longer a path"}`, rowRetired), false, ExitPass, ""},
		{"registry unparsable after", "[{", false, ExitFail, "journeys.json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, o := journeyRepo(t, head)
			if tc.remove {
				if err := os.Remove(filepath.Join(repo.Dir, journeys.RegistryPath)); err != nil {
					t.Fatal(err)
				}
			} else {
				mustWrite(t, filepath.Join(repo.Dir, journeys.RegistryPath), tc.after)
			}
			repo.Git("add", "-A")
			res := &Result{}

			code := o.refuseRemovedJourneyRows(context.Background(), nil, res)

			if logs := strings.Join(res.Logs, "\n"); code != tc.want || !strings.Contains(logs, tc.named) {
				t.Errorf("code = %d logs = %v, want %d naming %q", code, res.Logs, tc.want, tc.named)
			}
		})
	}
}

func TestRefuseRemovedJourneyRows_ReadsNothingWhenTheRegistryIsNotInTheChange(t *testing.T) {
	repo, o := journeyRepo(t, registryFile(rowActive))
	mustWrite(t, filepath.Join(repo.Dir, "README.md"), "changed\n")
	repo.Git("add", "-A")
	if code := o.refuseRemovedJourneyRows(context.Background(), nil, &Result{}); code != ExitPass {
		t.Errorf("code = %d, want pass: the registry is untouched", code)
	}
}

func TestRefuseRemovedJourneyRows_ANewRegistryHasNoRowToLose(t *testing.T) {
	repo, o := journeyRepo(t, "")
	mustWrite(t, filepath.Join(repo.Dir, journeys.RegistryPath), registryFile(rowActive))
	repo.Git("add", "-A")
	if code := o.refuseRemovedJourneyRows(context.Background(), nil, &Result{}); code != ExitPass {
		t.Errorf("code = %d, want pass for a registry that HEAD does not hold", code)
	}
}

func TestRefuseRemovedJourneyRows_AnUnreadableRegistryAtHeadIsARefusal(t *testing.T) {
	repo, o := journeyRepo(t, "[{")
	mustWrite(t, filepath.Join(repo.Dir, journeys.RegistryPath), registryFile(rowActive))
	repo.Git("add", "-A")
	res := &Result{}
	if code := o.refuseRemovedJourneyRows(context.Background(), nil, res); code != ExitFail || !strings.Contains(strings.Join(res.Logs, "\n"), "at HEAD") {
		t.Errorf("code = %d logs = %v, want a refusal naming HEAD", code, res.Logs)
	}
}

func TestRefuseRemovedJourneyRows_AnUnlistableChangeIsAFault(t *testing.T) {
	o := Options{RepoRoot: t.TempDir(), Runner: (&scriptRunner{rules: []scriptRule{{matchPrefix: "git diff --name-only --no-renames -z HEAD", exit: 128}}}).run()}
	res := &Result{}
	if code := o.refuseRemovedJourneyRows(context.Background(), nil, res); code != ExitGitFatal || !strings.Contains(strings.Join(res.Logs, "\n"), "journey registry check:") {
		t.Errorf("code = %d logs = %v, want ExitGitFatal naming the registry check", code, res.Logs)
	}
}

func TestRun_ADeletedJourneyRowIsRefusedBeforeAnyLaneRuns(t *testing.T) {
	repo, o := journeyRepo(t, registryFile(rowActive, rowRetired))
	mustWrite(t, filepath.Join(repo.Dir, journeys.RegistryPath), registryFile(rowRetired))
	repo.Git("add", "-A")

	res := o.Run(context.Background())

	if logs := strings.Join(res.Logs, "\n"); res.ExitCode != ExitFail || res.Attestation != nil || !strings.Contains(logs, "keep-me") {
		t.Errorf("exit = %d attestation = %v logs = %v, want a refusal naming keep-me and no attestation", res.ExitCode, res.Attestation, res.Logs)
	}
}

func TestRefuseRemovedJourneyRows_AnUnreadableHeadIsARefusalNamingHead(t *testing.T) {
	rules := []scriptRule{
		{matchPrefix: "git diff --name-only --no-renames -z HEAD", stdout: journeys.RegistryPath + "\x00"},
		{matchPrefix: "git cat-file", exit: 128},
	}
	o := Options{RepoRoot: t.TempDir(), Runner: (&scriptRunner{rules: rules}).run()}
	res := &Result{}
	if code := o.refuseRemovedJourneyRows(context.Background(), nil, res); code != ExitFail || !strings.Contains(strings.Join(res.Logs, "\n"), "at HEAD is unreadable") {
		t.Errorf("code = %d logs = %v, want a refusal naming HEAD", code, res.Logs)
	}
}
