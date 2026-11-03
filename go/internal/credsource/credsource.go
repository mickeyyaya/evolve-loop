package credsource

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	Service     = "evolve-loop"
	DefaultHost = "github.com"
)

var ErrNoCredential = errors.New("credential source: no credential for the host")

var ChildVariables = []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"}

var hostName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*$`)

type Runner func(ctx context.Context, name string, args ...string) (string, error)

type Answer struct {
	Source string
	Secret string
}

type Source interface {
	Name() string
	Lookup(ctx context.Context, host string) (Answer, error)
}

type Deps struct {
	Run  Runner
	Home func() (string, error)
}

var realizations = map[string]func(Deps) Source{
	"darwin":  func(d Deps) Source { return keychain{run: d.Run} },
	"linux":   func(d Deps) Source { return libsecretOrFile{run: d.Run, home: d.Home, read: os.ReadFile} },
	"windows": func(d Deps) Source { return credentialManager{run: d.Run} },
}

func For(goos string, d Deps) (Source, error) {
	realize, ok := realizations[goos]
	if !ok {
		return nil, fmt.Errorf("credential source: no realization for %s", goos)
	}
	return guarded{inner: realize(d)}, nil
}

func Default() (Source, error) {
	return For(runtime.GOOS, Deps{Run: execRunner, Home: passwdHome})
}

type guarded struct {
	inner Source
}

func (g guarded) Name() string { return g.inner.Name() }

func (g guarded) Lookup(ctx context.Context, host string) (Answer, error) {
	if !hostName.MatchString(host) {
		return Answer{}, fmt.Errorf("credential source: %q is not a host name", host)
	}
	return g.inner.Lookup(ctx, host)
}

func secretAnswer(source, out string) (Answer, error) {
	secret := strings.TrimSpace(out)
	if secret == "" {
		return Answer{}, ErrNoCredential
	}
	return Answer{Source: source, Secret: secret}, nil
}

func execRunner(ctx context.Context, name string, args ...string) (string, error) {
	out, err := sysexec.Command(ctx, name, args...).Output()
	return string(out), err
}

var lookupHome = func(uid string) (string, error) {
	u, err := user.LookupId(uid)
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}

func passwdHome() (string, error) {
	uid := strconv.Itoa(os.Getuid())
	home, err := lookupHome(uid)
	if err != nil {
		return "", fmt.Errorf("credential source: the passwd entry of uid %s: %w", uid, err)
	}
	if home == "" {
		return "", fmt.Errorf("credential source: uid %s has no home directory", uid)
	}
	return home, nil
}
