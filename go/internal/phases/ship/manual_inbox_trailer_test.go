package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestShipManual_BranchTrailerSkipsErrorAndEmptyBranchCases(t *testing.T) {
	t.Parallel()
	t.Run("branch lookup error", func(t *testing.T) {
		t.Parallel()
		opts := Options{
			ProjectRoot:   t.TempDir(),
			CommitMessage: "fix: keep message",
			Runner: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
				return 1, errors.New("git failed")
			},
		}
		fillManualBranchInboxTrailer(context.Background(), &opts)
		if opts.CommitMessage != "fix: keep message" {
			t.Fatalf("got %q, want unchanged message", opts.CommitMessage)
		}
	})

	t.Run("detached head", func(t *testing.T) {
		t.Parallel()
		opts := Options{
			ProjectRoot:   t.TempDir(),
			CommitMessage: "fix: keep message",
			Runner: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
				return 128, nil
			},
		}
		fillManualBranchInboxTrailer(context.Background(), &opts)
		if opts.CommitMessage != "fix: keep message" {
			t.Fatalf("got %q, want unchanged message", opts.CommitMessage)
		}
	})

	t.Run("unreadable inbox path", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		evolveDir := filepath.Join(root, ".evolve")
		if err := os.MkdirAll(evolveDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evolveDir, "inbox"), []byte("not a dir"), 0o644); err != nil {
			t.Fatal(err)
		}
		opts := Options{
			ProjectRoot:   root,
			CommitMessage: "fix: keep message",
			Runner: func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
				_, _ = io.WriteString(stdout, "console/branch-widget\n")
				return 0, nil
			},
		}
		fillManualBranchInboxTrailer(context.Background(), &opts)
		if opts.CommitMessage != "fix: keep message" {
			t.Fatalf("got %q, want unchanged message", opts.CommitMessage)
		}
	})
}
