package main

import (
	"errors"
	"strings"
	"testing"
)

func TestCodeRefreshAmp_Gotypes_TwoRefreshesBuildTwiceWithTheSameArguments(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	host, rec := gotypesHost(&refreshRunner{}, nil)

	for i := 0; i < 2; i++ {
		if rc, _, stderr := refreshWith(t, host, "--project-root", root); rc != 0 {
			t.Fatalf("refresh %d: rc=%d stderr=%q", i, rc, stderr)
		}
	}

	if len(rec.builds) != 2 || rec.builds[0].moduleDir != rec.builds[1].moduleDir || rec.builds[0].out != rec.builds[1].out {
		t.Errorf("builds = %v, want two builds with identical arguments", rec.builds)
	}
}

func TestCodeRefreshAmp_Gotypes_AFailedBuildNeverReachesStdout(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	host, _ := gotypesHost(&refreshRunner{}, errors.New("gotypes: boom"))

	rc, stdout, _ := refreshWith(t, host, "--project-root", root)

	if rc != 0 || strings.Contains(stdout, "WARN") || strings.Contains(stdout, "boom") {
		t.Errorf("rc=%d stdout=%q, want exit 0 and the warning on stderr only", rc, stdout)
	}
}

func TestCodeRefreshAmp_Gotypes_AFailedTrailStepUnderJSONStillReportsTheTrailError(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{failOn: "/abs/trail", failRC: 1}
	host, rec := gotypesHost(rr, errors.New("gotypes: boom"))

	rc, stdout, _ := refreshWith(t, host, "--json", "--project-root", root)

	e := decodeCodeEnvelope(t, stdout)
	if rc != codeExitFailed || e["ok"] == true || e["error"] == nil || len(rec.builds) != 1 {
		t.Errorf("rc=%d envelope=%v builds=%d, want exit 4, a not-ok envelope with the trail error, one build", rc, e, len(rec.builds))
	}
}
