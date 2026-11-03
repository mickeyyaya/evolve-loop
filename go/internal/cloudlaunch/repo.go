package cloudlaunch

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Repo struct {
	Branch string
	Remote string
	URL    string
	Ahead  int
}

type Refusal struct{ Reason string }

func (r *Refusal) Error() string { return r.Reason }

func refuse(format string, args ...any) error {
	return &Refusal{Reason: fmt.Sprintf(format, args...)}
}

func ReadRepo(ctx context.Context, run sysexec.RunFunc, dir string) (Repo, error) {
	var r Repo
	var err error
	if r.Branch, err = sysexec.Output(ctx, run, dir, "git", "symbolic-ref", "--quiet", "--short", "HEAD"); err != nil {
		return r, refuse("the tree %s is on no branch: the cloud session clones a branch: %v", dir, err)
	}
	if r.Remote, err = sysexec.Output(ctx, run, dir, "git", "config", "branch."+r.Branch+".remote"); err != nil {
		return r, refuse("branch %s has no upstream: push it with git push -u first", r.Branch)
	}
	if r.URL, err = sysexec.Output(ctx, run, dir, "git", "remote", "get-url", r.Remote); err != nil {
		return r, refuse("the remote %s of branch %s has no URL: %v", r.Remote, r.Branch, err)
	}
	ahead, err := sysexec.Output(ctx, run, dir, "git", "rev-list", "--count", "@{upstream}..HEAD")
	if err != nil {
		return r, refuse("branch %s and its upstream cannot be compared: fetch the remote first: %v", r.Branch, err)
	}
	if r.Ahead, err = strconv.Atoi(ahead); err != nil {
		return r, refuse("branch %s: the count of unpushed commits %q is not a number: %v", r.Branch, ahead, err)
	}
	return r, nil
}

func (r Repo) Check(hosts []string) error {
	if host := RemoteHost(r.URL); !slices.Contains(hosts, host) {
		return refuse("the remote %s of branch %s is on %q, not on a host of the manifest (%s): the cloud session clones only from those hosts", r.Remote, r.Branch, host, strings.Join(hosts, ", "))
	}
	if r.Ahead > 0 {
		return refuse("branch %s has %d commit(s) that are not pushed to %s: the cloud session clones the remote branch, so push first", r.Branch, r.Ahead, r.Remote)
	}
	return nil
}

func RemoteHost(remote string) string {
	host, _ := hostAndPath(remote)
	return host
}

func RepoSlug(remote string) string {
	host, path := hostAndPath(remote)
	if host == "" {
		return ""
	}
	return strings.TrimSuffix(strings.Trim(path, "/"), ".git")
}

func hostAndPath(remote string) (string, string) {
	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err != nil {
			return "", ""
		}
		return strings.ToLower(u.Hostname()), u.Path
	}
	hostPart, path, scp := strings.Cut(remote, ":")
	if !scp {
		return "", ""
	}
	_, host, _ := strings.Cut(hostPart, "@")
	if host == "" {
		host = hostPart
	}
	return strings.ToLower(host), path
}
