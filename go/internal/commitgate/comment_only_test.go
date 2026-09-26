package commitgate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const commentedAtHead = "package x\n\n// F is the entry point.\n// It used to live in cycle 42's retry loop.\nfunc F() {}\n"

const trimmedComment = "package x\n\n// F is the entry point.\nfunc F() {}\n"

type waiverCase struct {
	files     string
	noRenames string
	filesFlag string
	onDisk    map[string]string
	links     map[string]string
	atHead    map[string]string
	wantRun   int
}

func runWaiverCase(t *testing.T, c waiverCase) *Result {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	for p, body := range c.onDisk {
		mustWrite(t, filepath.Join(root, p), body)
	}
	for p, target := range c.links {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, p)); err != nil {
			t.Fatal(err)
		}
	}
	if c.noRenames == "" {
		c.noRenames = c.files
	}
	rules := []scriptRule{
		{matchPrefix: "git diff --name-only --no-renames HEAD", stdout: c.noRenames},
		{matchPrefix: "git diff --name-only HEAD", stdout: c.files},
	}
	for _, p := range strings.Fields(c.noRenames + " " + c.files + " " + c.filesFlag) {
		if body, ok := c.atHead[p]; ok {
			rules = append(rules, scriptRule{matchPrefix: "git show HEAD:" + p, stdout: body})
		} else {
			rules = append(rules, scriptRule{matchPrefix: "git show HEAD:" + p, exit: 128})
		}
	}
	rules = append(rules,
		scriptRule{matchPrefix: "git diff HEAD", stdout: "diff --git a/x.go b/x.go\n", exit: 1},
		scriptRule{matchPrefix: "gofmt -s -l"},
		scriptRule{matchPrefix: "go vet"},
		scriptRule{matchPrefix: "go test"},
	)
	o := baseOpts(root, "shasum", "go")
	o.Env = os.Environ()
	o.Files = c.filesFlag
	o.Runner = (&scriptRunner{rules: rules}).run()
	res := o.Run(context.Background())
	if res.ExitCode != c.wantRun {
		t.Fatalf("ExitCode = %d, want %d (%v)", res.ExitCode, c.wantRun, res.Logs)
	}
	return res
}

func TestRun_AProvenCommentOnlyChangeNeedsNoReviewers(t *testing.T) {
	t.Parallel()
	res := runWaiverCase(t, waiverCase{
		files:   "x.go\ndocs/architecture/packages/x.md\n",
		onDisk:  map[string]string{"x.go": trimmedComment, "docs/architecture/packages/x.md": "# x\n"},
		atHead:  map[string]string{"x.go": commentedAtHead},
		wantRun: ExitPass,
	})
	if res.Attestation == nil || len(res.Attestation.ReviewersRun) != 0 {
		t.Fatalf("a waived change records no reviewer, so ship adds no Reviewed-by trailer: %+v", res.Attestation)
	}
	if !strings.Contains(strings.Join(res.Logs, "\n"), "comment-only") {
		t.Fatalf("the log names the proof that replaced review: %v", res.Logs)
	}
}

func TestRun_AnyChangeBeyondCommentRemovalStillNeedsReviewers(t *testing.T) {
	t.Parallel()
	const moved = "package x\n\nfunc F() {\n\ty := g() //nolint:errcheck\n\t_ = y\n}\n\nfunc g() error { return nil }\n"
	for name, c := range map[string]waiverCase{
		"a code change": {
			files: "x.go\n", atHead: map[string]string{"x.go": commentedAtHead},
			onDisk: map[string]string{"x.go": "package x\n\nfunc F() { _ = 1 }\n"},
		},
		"a moved directive": {
			files: "x.go\n", atHead: map[string]string{"x.go": moved},
			onDisk: map[string]string{"x.go": strings.Replace(moved, "\ty := g() //nolint:errcheck\n\t_ = y", "\ty := g()\n\t//nolint:errcheck\n\t_ = y", 1)},
		},
		"an added Go file": {
			files: "x.go\n", onDisk: map[string]string{"x.go": "package x\n"},
		},
		"a deleted Go file": {
			files: "x.go\n", atHead: map[string]string{"x.go": commentedAtHead},
		},
		"a docs-only change": {
			files:  "docs/plans/p.md\n",
			atHead: map[string]string{"docs/plans/p.md": "# old plan\n"},
			onDisk: map[string]string{"docs/plans/p.md": "# plan\n"},
		},
		"a persona beside a comment-only edit": {
			files:  "x.go\nagents/evolve-builder.md\n",
			atHead: map[string]string{"x.go": commentedAtHead, "agents/evolve-builder.md": "old\n"},
			onDisk: map[string]string{"x.go": trimmedComment, "agents/evolve-builder.md": "new\n"},
		},
		"a non-Markdown file under docs": {
			files:  "x.go\ndocs/architecture/phase-registry.json\n",
			atHead: map[string]string{"x.go": commentedAtHead, "docs/architecture/phase-registry.json": "{}\n"},
			onDisk: map[string]string{"x.go": trimmedComment, "docs/architecture/phase-registry.json": "{\"a\":1}\n"},
		},
		"a docs Markdown path that is a symlink": {
			files:  "x.go\ndocs/x.md\n",
			atHead: map[string]string{"x.go": commentedAtHead},
			onDisk: map[string]string{"x.go": trimmedComment, "go/secret.go": "package secret\n"},
			links:  map[string]string{"docs/x.md": "../go/secret.go"},
		},
		"a Go file replaced by a symlink": {
			files:  "x.go\n",
			atHead: map[string]string{"x.go": commentedAtHead},
			onDisk: map[string]string{"real.go": commentedAtHead},
			links:  map[string]string{"x.go": "real.go"},
		},
		"a comment edit under testdata": {
			files:  "p/testdata/fixture.go\n",
			atHead: map[string]string{"p/testdata/fixture.go": commentedAtHead},
			onDisk: map[string]string{"p/testdata/fixture.go": trimmedComment},
		},
		"a rename whose source only the no-renames listing shows": {
			files:     "x.go\ndocs/archive/x.md\n",
			noRenames: "x.go\nagents/x.md\ndocs/archive/x.md\n",
			atHead:    map[string]string{"x.go": commentedAtHead, "agents/x.md": "persona\n"},
			onDisk:    map[string]string{"x.go": trimmedComment, "docs/archive/x.md": "persona\n"},
		},
		"a file list passed with --files": {
			files:     "x.go\ncore.go\n",
			filesFlag: "x.go",
			atHead:    map[string]string{"x.go": commentedAtHead, "core.go": "package x\n"},
			onDisk:    map[string]string{"x.go": trimmedComment, "core.go": "package x\n\nvar Y = 1\n"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			c.wantRun = ExitFail
			res := runWaiverCase(t, c)
			if !strings.Contains(strings.Join(res.Logs, "\n"), "missing required review capability") {
				t.Fatalf("expected the reviewer precondition to apply: %v", res.Logs)
			}
		})
	}
}

func TestRun_ADeniedWaiverSaysWhy(t *testing.T) {
	t.Parallel()
	res := runWaiverCase(t, waiverCase{
		files:   "x.go\n",
		atHead:  map[string]string{"x.go": commentedAtHead},
		onDisk:  map[string]string{"x.go": "package x\n\nfunc F() { _ = 1 }\n"},
		wantRun: ExitFail,
	})
	if !strings.Contains(strings.Join(res.Logs, "\n"), "x.go: code changed") {
		t.Fatalf("the log names the file and the reason review is still required: %v", res.Logs)
	}
}

func TestRun_ARealGitRenameCannotHideItsSourceFromTheWaiver(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	mustWrite(t, filepath.Join(root, "x.go"), commentedAtHead)
	mustWrite(t, filepath.Join(root, "agents", "x.md"), "persona\n")
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	mustWrite(t, filepath.Join(root, "x.go"), trimmedComment)
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	git("mv", "agents/x.md", "docs/x.md")
	git("add", "x.go")

	o := Options{RepoRoot: root, Runner: sysexec.DefaultRunner}
	why, refused := o.reviewWaiver(context.Background())
	if why != "" || refused != "agents/x.md: not a regular file" {
		t.Fatalf("a persona moved into docs/ must not ride a comment-only waiver: waived=%q refused=%q", why, refused)
	}
}
