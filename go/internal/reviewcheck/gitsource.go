package reviewcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxstamps"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type grantSurface struct {
	dir    string
	decode func(raw []byte) (Grants, error)
}

var grantSurfaces = []grantSurface{
	{".evolve/profiles/", decodeProfileGrants},
	{"go/internal/bridge/manifests/", decodeManifestGrants},
}

func decodeProfileGrants(raw []byte) (Grants, error) {
	var p profiles.Profile
	err := json.Unmarshal(raw, &p)
	g := Grants{AllowedTools: p.AllowedTools}
	if p.Sandbox != nil {
		g.WriteSubpaths = p.Sandbox.WriteSubpaths
	}
	return g, err
}

func decodeManifestGrants(raw []byte) (Grants, error) {
	var m bridge.Manifest
	err := json.Unmarshal(raw, &m)
	return Grants{EnvPassthrough: m.EnvPassthrough}, err
}

func InboxIntegrityAt(ctx context.Context, git gitexec.Git, r Range) ([]Refusal, error) {
	q := newGitQuery(ctx, git, r)
	change := InboxChange{
		BasePaths: q.lines("ls-tree", "-r", "--name-only", q.base, "--", inboxstamps.Pathspec),
		HeadPaths: q.lines("ls-tree", "-r", "--name-only", q.head, "--", inboxstamps.Pathspec),
		Renamed:   map[string]string{},
		Bodies:    q.bodies(),
		ReadBase: func(path string) ([]byte, error) {
			raw := q.show(q.base, path)
			return raw, q.err
		},
	}
	for _, line := range q.lines("diff", "--name-status", "-M", "--diff-filter=R", q.base, q.head) {
		if fields := strings.Split(line, "\t"); len(fields) == 3 {
			change.Renamed[fields[1]] = fields[2]
		}
	}
	if q.err != nil {
		return nil, q.err
	}
	return InboxIntegrity(change)
}

func PermissionWideningAt(ctx context.Context, git gitexec.Git, r Range) ([]Refusal, error) {
	q := newGitQuery(ctx, git, r)
	var changes []FileChange
	for _, s := range grantSurfaces {
		for _, line := range q.lines("diff", "--name-status", "-M", q.base, q.head, "--", s.dir) {
			fields := strings.Split(line, "\t")
			status, from, to := fields[0], fields[1], fields[len(fields)-1]
			if !strings.HasSuffix(to, ".json") {
				continue
			}
			change := FileChange{Path: to}
			if status != "A" {
				change.Base = q.grants(s, q.base, from)
			}
			if status != "D" {
				change.Head = q.grants(s, q.head, to)
			}
			changes = append(changes, change)
		}
	}
	tracked := map[string]bool{}
	for _, path := range q.lines("ls-tree", "-r", "--name-only", q.head) {
		tracked[path] = true
	}
	bodies := q.bodies()
	if q.err != nil {
		return nil, q.err
	}
	return PermissionWidening(changes, bodies, func(path string) bool { return tracked[path] }), nil
}

type gitQuery struct {
	ctx  context.Context
	git  gitexec.Git
	base string
	head string
	err  error
}

func newGitQuery(ctx context.Context, git gitexec.Git, r Range) *gitQuery {
	q := &gitQuery{ctx: ctx, git: git}
	q.base = q.commit(r.Base)
	q.head = q.commit(r.Head)
	return q
}

func (q *gitQuery) commit(rev string) string {
	if q.err != nil {
		return ""
	}
	sha := q.output("rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	if q.err != nil {
		q.err = fmt.Errorf("%q is not a commit: %w", rev, q.err)
	}
	return sha
}

func (q *gitQuery) output(args ...string) string {
	if q.err != nil {
		return ""
	}
	out, err := q.git.Output(q.ctx, append([]string{"-c", "core.quotePath=false"}, args...)...)
	q.err = err
	return out
}

func (q *gitQuery) lines(args ...string) []string {
	out := q.output(args...)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func (q *gitQuery) show(rev, path string) []byte {
	return []byte(q.output("show", rev+":"+path))
}

func (q *gitQuery) grants(s grantSurface, rev, path string) *Grants {
	raw := q.show(rev, path)
	if q.err != nil || len(raw) == 0 {
		return &Grants{}
	}
	g, err := s.decode(raw)
	if err != nil {
		q.err = fmt.Errorf("permission trace: parse %s at %s: %w", path, rev, err)
	}
	return &g
}

func (q *gitQuery) bodies() []string {
	return strings.Split(q.output("log", "--format=%B%x00", q.base+".."+q.head), "\x00")
}
