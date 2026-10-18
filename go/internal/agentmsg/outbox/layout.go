package outbox

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
)

const draftSuffix = ".json"

type Layout struct {
	Root  string
	Cycle int
}

func (l Layout) cycleDir() string {
	return filepath.Join(l.Root, ".evolve", "msg", strconv.Itoa(l.Cycle))
}

func keyOf(dispatch string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(dispatch))
}

func (l Layout) OutDir(dispatch string) string {
	return filepath.Join(l.cycleDir(), keyOf(dispatch), "out")
}

func (l Layout) BoxDir(dispatch string) string {
	return filepath.Join(l.cycleDir(), keyOf(dispatch), "box")
}

func (l Layout) Locate(path string) (dispatch, id, resolved string, err error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", "", fmt.Errorf("outbox: draft path: %w", err)
	}
	base, err := filepath.EvalSymlinks(l.cycleDir())
	if err != nil {
		return "", "", "", fmt.Errorf("outbox: cycle dir: %w", err)
	}
	rel, inside := strings.CutPrefix(real, base+string(filepath.Separator))
	parts := strings.Split(rel, string(filepath.Separator))
	if !inside || len(parts) != 3 || parts[1] != "out" || !strings.HasSuffix(parts[2], draftSuffix) {
		return "", "", "", fmt.Errorf("outbox: %s is not <cycle>/<key>/out/<id>%s", path, draftSuffix)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || keyOf(string(raw)) != parts[0] {
		return "", "", "", fmt.Errorf("outbox: %q is not the key of a dispatch", parts[0])
	}
	id = strings.TrimSuffix(parts[2], draftSuffix)
	if !envelope.ValidID(id) {
		return "", "", "", fmt.Errorf("outbox: %q is not a draft id", id)
	}
	return string(raw), id, real, nil
}
