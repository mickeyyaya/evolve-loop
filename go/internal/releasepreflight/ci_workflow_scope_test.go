package releasepreflight

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestDefaultCIConclusion_ReadsTheRequiredWorkflowNotTheNewestRun(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Git("commit", "--allow-empty", "-qm", "release")
	for _, tc := range []struct {
		name, requiredRuns string
		want               CIRunStatus
	}{
		{"required_red", `[{"status":"completed","conclusion":"failure","url":"https://ci/required"}]`, CIRunStatus{Conclusion: "failure", RunURL: "https://ci/required"}},
		{"required_running", `[{"status":"in_progress","conclusion":"","url":"https://ci/required"}]`, CIRunStatus{Conclusion: "pending", RunURL: "https://ci/required"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installFakeGH(t, tc.requiredRuns)
			got, err := defaultCIConclusion(repo.Dir)
			if err != nil || got != tc.want {
				t.Fatalf("defaultCIConclusion = %+v, %v; want %+v (a newer green run of another workflow must not mask it)", got, err, tc.want)
			}
		})
	}
}

func installFakeGH(t *testing.T, requiredRuns string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\ncase \" $* \" in\n" +
		"*\" --workflow " + ciparity.RequiredWorkflow + " \"*) echo '" + requiredRuns + "' ;;\n" +
		"*) echo '[{\"status\":\"completed\",\"conclusion\":\"success\",\"url\":\"https://ci/landing-pages\"}]' ;;\n" +
		"esac\n"
	fakeclitest.Install(t, filepath.Join(dir, "gh"), script)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
