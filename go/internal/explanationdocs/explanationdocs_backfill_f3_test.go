package explanationdocs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

func newFixtureF3(t *testing.T) fixture {
	t.Helper()
	f := fixture{
		root:     t.TempDir(),
		worktree: gittest.Fixture(t).Dir,
		cycle:    42,
		runID:    "run-42",
	}
	f.workspace = filepath.Join(f.root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(f.workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	f.write(t, "config/app.yaml", "enabled: false\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "base")
	f.base = f.git(t, "rev-parse", "HEAD")
	return f
}

func sealedRequiredF3(t *testing.T) fixture {
	t.Helper()
	f := newFixtureF3(t)
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}
	return f
}

func sealedNotApplicableF3(t *testing.T) fixture {
	t.Helper()
	f := newFixtureF3(t)
	f.activate(t)
	f.write(t, "config/app_test.go", "package config\n")
	f.writeWorkspace(t, "build-report.md", RenderNotApplicableDeclaration("the Build changed tests only"))
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}
	return f
}

func rewriteViewF3(t *testing.T, f fixture, manifest, snapshot bool, mutate func(*phaseio.ExplanationView)) {
	t.Helper()
	if manifest {
		path := filepath.Join(f.workspace, manifestFilename)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var view phaseio.ExplanationView
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatal(err)
		}
		mutate(&view)
		if err := atomicwrite.JSON(path, view); err != nil {
			t.Fatal(err)
		}
	}
	if snapshot {
		path := resultSnapshotPath(f.root, f.cycle)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var sealed resultSnapshot
		if err := json.Unmarshal(raw, &sealed); err != nil {
			t.Fatal(err)
		}
		mutate(&sealed.View)
		if err := atomicwrite.JSON(path, sealed); err != nil {
			t.Fatal(err)
		}
	}
}

func writeSparseF3(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(size); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestVerify_EachBrokenLinkOfASealedHandoffIsRefusedWithItsOwnReason(t *testing.T) {
	cases := []struct {
		name    string
		sealed  func(t *testing.T) fixture
		tamper  func(t *testing.T, f fixture)
		wantErr string
	}{
		{name: "workspace manifest removed", sealed: sealedRequiredF3, tamper: func(t *testing.T, f fixture) {
			if err := os.Remove(filepath.Join(f.workspace, manifestFilename)); err != nil {
				t.Fatal(err)
			}
		}, wantErr: "read build-explanation.json"},
		{name: "host snapshot removed", sealed: sealedRequiredF3, tamper: func(t *testing.T, f fixture) {
			if err := os.Remove(resultSnapshotPath(f.root, f.cycle)); err != nil {
				t.Fatal(err)
			}
		}, wantErr: "read host explanation snapshot"},
		{name: "workspace manifest differs from the snapshot", sealed: sealedRequiredF3, tamper: func(t *testing.T, f fixture) {
			rewriteViewF3(t, f, true, false, func(v *phaseio.ExplanationView) { v.Reason += " edited" })
		}, wantErr: "does not match the host snapshot"},
		{name: "lineage lookup fails on an unknown authored base", sealed: sealedRequiredF3, tamper: func(t *testing.T, f fixture) {
			rewriteViewF3(t, f, true, true, func(v *phaseio.ExplanationView) { v.AuthoredBaseSHA = strings.Repeat("a", 40) })
		}, wantErr: "verify rebound lineage"},
		{name: "material paths differ from the diff", sealed: sealedRequiredF3, tamper: func(t *testing.T, f fixture) {
			rewriteViewF3(t, f, true, true, func(v *phaseio.ExplanationView) { v.MaterialPaths = []string{"other/file.go"} })
		}, wantErr: "material_paths does not match"},
		{name: "build report removed", sealed: sealedRequiredF3, tamper: func(t *testing.T, f fixture) {
			if err := os.Remove(filepath.Join(f.workspace, "build-report.md")); err != nil {
				t.Fatal(err)
			}
		}, wantErr: "build-report.md is missing"},
		{name: "declaration section removed", sealed: sealedRequiredF3, tamper: func(t *testing.T, f fixture) {
			f.writeWorkspace(t, "build-report.md", "# Build Report\n\nnothing declared\n")
		}, wantErr: "missing the required ## Explanation Documentation section"},
		{name: "declaration repeats a field", sealed: sealedRequiredF3, tamper: func(t *testing.T, f fixture) {
			f.writeWorkspace(t, "build-report.md", "## Explanation Documentation\n- Status: REQUIRED\n- Status: NOT_APPLICABLE\n")
		}, wantErr: "Explanation Documentation"},
		{name: "required handoff no longer declared", sealed: sealedRequiredF3, tamper: func(t *testing.T, f fixture) {
			f.writeWorkspace(t, "build-report.md", RenderNotApplicableDeclaration("changed my mind"))
		}, wantErr: "required explanation handoff does not match the Build declaration"},
		{name: "not-applicable handoff no longer declared", sealed: sealedNotApplicableF3, tamper: func(t *testing.T, f fixture) {
			f.writeWorkspace(t, "build-report.md", RenderNotApplicableDeclaration("a different reason"))
		}, wantErr: "not-applicable explanation handoff does not match the Build declaration"},
		{name: "not-applicable handoff carries a document", sealed: sealedNotApplicableF3, tamper: func(t *testing.T, f fixture) {
			rewriteViewF3(t, f, true, true, func(v *phaseio.ExplanationView) { v.DocumentPath = "docs/explain/builds/x.md" })
		}, wantErr: "not-applicable explanation handoff does not match the Build declaration"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.sealed(t)
			tc.tamper(t, f)

			view, active, err := Verify(context.Background(), f.binding())

			if view != nil || !active || err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("view=%v active=%v err=%v, want an error containing %q", view, active, err, tc.wantErr)
			}
		})
	}
}

func TestVerify_ADiffTooLargeToDigestIsRefusedAfterTheSeal(t *testing.T) {
	f := sealedRequiredF3(t)
	writeSparseF3(t, filepath.Join(f.worktree, "docs", "oversize.md"), 65<<20)

	view, active, err := Verify(context.Background(), f.binding())

	if view != nil || !active || err == nil || !strings.Contains(err.Error(), "hash base-bound diff") {
		t.Fatalf("view=%v active=%v err=%v", view, active, err)
	}
}

func TestVerifyResolvedV1_AWorktreeThatIsNotARepositoryCannotDeriveTheDiff(t *testing.T) {
	f := sealedRequiredF3(t)

	view, active, err := verifyResolvedV1(context.Background(), f.binding(), t.TempDir(), LoadSnapshot)

	if view != nil || !active || err == nil || !strings.Contains(err.Error(), "derive base-bound diff") {
		t.Fatalf("view=%v active=%v err=%v", view, active, err)
	}
}

func TestVerifyResolvedV1_TheViewMustCarryTheActiveSchemaContractCycleAndBase(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*phaseio.ExplanationView)
		wantErr string
	}{
		{name: "unsupported artifact schema", mutate: func(v *phaseio.ExplanationView) { v.SchemaVersion = 99 }, wantErr: "schema/contract version does not match"},
		{name: "unsupported contract version", mutate: func(v *phaseio.ExplanationView) { v.ContractVersion = 99 }, wantErr: "schema/contract version does not match"},
		{name: "another cycle", mutate: func(v *phaseio.ExplanationView) { v.Cycle = 43 }, wantErr: "cycle/base does not match"},
		{name: "another base", mutate: func(v *phaseio.ExplanationView) { v.BaseSHA = strings.Repeat("b", 40) }, wantErr: "cycle/base does not match"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := sealedRequiredF3(t)
			rewriteViewF3(t, f, true, false, tc.mutate)
			loadsTheManifest := func(CycleBinding) (*phaseio.ExplanationView, error) { return Load(f.workspace) }

			view, active, err := verifyResolvedV1(context.Background(), f.binding(), f.worktree, loadsTheManifest)

			if view != nil || !active || err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("view=%v active=%v err=%v, want an error containing %q", view, active, err, tc.wantErr)
			}
		})
	}
}

func TestVerifyResolvedV1_ASnapshotLoaderFailureEndsTheVerification(t *testing.T) {
	f := sealedRequiredF3(t)
	boom := errors.New("snapshot store down")

	view, active, err := verifyResolvedV1(context.Background(), f.binding(), f.worktree, func(CycleBinding) (*phaseio.ExplanationView, error) { return nil, boom })

	if view != nil || !active || !errors.Is(err, boom) {
		t.Fatalf("view=%v active=%v err=%v", view, active, err)
	}
}

func TestVerifyResolved_AnUnsupportedContractVersionIsAnActiveFault(t *testing.T) {
	f := newFixtureF3(t)
	binding := f.binding()
	binding.ContractVersion = 99

	view, active, err := verifyResolved(context.Background(), binding, f.worktree)

	if view != nil || !active || err == nil || !strings.Contains(err.Error(), "unsupported explanation contract version 99") {
		t.Fatalf("view=%v active=%v err=%v", view, active, err)
	}
}

func TestCheckResolved_DispatchesOnTheContractVersion(t *testing.T) {
	f := newFixtureF3(t)
	unsupported := f.binding()
	unsupported.ContractVersion = 99

	failures, err := checkResolved(context.Background(), unsupported)
	if failures != nil || err == nil || !strings.Contains(err.Error(), "unsupported explanation contract version 99") {
		t.Fatalf("failures=%v err=%v", failures, err)
	}

	empty := f.binding()
	empty.Workspace = ""
	if _, err := checkResolved(context.Background(), empty); err == nil || !strings.Contains(err.Error(), "active contract requires") {
		t.Fatalf("version 1 must reach the v1 checker: %v", err)
	}
}

func TestCheckBuildV1_EachPrerequisiteFailureIsAFaultNotAContentFailure(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T, f fixture) CycleBinding
		wantErr string
	}{
		{name: "no worktree", setup: func(t *testing.T, f fixture) CycleBinding {
			b := f.binding()
			b.Worktree = ""
			return b
		}, wantErr: "active contract requires worktree, workspace, and base SHA"},
		{name: "no workspace", setup: func(t *testing.T, f fixture) CycleBinding {
			b := f.binding()
			b.Workspace = ""
			return b
		}, wantErr: "active contract requires worktree, workspace, and base SHA"},
		{name: "no base", setup: func(t *testing.T, f fixture) CycleBinding {
			b := f.binding()
			b.BaseSHA = ""
			return b
		}, wantErr: "active contract requires worktree, workspace, and base SHA"},
		{name: "worktree is not a repository", setup: func(t *testing.T, f fixture) CycleBinding {
			b := f.binding()
			b.Worktree = t.TempDir()
			return b
		}, wantErr: "derive base-bound diff"},
		{name: "diff too large to digest", setup: func(t *testing.T, f fixture) CycleBinding {
			writeSparseF3(t, filepath.Join(f.worktree, "oversize.bin"), 65<<20)
			return f.binding()
		}, wantErr: "hash base-bound diff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixtureF3(t)
			binding := tc.setup(t, f)

			failures, err := checkBuildV1(context.Background(), binding)

			if failures != nil || err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("failures=%v err=%v, want a fault containing %q", failures, err, tc.wantErr)
			}
		})
	}
}

type flipContextF3 struct {
	context.Context
	calls int
}

func (c *flipContextF3) Err() error {
	c.calls++
	if c.calls > 1 {
		return context.Canceled
	}
	return nil
}

func TestRevalidateResult_ACancelDuringTheCheckWinsOverTheRevalidationFault(t *testing.T) {
	ctx := &flipContextF3{Context: context.Background()}
	binding := CycleBinding{ContractVersion: contractV1, Workspace: t.TempDir()}

	view, err := revalidateResult(ctx, binding)

	if view != nil || !errors.Is(err, context.Canceled) || ctx.calls != 2 {
		t.Fatalf("view=%v err=%v err-calls=%d", view, err, ctx.calls)
	}
}

func TestRevalidateResult_WithoutAnActiveCheckTheWorkspaceHandoffIsStillHeldToTheHostContract(t *testing.T) {
	good := phaseio.ExplanationView{SchemaVersion: currentArtifactSchema, DiffSHA256: "digest"}
	cases := []struct {
		name    string
		mutate  func(*phaseio.ExplanationView)
		wantErr string
	}{
		{name: "unsupported artifact schema", mutate: func(v *phaseio.ExplanationView) { v.SchemaVersion = 2 }, wantErr: "does not match the sealed host contract"},
		{name: "another contract version", mutate: func(v *phaseio.ExplanationView) { v.ContractVersion = contractV1 }, wantErr: "does not match the sealed host contract"},
		{name: "another cycle", mutate: func(v *phaseio.ExplanationView) { v.Cycle = 5 }, wantErr: "does not match the sealed host contract"},
		{name: "another base", mutate: func(v *phaseio.ExplanationView) { v.BaseSHA = "abc" }, wantErr: "does not match the sealed host contract"},
		{name: "no diff digest", mutate: func(v *phaseio.ExplanationView) { v.DiffSHA256 = "" }, wantErr: "does not match the sealed host contract"},
		{name: "already rebound", mutate: func(v *phaseio.ExplanationView) { v.AuthoredBaseSHA = "abc" }, wantErr: "does not match the sealed host contract"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			view := good
			tc.mutate(&view)
			if err := atomicwrite.JSON(filepath.Join(workspace, manifestFilename), view); err != nil {
				t.Fatal(err)
			}

			got, err := revalidateResult(context.Background(), CycleBinding{Workspace: workspace})

			if got != nil || err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got=%v err=%v", got, err)
			}
		})
	}
	t.Run("a matching handoff is returned whole", func(t *testing.T) {
		workspace := t.TempDir()
		if err := atomicwrite.JSON(filepath.Join(workspace, manifestFilename), good); err != nil {
			t.Fatal(err)
		}

		got, err := revalidateResult(context.Background(), CycleBinding{Workspace: workspace})

		if err != nil || !SameView(got, &good) {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})
	t.Run("a missing handoff is a load error", func(t *testing.T) {
		got, err := revalidateResult(context.Background(), CycleBinding{Workspace: t.TempDir()})

		if got != nil || err == nil || !strings.Contains(err.Error(), "read build-explanation.json") {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
}

func TestCheckNotApplicable_OnlyAWellFormedDeclarationBecomesAManifest(t *testing.T) {
	cases := []struct {
		name        string
		declaration reportDeclaration
		wantFailure string
	}{
		{name: "status is required", declaration: reportDeclaration{Status: "REQUIRED", Reason: "r"}, wantFailure: "must declare Status: NOT_APPLICABLE"},
		{name: "reason is blank", declaration: reportDeclaration{Status: "NOT_APPLICABLE", Reason: "   "}, wantFailure: "must declare Status: NOT_APPLICABLE"},
		{name: "a document is attached", declaration: reportDeclaration{Status: "NOT_APPLICABLE", Reason: "r", Document: "docs/x.md"}, wantFailure: "with no Document"},
		{name: "reason exceeds the metadata cap", declaration: reportDeclaration{Status: "NOT_APPLICABLE", Reason: strings.Repeat("x", 1001)}, wantFailure: "Reason exceeds 1000 bytes"},
		{name: "reason holds a control character", declaration: reportDeclaration{Status: "NOT_APPLICABLE", Reason: "bad\x07reason"}, wantFailure: "Reason contains control characters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixtureF3(t)

			failures, err := checkNotApplicable(f.binding(), tc.declaration, nil, "digest")

			if err != nil || len(failures) != 1 || !strings.Contains(failures[0], tc.wantFailure) {
				t.Fatalf("failures=%v err=%v", failures, err)
			}
			if _, loadErr := Load(f.workspace); loadErr == nil {
				t.Fatal("a refused declaration must not write a manifest")
			}
		})
	}
	t.Run("the limit itself is accepted and the reason is trimmed", func(t *testing.T) {
		f := newFixtureF3(t)
		reason := strings.Repeat("y", 998)

		failures, err := checkNotApplicable(f.binding(), reportDeclaration{Status: "NOT_APPLICABLE", Reason: " " + reason + " "}, []string{"a.go"}, "digest")

		if err != nil || failures != nil {
			t.Fatalf("failures=%v err=%v", failures, err)
		}
		view, loadErr := Load(f.workspace)
		if loadErr != nil || view.Status != statusNA || view.Reason != reason || view.DiffSHA256 != "digest" || view.BaseSHA != f.base || len(view.MaterialPaths) != 1 {
			t.Fatalf("view=%+v err=%v", view, loadErr)
		}
	})
	t.Run("a workspace outside the project root is a fault", func(t *testing.T) {
		f := newFixtureF3(t)
		binding := f.binding()
		binding.Workspace = t.TempDir()

		failures, err := checkNotApplicable(binding, reportDeclaration{Status: "NOT_APPLICABLE", Reason: "r"}, nil, "digest")

		if failures != nil || err == nil || !strings.Contains(err.Error(), "workspace must be inside the project root") {
			t.Fatalf("failures=%v err=%v", failures, err)
		}
	})
}

func requiredViewF3(f fixture, material ...string) *phaseio.ExplanationView {
	return &phaseio.ExplanationView{
		SchemaVersion: currentArtifactSchema, ContractVersion: contractV1, Status: statusRequired, Cycle: f.cycle,
		BaseSHA: f.base, DocumentPath: cycleDocumentPath(f.cycle, f.runID), MaterialPaths: material,
	}
}

func TestValidateRequired_EveryDefectIsReportedAndOnlyAGitFaultIsAFault(t *testing.T) {
	prepared := func(t *testing.T) (fixture, []string, []string) {
		f := newFixtureF3(t)
		f.prepareRequired(t)
		return f, []string{"config/app.yaml", cycleDocumentPath(f.cycle, f.runID)}, []string{"config/app.yaml"}
	}
	t.Run("a non canonical document path stops at once", func(t *testing.T) {
		f, changed, material := prepared(t)
		view := requiredViewF3(f, material...)
		view.DocumentPath = "docs/explain/builds/other.md"

		failures, fault := validateRequired(context.Background(), f.binding(), changed, material, view)

		if fault != nil || len(failures) != 1 || !strings.Contains(failures[0], "Document path is not canonical") {
			t.Fatalf("failures=%v fault=%v", failures, fault)
		}
	})
	t.Run("a missing base is named and the immutability probe is skipped", func(t *testing.T) {
		f, changed, material := prepared(t)
		view := requiredViewF3(f, material...)
		view.BaseSHA = ""
		binding := f.binding()
		binding.Worktree = t.TempDir()
		docPath := filepath.Join(binding.Worktree, filepath.FromSlash(view.DocumentPath))
		if err := os.MkdirAll(filepath.Dir(docPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(docPath, []byte(f.validDocument()), 0o644); err != nil {
			t.Fatal(err)
		}

		failures, fault := validateRequired(context.Background(), binding, changed, material, view)

		if fault != nil || !containsFailure(failures, "base_sha is required") || containsFailure(failures, "immutable") {
			t.Fatalf("a repository-less worktree would fault if the base probe ran: failures=%v fault=%v", failures, fault)
		}
	})
	t.Run("material paths that differ from the diff are named", func(t *testing.T) {
		f, changed, material := prepared(t)
		view := requiredViewF3(f, "something/else.go")

		failures, fault := validateRequired(context.Background(), f.binding(), changed, material, view)

		if fault != nil || !containsFailure(failures, "manifest material_paths does not match the base-bound diff") {
			t.Fatalf("failures=%v fault=%v", failures, fault)
		}
	})
	t.Run("a worktree that cannot answer for the base is a fault", func(t *testing.T) {
		f, changed, material := prepared(t)
		binding := f.binding()
		binding.Worktree = t.TempDir()

		failures, fault := validateRequired(context.Background(), binding, changed, material, requiredViewF3(f, material...))

		if fault == nil || !strings.Contains(fault.Error(), "cannot verify cycle document immutability") {
			t.Fatalf("failures=%v fault=%v", failures, fault)
		}
	})
	t.Run("a stale document digest is reported and replaced by the real one", func(t *testing.T) {
		f, changed, material := prepared(t)
		view := requiredViewF3(f, material...)
		view.DocumentSHA256 = "stale"
		sum := sha256.Sum256([]byte(f.validDocument()))

		failures, fault := validateRequired(context.Background(), f.binding(), changed, material, view)

		if fault != nil || !containsFailure(failures, "document SHA256 mismatch") || view.DocumentSHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("failures=%v fault=%v sha=%s", failures, fault, view.DocumentSHA256)
		}
	})
	t.Run("a faithful view passes and learns its document digest", func(t *testing.T) {
		f, changed, material := prepared(t)
		view := requiredViewF3(f, material...)

		failures, fault := validateRequired(context.Background(), f.binding(), changed, material, view)

		if fault != nil || len(failures) != 0 || view.DocumentSHA256 == "" {
			t.Fatalf("failures=%v fault=%v", failures, fault)
		}
	})
}

func documentF3(f fixture, edit func(string) string) string {
	return edit(f.validDocument())
}

func TestValidateDocument_EachStructuralDefectHasItsOwnFailure(t *testing.T) {
	f := newFixtureF3(t)
	changed := []string{"config/app.yaml"}
	material := []string{"config/app.yaml"}
	cases := []struct {
		name string
		edit func(string) string
		want string
	}{
		{name: "build binding missing", edit: func(d string) string {
			return strings.Replace(d, "## Build Binding\n- Cycle: 42\n- Base SHA: "+f.base+"\n\n", "", 1)
		}, want: "missing ## Build Binding"},
		{name: "build binding repeated", edit: func(d string) string {
			return d + "\n## Build Binding\n- Cycle: 42\n"
		}, want: "Build Binding"},
		{name: "build binding repeats a field", edit: func(d string) string {
			return strings.Replace(d, "- Cycle: 42\n", "- Cycle: 42\n- Cycle: 43\n", 1)
		}, want: "duplicate cycle fields"},
		{name: "build binding names another cycle", edit: func(d string) string {
			return strings.Replace(d, "- Cycle: 42", "- Cycle: 41", 1)
		}, want: "Build Binding must contain the exact Cycle and Base SHA"},
		{name: "build binding names another base", edit: func(d string) string {
			return strings.Replace(d, "- Base SHA: "+f.base, "- Base SHA: "+strings.Repeat("c", 40), 1)
		}, want: "Build Binding must contain the exact Cycle and Base SHA"},
		{name: "a required heading repeated", edit: func(d string) string {
			return d + "\n## Summary\nA second summary that repeats the heading.\n"
		}, want: "Summary"},
		{name: "a required heading missing", edit: func(d string) string {
			return strings.Replace(d, "## Limitations\nThis does not add per-user configuration.\n", "", 1)
		}, want: "missing ## Limitations"},
		{name: "a required heading too thin", edit: func(d string) string {
			return strings.Replace(d, "Enable the application through its existing configuration field.", "Short.", 1)
		}, want: "## Summary must contain substantive content"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			failures := validateDocument(documentF3(f, tc.edit), f.cycle, f.base, changed, material)

			if !containsFailure(failures, tc.want) {
				t.Fatalf("failures=%v, want one containing %q", failures, tc.want)
			}
			for _, failure := range failures {
				if !strings.HasPrefix(failure, "Explanation Documentation: ") {
					t.Fatalf("every failure carries the contract prefix: %q", failure)
				}
			}
		})
	}
	t.Run("the faithful document has no failure", func(t *testing.T) {
		if failures := validateDocument(f.validDocument(), f.cycle, f.base, changed, material); len(failures) != 0 {
			t.Fatalf("failures=%v", failures)
		}
	})
	t.Run("a section at exactly its minimum size is substantive and one byte less is not", func(t *testing.T) {
		atMinimum := strings.Replace(f.validDocument(), "The schema and setting name remain unchanged.", "None", 1)
		belowMinimum := strings.Replace(f.validDocument(), "The schema and setting name remain unchanged.", "Non", 1)

		if failures := validateDocument(atMinimum, f.cycle, f.base, changed, material); len(failures) != 0 {
			t.Fatalf("4 bytes meet the Compatibility minimum: %v", failures)
		}
		if failures := validateDocument(belowMinimum, f.cycle, f.base, changed, material); !containsFailure(failures, "## Compatibility must contain substantive content") {
			t.Fatalf("3 bytes miss the Compatibility minimum: %v", failures)
		}
	})
}

func TestChangedAreaEntries_CitedPathsAreValidatedBeforeTheyAreRecorded(t *testing.T) {
	changed := []string{"config/app.yaml"}
	explanation := " — flips the existing runtime setting while keeping its schema."
	cases := []struct {
		name        string
		body        string
		wantEntries map[string]string
		wantFailure string
	}{
		{name: "parent traversal", body: "- `../escape.txt`" + explanation, wantFailure: "contains an invalid repo-relative path"},
		{name: "absolute path", body: "- `/etc/passwd`" + explanation, wantFailure: "contains an invalid repo-relative path"},
		{name: "colon in the path", body: "- `a:b`" + explanation, wantFailure: "contains an invalid repo-relative path"},
		{name: "explanation shorter than the minimum", body: "- `config/app.yaml` — too short", wantFailure: "path config/app.yaml needs a what/why explanation"},
		{name: "explanation exactly at the minimum", body: "- `config/app.yaml` — 0123456789", wantEntries: map[string]string{"config/app.yaml": "0123456789"}},
		{name: "one byte under the minimum", body: "- `config/app.yaml` — 012345678", wantFailure: "needs a what/why explanation"},
		{name: "valid citation", body: "- `config/app.yaml`" + explanation, wantEntries: map[string]string{"config/app.yaml": strings.TrimSpace(strings.TrimLeft(explanation, " —"))}},
		{name: "empty body", body: "", wantEntries: map[string]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entries, failures := changedAreaEntries(tc.body, changed)

			if tc.wantFailure == "" {
				if len(failures) != 0 || len(entries) != len(tc.wantEntries) {
					t.Fatalf("entries=%v failures=%v", entries, failures)
				}
				for path, text := range tc.wantEntries {
					if entries[path] != text {
						t.Fatalf("entries[%s]=%q, want %q", path, entries[path], text)
					}
				}
				return
			}
			if len(entries) != 0 || len(failures) != 1 || !strings.Contains(failures[0], tc.wantFailure) {
				t.Fatalf("entries=%v failures=%v", entries, failures)
			}
		})
	}
}

type scriptedFileF3 struct {
	real   *os.File
	reader io.Reader
	stats  []func() (os.FileInfo, error)
	calls  int
}

func (s *scriptedFileF3) Read(p []byte) (int, error) { return s.reader.Read(p) }
func (s *scriptedFileF3) Close() error               { return s.real.Close() }
func (s *scriptedFileF3) Stat() (os.FileInfo, error) {
	i := s.calls
	if i >= len(s.stats) {
		i = len(s.stats) - 1
	}
	s.calls++
	return s.stats[i]()
}

type errReaderF3 struct{}

func (errReaderF3) Read([]byte) (int, error) { return 0, errors.New("disk read failed") }

type zeroReaderF3 struct{}

func (zeroReaderF3) Read(p []byte) (int, error) { return len(p), nil }

func opsWithFileF3(t *testing.T, path string, build func(real *os.File) *scriptedFileF3) regularReadOps {
	t.Helper()
	ops := osRegularReadOps
	ops.open = func(string) (regularReadFile, error) {
		real, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return build(real), nil
	}
	return ops
}

func realStatF3(real *os.File) func() (os.FileInfo, error) {
	return func() (os.FileInfo, error) { return real.Stat() }
}

func TestReadRegularWithinOps_EveryGuardRefusesWithItsOwnMessage(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	if err := os.WriteFile(path, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "other.md")
	if err := os.WriteFile(other, []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	otherInfo := func() (os.FileInfo, error) { return os.Stat(other) }
	failAbs := errors.New("abs failed")
	failEval := errors.New("eval failed")

	cases := []struct {
		name    string
		rel     string
		ops     func(t *testing.T) regularReadOps
		wantErr string
		wantIs  error
	}{
		{name: "traversal path", rel: "../doc.md", ops: func(*testing.T) regularReadOps { return osRegularReadOps }, wantErr: "invalid relative path"},
		{name: "empty path", rel: "", ops: func(*testing.T) regularReadOps { return osRegularReadOps }, wantErr: "invalid relative path"},
		{name: "absolute path", rel: "/etc/passwd", ops: func(*testing.T) regularReadOps { return osRegularReadOps }, wantErr: "invalid relative path"},
		{name: "abs fails", rel: "doc.md", ops: func(*testing.T) regularReadOps {
			ops := osRegularReadOps
			ops.abs = func(string) (string, error) { return "", failAbs }
			return ops
		}, wantIs: failAbs},
		{name: "root cannot be resolved", rel: "doc.md", ops: func(*testing.T) regularReadOps {
			ops := osRegularReadOps
			ops.evalSymlinks = func(string) (string, error) { return "", failEval }
			return ops
		}, wantErr: "resolve worktree"},
		{name: "file cannot be resolved", rel: "doc.md", ops: func(*testing.T) regularReadOps {
			ops := osRegularReadOps
			ops.evalSymlinks = func(p string) (string, error) {
				if p == path {
					return "", failEval
				}
				return filepath.EvalSymlinks(p)
			}
			return ops
		}, wantErr: "escapes the worktree"},
		{name: "open fails", rel: "doc.md", ops: func(*testing.T) regularReadOps {
			ops := osRegularReadOps
			ops.open = func(string) (regularReadFile, error) { return nil, os.ErrPermission }
			return ops
		}, wantErr: "open doc.md without following links", wantIs: os.ErrPermission},
		{name: "stat fails", rel: "doc.md", ops: func(t *testing.T) regularReadOps {
			return opsWithFileF3(t, path, func(real *os.File) *scriptedFileF3 {
				return &scriptedFileF3{real: real, reader: real, stats: []func() (os.FileInfo, error){func() (os.FileInfo, error) { return nil, errors.New("stat failed") }}}
			})
		}, wantErr: "changed type or identity while opening"},
		{name: "opened file is not a regular file", rel: "doc.md", ops: func(t *testing.T) regularReadOps {
			return opsWithFileF3(t, path, func(real *os.File) *scriptedFileF3 {
				return &scriptedFileF3{real: real, reader: real, stats: []func() (os.FileInfo, error){func() (os.FileInfo, error) { return dirInfo, nil }}}
			})
		}, wantErr: "changed type or identity while opening"},
		{name: "opened file is another file", rel: "doc.md", ops: func(t *testing.T) regularReadOps {
			return opsWithFileF3(t, path, func(real *os.File) *scriptedFileF3 {
				return &scriptedFileF3{real: real, reader: real, stats: []func() (os.FileInfo, error){otherInfo}}
			})
		}, wantErr: "changed type or identity while opening"},
		{name: "read fails", rel: "doc.md", ops: func(t *testing.T) regularReadOps {
			return opsWithFileF3(t, path, func(real *os.File) *scriptedFileF3 {
				return &scriptedFileF3{real: real, reader: errReaderF3{}, stats: []func() (os.FileInfo, error){realStatF3(real)}}
			})
		}, wantErr: "read doc.md"},
		{name: "body grows past the cap while reading", rel: "doc.md", ops: func(t *testing.T) regularReadOps {
			return opsWithFileF3(t, path, func(real *os.File) *scriptedFileF3 {
				return &scriptedFileF3{real: real, reader: zeroReaderF3{}, stats: []func() (os.FileInfo, error){realStatF3(real)}}
			})
		}, wantIs: errArtifactTooLarge},
		{name: "second stat fails", rel: "doc.md", ops: func(t *testing.T) regularReadOps {
			return opsWithFileF3(t, path, func(real *os.File) *scriptedFileF3 {
				return &scriptedFileF3{real: real, reader: real, stats: []func() (os.FileInfo, error){realStatF3(real), func() (os.FileInfo, error) { return nil, errors.New("stat failed") }}}
			})
		}, wantErr: "changed while reading"},
		{name: "second stat is another file", rel: "doc.md", ops: func(t *testing.T) regularReadOps {
			return opsWithFileF3(t, path, func(real *os.File) *scriptedFileF3 {
				return &scriptedFileF3{real: real, reader: real, stats: []func() (os.FileInfo, error){realStatF3(real), otherInfo}}
			})
		}, wantErr: "changed while reading"},
		{name: "size changes between the two stats", rel: "doc.md", ops: func(t *testing.T) regularReadOps {
			return opsWithFileF3(t, path, func(real *os.File) *scriptedFileF3 {
				grow := func() (os.FileInfo, error) {
					before, err := real.Stat()
					if err != nil {
						return nil, err
					}
					appender, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
					if err != nil {
						return nil, err
					}
					_, _ = appender.WriteString("more")
					_ = appender.Close()
					if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
						return nil, err
					}
					return real.Stat()
				}
				return &scriptedFileF3{real: real, reader: real, stats: []func() (os.FileInfo, error){realStatF3(real), grow}}
			})
		}, wantErr: "changed while reading"},
		{name: "modification time changes between the two stats", rel: "doc.md", ops: func(t *testing.T) regularReadOps {
			return opsWithFileF3(t, other, func(real *os.File) *scriptedFileF3 {
				touch := func() (os.FileInfo, error) {
					future := time.Now().Add(time.Hour)
					if err := os.Chtimes(other, future, future); err != nil {
						return nil, err
					}
					return real.Stat()
				}
				return &scriptedFileF3{real: real, reader: real, stats: []func() (os.FileInfo, error){realStatF3(real), touch}}
			})
		}, wantErr: "changed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rel := tc.rel
			if tc.name == "modification time changes between the two stats" {
				rel = "other.md"
			}

			body, sum, err := readRegularWithinOps(root, rel, tc.ops(t))

			if body != "" || sum != "" || err == nil {
				t.Fatalf("body=%q sum=%q err=%v", body, sum, err)
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v, want it to contain %q", err, tc.wantErr)
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Fatalf("err=%v, want it to wrap %v", err, tc.wantIs)
			}
		})
	}
}

func TestReadRegularWithin_ReturnsTheBodyAndItsDigestForARegularFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "doc.md"), []byte("héllo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte("héllo\n"))

	body, sum, err := readRegularWithin(root, "doc.md")

	if err != nil || body != "héllo\n" || sum != hex.EncodeToString(want[:]) {
		t.Fatalf("body=%q sum=%q err=%v", body, sum, err)
	}
}

func TestReadRegularWithin_AFileReachedThroughADirectorySymlinkEscapesTheRoot(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}

	body, _, err := readRegularWithin(root, "link/secret.md")

	if body != "" || err == nil || !strings.Contains(err.Error(), "escapes the worktree") {
		t.Fatalf("body=%q err=%v", body, err)
	}
}

func TestReadRegularWithin_AnUnreadableRegularFileFailsTheNoFollowOpen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens a mode-000 file")
	}
	root := t.TempDir()
	path := filepath.Join(root, "locked.md")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, _, err := readRegularWithin(root, "locked.md")

	if !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "open locked.md without following links") {
		t.Fatalf("err=%v", err)
	}
}

func TestOsRegularReadOpsOpen_ReturnsANilInterfaceOnFailureAndAnOpenFileOnSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}

	missing, err := osRegularReadOps.open(filepath.Join(dir, "absent.md"))
	if err == nil || missing != nil {
		t.Fatalf("a failed open must return a nil interface: %v %v", missing, err)
	}
	file, err := osRegularReadOps.open(filepath.Join(dir, "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if body, _ := io.ReadAll(file); string(body) != "abc" {
		t.Fatalf("body=%q", body)
	}
}

func chmodDirF3(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestArchivePredicatePackage_MovesThePackageUnderTheDatedArchive(t *testing.T) {
	worktree := t.TempDir()
	src := filepath.Join(worktree, "pkg", "alpha")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "p.go"), []byte("package alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rel, err := archivePredicatePackage(worktree, "pkg/alpha")

	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rel, "docs/private/research/archived-") || !strings.HasSuffix(rel, "/superseded-predicate-packages/alpha") {
		t.Fatalf("rel = %q", rel)
	}
	if body, err := os.ReadFile(filepath.Join(worktree, filepath.FromSlash(rel), "p.go")); err != nil || string(body) != "package alpha\n" {
		t.Fatalf("archived body=%q err=%v", body, err)
	}
	if _, err := os.Lstat(src); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the source package must be gone: %v", err)
	}
}

func TestArchivePredicatePackage_EachBlockedStepFailsWithoutMovingThePackage(t *testing.T) {
	prepare := func(t *testing.T) (worktree, src string) {
		worktree = t.TempDir()
		src = filepath.Join(worktree, "pkg", "alpha")
		if err := os.MkdirAll(src, 0o755); err != nil {
			t.Fatal(err)
		}
		return worktree, src
	}
	t.Run("the archive directory path is blocked by a file", func(t *testing.T) {
		worktree, src := prepare(t)
		if err := os.WriteFile(filepath.Join(worktree, "docs"), []byte("a file"), 0o644); err != nil {
			t.Fatal(err)
		}

		rel, err := archivePredicatePackage(worktree, "pkg/alpha")

		if rel != "" || err == nil {
			t.Fatalf("rel=%q err=%v", rel, err)
		}
		if _, statErr := os.Lstat(src); statErr != nil {
			t.Fatalf("the package must stay in place: %v", statErr)
		}
	})
	t.Run("the destination cannot be inspected", func(t *testing.T) {
		worktree, src := prepare(t)
		archive := filepath.Join(worktree, archiveDateDir(), "superseded-predicate-packages")
		if err := os.MkdirAll(archive, 0o755); err != nil {
			t.Fatal(err)
		}
		chmodDirF3(t, archive, 0)

		rel, err := archivePredicatePackage(worktree, "pkg/alpha")

		if rel != "" || err == nil || !strings.Contains(err.Error(), "inspect archive destination") {
			t.Fatalf("rel=%q err=%v", rel, err)
		}
		chmodDirF3(t, archive, 0o755)
		if _, statErr := os.Lstat(src); statErr != nil {
			t.Fatalf("the package must stay in place: %v", statErr)
		}
	})
	t.Run("the rename is refused", func(t *testing.T) {
		worktree, src := prepare(t)
		archive := filepath.Join(worktree, archiveDateDir(), "superseded-predicate-packages")
		if err := os.MkdirAll(archive, 0o755); err != nil {
			t.Fatal(err)
		}
		chmodDirF3(t, archive, 0o500)

		rel, err := archivePredicatePackage(worktree, "pkg/alpha")

		if rel != "" || err == nil || !strings.Contains(err.Error(), "archive superseded predicate package pkg/alpha") {
			t.Fatalf("rel=%q err=%v", rel, err)
		}
		if _, statErr := os.Lstat(src); statErr != nil {
			t.Fatalf("the package must stay in place: %v", statErr)
		}
	})
}
