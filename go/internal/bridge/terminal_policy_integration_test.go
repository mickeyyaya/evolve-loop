//go:build integration && darwin

package bridge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
)

func TestRealTmuxSandboxTerminalConfinement(t *testing.T) {
	requireTmux(t)
	probe := sandbox.Probe()
	if probe.OS != "darwin" || !probe.Available || !probe.CapabilityChecked || !probe.Capable {
		t.Skipf("UNVERIFIED native macOS sandbox: %+v", probe)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed; native terminal raw-mode fixture requires it")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	peer := itSession("terminal-peer")
	if err := itTmuxCtl.NewSession(ctx, peer, 80, 24); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = itTmuxCtl.KillSession(context.Background(), peer) })
	peerTTY, err := itTmuxCtl.run(ctx, "display-message", "-p", "-t", peer, "#{pane_tty}")
	if err != nil {
		t.Fatal(err)
	}
	peerTTY = strings.TrimSpace(peerTTY)
	// Establish that the independently owned peer exists and is writable by
	// this user, so a missing device or ordinary Unix permissions cannot pass
	// the child's negative test.
	peerFile, err := os.OpenFile(peerTTY, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("peer tty control unavailable: %v", err)
	}
	t.Cleanup(func() { _ = peerFile.Close() })
	var peerGroup int32
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, peerFile.Fd(), syscall.TIOCGETD, uintptr(unsafe.Pointer(&peerGroup))); errno != 0 {
		t.Fatalf("unwrapped terminal ioctl control failed: %v", errno)
	}

	cfg := itConfig(t)
	cfg.RequireSandbox = true
	cfg.AllowNetwork = true
	cfg.ArtifactTimeoutS = 5
	protected := filepath.Join(cfg.Worktree, "protected")
	mustWrite(t, protected, "retained")
	// runTmuxREPL receives denials already canonicalized by Engine.Launch.
	cfg.DenyPaths, err = resolveSandboxDenials([]string{protected}, cfg.ProjectRoot, cfg.Worktree, true)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, cfg.PromptFile, "ARTIFACT="+cfg.Artifact)
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const marker = "TERMINAL-FIXTURE-READY"
	// Resolve the assigned tty from the CHILD's inherited stdin, independently
	// of the parent's pane_tty lookup. Keep the ready marker only in script
	// contents, never in the launch command captured by tmux.
	script := fmt.Sprintf(`const fs=require('node:fs'), tty=require('node:tty'), cp=require('node:child_process');
const peer=%q, protectedPath=%q, artifact=%q, marker=%q, testBinary=%q;
function denied(action, name) {
  try { action(); } catch (err) {
    if (err.code==='EPERM' || err.code==='EACCES') return;
    throw new Error(name+' failed inconclusively: '+err.message);
  }
  throw new Error(name+' unexpectedly permitted');
}
const assigned=cp.execFileSync('/usr/bin/tty',{stdio:['inherit','pipe','pipe'],encoding:'utf8'}).trim();
if (assigned===peer) throw new Error('assigned tty is the peer fixture');
for (const path of ['/dev/tty',assigned]) {
  const fd=fs.openSync(path,'r+');
  try { fs.writeSync(fd,'terminal write verified\n'); } finally { fs.closeSync(fd); }
}
process.stdin.setRawMode(true);
process.stdin.setRawMode(false);
try {
 cp.execFileSync(testBinary,['-test.run=^TestSandboxTerminalIoctlFixture$'],{
  stdio:['inherit','pipe','pipe'],env:{...process.env,EVOLVE_TERMINAL_IOCTL_FIXTURE:'1'}
 });
} catch(err) { throw new Error('ioctl fixture: '+String(err.stdout)+' '+String(err.stderr)); }
denied(()=>{const fd=fs.openSync(peer,'r+'); fs.closeSync(fd);},'peer tty write');
let peerStream;
try {
  peerStream=new tty.ReadStream(fs.openSync(peer,'r'));
  denied(()=>{peerStream.setRawMode(true); peerStream.setRawMode(false);},'peer tty ioctl');
} finally { if(peerStream) peerStream.destroy(); }
denied(()=>{const fd=fs.openSync(protectedPath,'a'); fs.closeSync(fd);},'protected file write');
let received='', completed=false;
process.stdin.setEncoding('utf8');
process.stdin.on('data',chunk=>{
  received+=chunk;
  if(!completed && received.includes('ARTIFACT='+artifact)) {
    fs.writeFileSync(artifact,'terminal-policy-verified');
    completed=true;
  }
  console.log(marker);
});
console.log(marker);
process.stdin.resume();
`, peerTTY, protected, cfg.Artifact, marker, testBinary)
	child := filepath.Join(cfg.Worktree, "terminal-fixture.js")
	mustWrite(t, child, script)
	deps := itDeps(120 * time.Millisecond)
	deps.Env = map[string]string{"PATH": "/var/run/codex.system/bootstrap/usr/bin:" + os.Getenv("PATH")}
	deps.LookupEnv = mapLookup(nil)
	if !sandbox.DetectNested(depEnvGetter(deps)) {
		t.Fatal("fixture must retain the Codex bootstrap session hint")
	}
	// Rebind after configuring the environment: itDeps has already defaulted
	// the wrapper closure. Use the measured native probe, never a fake success.
	deps.SandboxWrap = defaultSandboxWrapWithProbe(deps, func() sandbox.ProbeResult { return probe })
	var stderr bytes.Buffer
	deps.Stderr = &stderr
	session := itSession("terminal-policy")
	t.Cleanup(func() { _ = itTmuxCtl.KillSession(context.Background(), session) })
	launch := shellQuotePOSIX(node) + " " + shellQuotePOSIX(child)
	rc, err := runTmuxREPL(ctx, cfg, deps, itLaunch(session, launch, marker, 0, false))
	if err != nil || rc != ExitOK {
		t.Fatalf("native terminal fixture failed: rc=%d err=%v\n%s\n%s", rc, err, &stderr,
			readFile(t, filepath.Join(cfg.Workspace, "tmux-final-scrollback.txt")))
	}
	if got := readFile(t, cfg.Artifact); got != "terminal-policy-verified" {
		t.Fatalf("artifact=%q, want completed terminal-policy proof", got)
	}
	if got := readFile(t, protected); got != "retained" {
		t.Fatalf("protected file=%q, want retained", got)
	}
}

// Invoked only inside the native terminal fixture's sandbox. TIOCGETD is a
// harmless query, supported by the unwrapped control, outside the allowlist.
func TestSandboxTerminalIoctlFixture(t *testing.T) {
	if os.Getenv("EVOLVE_TERMINAL_IOCTL_FIXTURE") != "1" {
		return
	}
	terminal, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()
	// Different clients choose immediate, drained, or flushed attribute updates.
	var attrs syscall.Termios
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, terminal.Fd(), syscall.TIOCGETA, uintptr(unsafe.Pointer(&attrs))); errno != 0 {
		t.Fatalf("read termios: %v", errno)
	}
	for _, command := range []uintptr{syscall.TIOCSETA, syscall.TIOCSETAW, syscall.TIOCSETAF} {
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, terminal.Fd(), command, uintptr(unsafe.Pointer(&attrs))); errno != 0 {
			t.Fatalf("set termios %#x: %v", command, errno)
		}
	}
	var group int32
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, terminal.Fd(), syscall.TIOCGETD, uintptr(unsafe.Pointer(&group)))
	if errno != syscall.EPERM && errno != syscall.EACCES {
		t.Fatalf("unlisted terminal ioctl: got %v, want permission denial", errno)
	}
}
