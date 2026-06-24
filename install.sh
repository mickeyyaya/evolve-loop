#!/bin/sh
# evolve-loop one-line installer.
#
#   curl -fsSL https://mickeyyaya.github.io/evolve-loop/install.sh | sh
#
# Prefer a prebuilt binary; fall back to building from source. Auto-installs the
# few dependency tools it needs, then installs evolve for whichever AI CLI(s) you
# have on PATH (Claude Code, Codex, Antigravity). Idempotent — safe to re-run.
#
# Wary of `curl | sh`? Inspect first:
#   curl -fsSL https://mickeyyaya.github.io/evolve-loop/install.sh -o install.sh
#   less install.sh && sh install.sh
#
# POSIX sh — runs under dash / busybox ash / bash. No bashisms, no pipefail.
set -eu

# ---- Config (env-overridable so CI/tests can redirect) ---------------------
REPO="mickeyyaya/evolve-loop"
RELEASE_BASE="${EVO_RELEASE_BASE:-https://github.com/${REPO}/releases/latest/download}"
ASSET_BASE="${EVO_ASSET_BASE:-$RELEASE_BASE}"            # point at a 404 to force the build path
SOURCE_TARBALL="${EVO_SOURCE_TARBALL:-https://github.com/${REPO}/archive/refs/heads/main.tar.gz}"
INSTALL_LIB="${EVO_INSTALL_LIB:-$HOME/.evolve-loop}"     # binary + skill payload live here
BIN_DIR_DEFAULT="$HOME/.local/bin"
FORCE_BUILD="${EVO_FORCE_BUILD:-0}"                      # test seam

log()  { printf '%s\n' "evolve-install: $*"; }
warn() { printf '%s\n' "evolve-install: WARNING: $*" >&2; }
die()  { printf '%s\n' "evolve-install: ERROR: $*" >&2; exit 1; }

SUDO=""
need_sudo() {
	if [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1; then SUDO="sudo"; else SUDO=""; fi
}

# ---- 1. OS / arch ----------------------------------------------------------
detect_platform() {
	uname_s="$(uname -s)"
	uname_m="$(uname -m)"
	case "$uname_s" in
		Darwin) OS=darwin ;;
		Linux)  OS=linux ;;
		*) die "unsupported OS '$uname_s' (prebuilt binaries: darwin/linux). Build manually: clone the repo, then 'cd go && go build ./cmd/evolve'." ;;
	esac
	case "$uname_m" in
		x86_64|amd64)  ARCH=amd64 ;;
		aarch64|arm64) ARCH=arm64 ;;
		*) die "unsupported arch '$uname_m' (prebuilt binaries: amd64/arm64)." ;;
	esac
	PLATFORM="${OS}_${ARCH}"   # matches goreleaser {{.Os}}_{{.Arch}} (lowercase)
}

# ---- 2. Package manager + dependency tools ---------------------------------
detect_pkgmgr() {
	if   command -v brew    >/dev/null 2>&1; then PKG=brew
	elif command -v apt-get >/dev/null 2>&1; then PKG=apt
	elif command -v dnf     >/dev/null 2>&1; then PKG=dnf
	elif command -v yum     >/dev/null 2>&1; then PKG=yum
	elif command -v pacman  >/dev/null 2>&1; then PKG=pacman
	elif command -v apk     >/dev/null 2>&1; then PKG=apk
	else PKG=none
	fi
}

pkg_install() {  # pkg_install <package-name>
	case "$PKG" in
		brew)   brew install "$1" ;;
		apt)    $SUDO apt-get update -qq && $SUDO apt-get install -y "$1" ;;
		dnf)    $SUDO dnf install -y "$1" ;;
		yum)    $SUDO yum install -y "$1" ;;
		pacman) $SUDO pacman -Sy --noconfirm "$1" ;;
		apk)    $SUDO apk add --no-cache "$1" ;;
		none)   return 1 ;;
	esac
}

ensure_tool() {  # ensure_tool <command> <package> <hard|soft>
	if command -v "$1" >/dev/null 2>&1; then return 0; fi
	log "missing dependency '$1' — installing ($2)"
	if [ "$PKG" = none ]; then
		if [ "$3" = hard ]; then die "no supported package manager; install '$1' manually then re-run"; fi
		warn "no package manager; skipping optional '$1'"
		return 0
	fi
	if pkg_install "$2" && command -v "$1" >/dev/null 2>&1; then
		return 0
	fi
	if [ "$3" = hard ]; then die "could not install '$1' via $PKG; install it manually then re-run"; fi
	warn "could not install optional '$1' (continuing)"
	return 0
}

ensure_go() {
	if command -v go >/dev/null 2>&1; then return 0; fi
	log "missing 'go' (needed to build from source) — installing"
	# Go's distro package name varies; everything else is pkg_install's dispatch.
	case "$PKG" in
		apt)     go_pkg=golang-go ;;
		dnf|yum) go_pkg=golang ;;
		none)    die "build fallback needs Go and no package manager is available; install Go from https://go.dev/dl then re-run" ;;
		*)       go_pkg=go ;; # brew / pacman / apk
	esac
	pkg_install "$go_pkg" || true
	command -v go >/dev/null 2>&1 || die "Go install failed or not on PATH; install from https://go.dev/dl then re-run"
}

# ---- 3. LLM CLI presence (warn-only — never auto-install a CLI) -------------
detect_llm_cli() {
	have=""
	for c in claude codex gemini agy; do
		if command -v "$c" >/dev/null 2>&1; then have="$have $c"; fi
	done
	if [ -z "$have" ]; then
		warn "no AI CLI found (claude / codex / gemini / agy). evolve-loop needs at least one."
		warn "Install Claude Code (https://docs.claude.com/claude-code) or another supported CLI,"
		warn "then re-run this installer — skills will be projected for whichever you add."
	else
		log "detected AI CLI(s):$have"
	fi
}

# ---- 4. Download + verify --------------------------------------------------
fetch() {  # fetch <url> <dest>  (curl or wget; non-zero on HTTP error)
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --retry 3 -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$2" "$1"
	else
		die "neither curl nor wget available to download $1"
	fi
}

verify_checksum() {  # verify_checksum <file> <name-in-sums> <checksums.txt>
	if command -v sha256sum >/dev/null 2>&1; then
		got="$(sha256sum "$1" | awk '{print $1}')"
	elif command -v shasum >/dev/null 2>&1; then
		got="$(shasum -a 256 "$1" | awk '{print $1}')"
	else
		warn "no sha256 tool; skipping integrity check"
		return 0
	fi
	want="$(awk -v n="$2" '$2 == n {print $1}' "$3")"
	[ -n "$want" ] && [ "$want" = "$got" ]
}

try_prebuilt() {
	if [ "$FORCE_BUILD" = 1 ]; then log "EVO_FORCE_BUILD=1 — skipping prebuilt"; return 1; fi
	asset="evolve_${PLATFORM}.tar.gz"
	url="${ASSET_BASE}/${asset}"
	tmp="$(mktemp -d "${TMPDIR:-/tmp}/evolve.XXXXXX")"
	log "trying prebuilt: $url"
	if ! fetch "$url" "$tmp/$asset"; then
		rm -rf "$tmp"; log "no prebuilt asset for $PLATFORM — building from source"; return 1
	fi
	if fetch "${ASSET_BASE}/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
		if ! verify_checksum "$tmp/$asset" "$asset" "$tmp/checksums.txt"; then
			rm -rf "$tmp"; die "checksum mismatch for $asset — refusing to install (corrupt or tampered download)"
		fi
		log "checksum verified"
	else
		warn "checksums.txt unavailable — skipping integrity check"
	fi
	mkdir -p "$INSTALL_LIB"
	if ! tar -xzf "$tmp/$asset" -C "$INSTALL_LIB"; then
		rm -rf "$tmp"; log "archive extraction failed — building from source"; return 1
	fi
	rm -rf "$tmp"
	if [ ! -x "$INSTALL_LIB/evolve" ]; then log "archive missing evolve binary — building from source"; return 1; fi
	INSTALLED_BIN="$INSTALL_LIB/evolve"
	log "installed prebuilt binary → $INSTALLED_BIN"
	return 0
}

# ---- 5. Build fallback -----------------------------------------------------
build_from_source() {
	need_sudo
	ensure_tool git git hard
	ensure_go
	src="$(mktemp -d "${TMPDIR:-/tmp}/evolve-src.XXXXXX")"
	log "building from source"
	if git clone --depth 1 "https://github.com/${REPO}.git" "$src/evolve-loop" >/dev/null 2>&1; then
		repo="$src/evolve-loop"
	else
		fetch "$SOURCE_TARBALL" "$src/src.tar.gz" || { rm -rf "$src"; die "could not obtain source (git clone and tarball both failed)"; }
		tar -xzf "$src/src.tar.gz" -C "$src" || { rm -rf "$src"; die "source tarball corrupt"; }
		repo="$(find "$src" -maxdepth 1 -type d -name 'evolve-loop*' | head -n1)"
	fi
	[ -d "$repo/go" ] || { rm -rf "$src"; die "unexpected source layout (no go/ dir)"; }
	mkdir -p "$INSTALL_LIB"
	( cd "$repo/go" && CGO_ENABLED=0 go build -trimpath -o "$INSTALL_LIB/evolve" ./cmd/evolve ) \
		|| { rm -rf "$src"; die "go build failed"; }
	# Stage the skill payload beside the binary (installer reads it from disk).
	for p in .claude-plugin agents skills docs; do
		if [ -e "$repo/$p" ]; then rm -rf "${INSTALL_LIB:?}/$p"; cp -R "$repo/$p" "$INSTALL_LIB/"; fi
	done
	rm -rf "$src"
	INSTALLED_BIN="$INSTALL_LIB/evolve"
	log "built binary → $INSTALLED_BIN"
}

# ---- 6. PATH ---------------------------------------------------------------
place_on_path() {
	bindir="$BIN_DIR_DEFAULT"
	mkdir -p "$bindir" 2>/dev/null || bindir=""
	if [ -z "$bindir" ] || [ ! -w "$bindir" ]; then
		if [ -w /usr/local/bin ]; then
			bindir=/usr/local/bin; LN="ln -sf"
		else
			need_sudo
			if [ -n "$SUDO" ]; then bindir=/usr/local/bin; LN="$SUDO ln -sf"
			else die "no writable bin dir found; add '$INSTALL_LIB' to PATH manually"; fi
		fi
	else
		LN="ln -sf"
	fi
	$LN "$INSTALLED_BIN" "$bindir/evolve"
	EVO_BIN="$bindir/evolve"
	case ":$PATH:" in
		*":$bindir:"*) : ;;
		*) warn "$bindir is not on your PATH. Add to your shell profile:"
		   warn "  export PATH=\"$bindir:\$PATH\"" ;;
	esac
	log "linked evolve → $EVO_BIN"
}

# ---- 7. Install skills + verify --------------------------------------------
run_evolve_install() {
	log "installing skills for your CLI(s)..."
	# EVOLVE_PROJECT_ROOT points evolve at the bundled payload. Do NOT set CI
	# (that flips install into validate-only). </dev/null keeps it non-interactive.
	EVOLVE_PROJECT_ROOT="$INSTALL_LIB" "$INSTALLED_BIN" install </dev/null || die "evolve install failed"
}

final_checks() {
	"$EVO_BIN" version >/dev/null 2>&1 || warn "evolve did not report a version"
	"$EVO_BIN" doctor probe tmux >/dev/null 2>&1 || warn "tmux probe failed (run 'evolve doctor' to diagnose)"
	log "done. Next: evolve doctor    then    /evo:loop --cycles 3 \"your goal\""
	log "uninstall: evolve uninstall   (and: rm -rf $INSTALL_LIB)"
}

main() {
	detect_platform
	detect_pkgmgr
	need_sudo
	ensure_tool git git soft
	ensure_tool jq jq soft
	ensure_tool tmux tmux hard
	detect_llm_cli
	if ! try_prebuilt; then build_from_source; fi
	place_on_path
	run_evolve_install
	final_checks
}

main "$@"
