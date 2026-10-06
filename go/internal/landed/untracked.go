package landed

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

func ListUntracked(ctx context.Context, worktree gitexec.Git) ([]UntrackedFile, error) {
	listed, err := gitIn(ctx, worktree, "", "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	names := strings.FieldsFunc(listed, func(r rune) bool { return r == 0 })
	if len(names) == 0 {
		return nil, nil
	}
	slices.Sort(names)
	if i := slices.IndexFunc(names, func(n string) bool { return strings.Contains(n, "\n") }); i >= 0 {
		return nil, fmt.Errorf("untracked %q has a newline in its name, so its content cannot be compared with origin/main", names[i])
	}
	hashes, err := gitIn(ctx, worktree, strings.Join(names, "\n")+"\n", "hash-object", "--stdin-paths")
	if err != nil {
		return nil, err
	}
	blobs := strings.Fields(hashes)
	if len(blobs) != len(names) {
		return nil, fmt.Errorf("git hash-object returned %d hashes for %d untracked files", len(blobs), len(names))
	}
	files := make([]UntrackedFile, len(names))
	for i, n := range names {
		files[i] = UntrackedFile{Name: n, Hash: blobs[i]}
	}
	return files, nil
}

func Untracked(ctx context.Context, worktree gitexec.Git, files []UntrackedFile) (Verdict, error) {
	if len(files) == 0 {
		return landedVerdict, nil
	}
	var query strings.Builder
	for _, f := range files {
		query.WriteString(plane.OriginMainRef + ":" + f.Name + "\n")
	}
	out, err := gitIn(ctx, worktree, query.String(), "cat-file", "--batch-check=%(objectname)")
	if err != nil {
		return Verdict{}, err
	}
	answers := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(answers) != len(files) {
		return Verdict{}, fmt.Errorf("git cat-file answered %d of %d untracked files", len(answers), len(files))
	}
	for i, f := range files {
		switch {
		case answers[i] == f.Hash:
		case strings.HasSuffix(answers[i], " missing"):
			return notLanded("untracked %s is not in origin/main", f.Name), nil
		default:
			return notLanded("untracked %s differs from origin/main", f.Name), nil
		}
	}
	return landedVerdict, nil
}

func gitIn(ctx context.Context, g gitexec.Git, stdin string, args ...string) (string, error) {
	var out, errb strings.Builder
	code, err := g.Exec(ctx, "git", g.Dir, args, nil, strings.NewReader(stdin), &out, &errb)
	if err != nil || code != 0 {
		return "", fmt.Errorf("git %s: rc=%d err=%v: %s", strings.Join(args[:min(2, len(args))], " "), code, err, firstLine(errb.String()))
	}
	return out.String(), nil
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
