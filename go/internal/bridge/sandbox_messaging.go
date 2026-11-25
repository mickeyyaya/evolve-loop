package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/channelstore"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/outbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type MessagingIdentity struct {
	Cycle    int
	Dispatch string
}

func withMessaging(cfg sandbox.Config, id *MessagingIdentity) (sandbox.Config, error) {
	scope, err := messagingScope(id, cfg.RepoRoot)
	if err == nil {
		err = ensureMessagingDirs(scope)
	}
	cfg.Messaging = scope
	return cfg, err
}

func messagingScope(id *MessagingIdentity, root string) (*sandbox.MessagingScope, error) {
	if id == nil {
		return nil, nil
	}
	if id.Dispatch == "" || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("messaging identity needs a dispatch and an absolute project root (dispatch %q, root %q)", id.Dispatch, root)
	}
	root = filepath.Clean(root)
	if _, err := canonicalWithin(root, outbox.TreeRel); err != nil {
		return nil, fmt.Errorf("messaging tree: %w", err)
	}
	chRel := strings.TrimPrefix(paths.EventChannelsRoot(paths.EvolveDirOf(root)), root+string(filepath.Separator))
	chRoot, err := canonicalWithin(root, chRel)
	if err != nil {
		return nil, fmt.Errorf("messaging store: %w", err)
	}
	storeDirs, err := storeChannelDirs(chRoot, id.Cycle)
	if err != nil {
		return nil, err
	}
	layout := outbox.Layout{Root: root, Cycle: id.Cycle}
	return &sandbox.MessagingScope{
		Tree:        layout.TreeDir(),
		Dispatch:    layout.DispatchDir(id.Dispatch),
		Out:         layout.OutDir(id.Dispatch),
		StorePrefix: filepath.Join(chRoot, channelstore.ChannelPrefix),
		StoreDirs:   storeDirs,
	}, nil
}

func storeChannelDirs(chRoot string, cycle int) ([]string, error) {
	entries, err := os.ReadDir(chRoot)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("messaging store: %w", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), channelstore.ChannelPrefix) {
			dirs = append(dirs, filepath.Join(chRoot, e.Name()))
		}
	}
	for _, name := range channelstore.Channels(cycle) {
		if dir := filepath.Join(chRoot, name); !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}
	return dirs, nil
}

func ensureMessagingDirs(scope *sandbox.MessagingScope) error {
	if scope == nil {
		return nil
	}
	for _, dir := range append([]string{scope.Tree}, scope.StoreDirs...) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("messaging directory: %w", err)
		}
	}
	return nil
}
