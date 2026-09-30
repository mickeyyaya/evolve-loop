package ciparitygate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type tierLog struct {
	workspace string
	args      []string
	path      string
}

func (l *tierLog) target() string { return filepath.Join(l.workspace, "integration-tier.log") }

func (l *tierLog) append(n int, note string, a attempt) error {
	if l.workspace == "" {
		return nil
	}
	p := l.target()
	entry := fmt.Sprintf("# attempt %d%s\n# go %s\n# exit: %d\n\n%s\n%s\n", n, note, strings.Join(l.args, " "), a.code, a.out, a.errOut)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.WriteString(entry)
	if err == nil {
		l.path = p
	}
	return err
}

func (g *Gates) logAttempt(req Request, l *tierLog, n int, note string, a attempt) {
	if err := l.append(n, note, a); err != nil {
		g.warn(gateTier, req, CodeTierLogWriteFailed, "integration-tier.log write failed: "+err.Error(), "path", l.target(), "attempt", strconv.Itoa(n), "err", err.Error())
	}
}

func offendersWithLogPointer(a attempt, logPath string) []string {
	offenders := offenderLines(strings.TrimSpace(a.out + "\n" + a.errOut))
	if logPath != "" {
		offenders = append(offenders, "full output: "+logPath)
	}
	return offenders
}
