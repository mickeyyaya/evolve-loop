package explanationdocs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func (f fixture) sealCitingTestFile(t *testing.T) {
	t.Helper()
	f.prepareRequired(t)
	f.write(t, "config/app_test.go", "package config_test\n")
	doc := strings.Replace(f.validDocument(), "while preserving its schema.\n",
		"while preserving its schema.\n- `config/app_test.go` — pins the enabled setting through the public field.\n", 1)
	f.write(t, cycleDocumentPath(f.cycle, f.runID), doc)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("CheckBuild before the post-Build writer: %v", failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}
}

func TestRefreshResult_StaleCitationAfterPostBuildWriterIsAContentFailure(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.sealCitingTestFile(t)
	if err := os.Remove(filepath.Join(f.worktree, "config", "app_test.go")); err != nil {
		t.Fatal(err)
	}

	requiresBuild, err := RefreshResult(context.Background(), f.binding())

	if requiresBuild || err == nil {
		t.Fatalf("RefreshResult = (%v, %v), want the stale citation reported as an error", requiresBuild, err)
	}
	if !errors.Is(err, ErrContent) {
		t.Fatalf("a stale Changed Areas citation is the explanation's own content, want errors.Is(err, ErrContent): %v", err)
	}
	if want := "cited path config/app_test.go is not in the Build diff"; !strings.Contains(err.Error(), want) {
		t.Errorf("the content failure keeps the validator's verdict text %q: %v", want, err)
	}
}

func TestRefreshResult_UnreadableOrUnboundFailureIsNotAContentFailure(t *testing.T) {
	for name, breakIt := range map[string]func(t *testing.T, f fixture) CycleBinding{
		"a corrupt host snapshot": func(t *testing.T, f fixture) CycleBinding {
			if err := os.WriteFile(resultSnapshotPath(f.root, f.cycle), []byte("{not json"), 0o644); err != nil {
				t.Fatal(err)
			}
			return f.binding()
		},
		"an explanation document that is no longer a regular file": func(t *testing.T, f fixture) CycleBinding {
			doc := filepath.Join(f.worktree, filepath.FromSlash(cycleDocumentPath(f.cycle, f.runID)))
			body, err := os.ReadFile(doc)
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(t.TempDir(), "elsewhere.md")
			if err := os.WriteFile(target, body, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(doc); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, doc); err != nil {
				t.Fatal(err)
			}
			return f.binding()
		},
		"a missing Build report": func(t *testing.T, f fixture) CycleBinding {
			if err := os.Remove(filepath.Join(f.workspace, "build-report.md")); err != nil {
				t.Fatal(err)
			}
			return f.binding()
		},
		"a binding that names another worktree": func(t *testing.T, f fixture) CycleBinding {
			binding := f.binding()
			binding.Worktree = t.TempDir()
			return binding
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.activate(t)
			f.sealCitingTestFile(t)
			binding := breakIt(t, f)

			requiresBuild, err := RefreshResult(context.Background(), binding)

			if requiresBuild || err == nil {
				t.Fatalf("RefreshResult = (%v, %v), want an abort-class error", requiresBuild, err)
			}
			if errors.Is(err, ErrContent) {
				t.Fatalf("a binding or unreadable failure must not be classed as explanation content: %v", err)
			}
		})
	}
}

func TestRefreshResult_AContentFailureMatchesOnlyErrContent(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.sealCitingTestFile(t)
	if err := os.Remove(filepath.Join(f.worktree, "config", "app_test.go")); err != nil {
		t.Fatal(err)
	}

	_, err := RefreshResult(context.Background(), f.binding())

	if !errors.Is(err, ErrContent) || errors.Is(err, context.Canceled) || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a content failure matches ErrContent and no other sentinel: %v", err)
	}
}

func TestRefreshResult_AFaultKeepsTheValidatorsFaultText(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.sealCitingTestFile(t)
	if err := os.Remove(filepath.Join(f.workspace, "build-report.md")); err != nil {
		t.Fatal(err)
	}

	_, err := RefreshResult(context.Background(), f.binding())

	if want := "revalidate builder explanation handoff before sealing: Explanation Documentation: build-report.md is missing"; err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
}

func TestCheckBuild_AMaterialDiffDeclaredNotApplicableIsRejected(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	f.writeWorkspace(t, "build-report.md", "# Build Report\n\n## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Document: "+cycleDocumentPath(f.cycle, f.runID)+"\n")

	failures := f.check(t)

	want := "Explanation Documentation: Status must be REQUIRED for a material Build diff (material: config/app.yaml)"
	if !slices.Contains(failures, want) {
		t.Fatalf("a material diff declared NOT_APPLICABLE must be rejected with %q, got %q", want, failures)
	}
}

func TestCheckBuild_AFaultBesideContentVerdictsKeepsEveryLine(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	document := cycleDocumentPath(f.cycle, f.runID)
	symlinkDocument(t, f, document)
	f.writeWorkspace(t, "build-report.md", "# Build Report\n\n## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Document: "+document+"\n")

	failures := f.check(t)

	want := []string{
		"Explanation Documentation: Status must be REQUIRED for a material Build diff (material: config/app.yaml)",
		"Explanation Documentation: " + document + " must be a regular non-symlink file",
	}
	if !slices.Equal(failures, want) {
		t.Fatalf("CheckBuild reports the declaration verdict and the fault, in order:\n got %q\nwant %q", failures, want)
	}
}

func symlinkDocument(t *testing.T, f fixture, document string) {
	t.Helper()
	path := filepath.Join(f.worktree, filepath.FromSlash(document))
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "elsewhere.md")
	if err := os.WriteFile(target, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}

func TestSealResult_AManifestWriteFaultIsNotExplanationContent(t *testing.T) {
	for name, prepare := range map[string]func(t *testing.T, f fixture){
		"a required explanation": func(t *testing.T, f fixture) { f.prepareRequired(t) },
		"a not-applicable declaration": func(t *testing.T, f fixture) {
			f.writeWorkspace(t, "build-report.md", "# Build Report\n\n"+RenderNotApplicableDeclaration("the walk produces no Build diff"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.activate(t)
			prepare(t, f)
			if err := os.MkdirAll(filepath.Join(f.workspace, manifestFilename), 0o755); err != nil {
				t.Fatal(err)
			}

			err := SealResult(context.Background(), f.binding())

			if err == nil || errors.Is(err, ErrContent) {
				t.Fatalf("an unwritable workspace manifest is a fault, never explanation content: %v", err)
			}
		})
	}
}

func TestRefreshResult_AnOversizedDocumentIsExplanationContent(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.sealCitingTestFile(t)
	document := cycleDocumentPath(f.cycle, f.runID)
	f.write(t, document, strings.Repeat("x", maxArtifactBytes+1))

	_, err := RefreshResult(context.Background(), f.binding())

	if !errors.Is(err, ErrContent) || !strings.Contains(err.Error(), document+" exceeds") {
		t.Fatalf("an oversized explanation document is the Builder's content to re-author: %v", err)
	}
}

func TestRefreshResult_AMissingDocumentIsExplanationContent(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.sealCitingTestFile(t)
	document := cycleDocumentPath(f.cycle, f.runID)
	if err := os.Remove(filepath.Join(f.worktree, filepath.FromSlash(document))); err != nil {
		t.Fatal(err)
	}

	_, err := RefreshResult(context.Background(), f.binding())

	if !errors.Is(err, ErrContent) || !strings.Contains(err.Error(), document) {
		t.Fatalf("a missing explanation document is the Builder's content to author: %v", err)
	}
}

func TestRefreshResult_AFaultBesideAContentVerdictAbortsAndKeepsEveryLine(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.sealCitingTestFile(t)
	document := cycleDocumentPath(f.cycle, f.runID)
	symlinkDocument(t, f, document)
	f.writeWorkspace(t, "build-report.md", "# Build Report\n\n## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Document: "+document+"\n")

	_, err := RefreshResult(context.Background(), f.binding())

	if err == nil || errors.Is(err, ErrContent) {
		t.Fatalf("a fault beside a content verdict is a fault, not content: %v", err)
	}
	want := "revalidate builder explanation handoff before sealing: " +
		"Explanation Documentation: Status must be REQUIRED for a material Build diff (material: config/app.yaml); " +
		"Explanation Documentation: " + document + " must be a regular non-symlink file"
	if err.Error() != want {
		t.Fatalf("the abort keeps the content verdict and the fault, in order:\n got %q\nwant %q", err.Error(), want)
	}
}
