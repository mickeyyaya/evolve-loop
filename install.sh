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
# Pin a release:  EVO_VERSION=v22.0.1 curl ... | sh    (default: latest, via the
# releases/latest/download redirect — no GitHub API call, no rate limit).
if [ -n "${EVO_VERSION:-}" ]; then
	RELEASE_BASE="${EVO_RELEASE_BASE:-https://github.com/${REPO}/releases/download/${EVO_VERSION}}"
else
	RELEASE_BASE="${EVO_RELEASE_BASE:-https://github.com/${REPO}/releases/latest/download}"
fi
ASSET_BASE="${EVO_ASSET_BASE:-$RELEASE_BASE}"            # point at a 404 to force the build path
SOURCE_TARBALL="${EVO_SOURCE_TARBALL:-https://github.com/${REPO}/archive/refs/heads/main.tar.gz}"
INSTALL_LIB="${EVO_INSTALL_LIB:-$HOME/.evolve-loop}"     # binary + skill payload live here
BIN_DIR_DEFAULT="$HOME/.local/bin"
FORCE_BUILD="${EVO_FORCE_BUILD:-0}"                      # test seam
# Corporate air-gap path (--binary <path>): install a pre-approved local
# artifact instead of downloading or building — no compiler, no network, one
# approved fingerprint. Set --checksums (or EVO_CHECKSUMS) to a checksums.txt to
# verify the artifact's SHA256 against the approved fingerprint before install.
BINARY_PATH=""
EVO_CHECKSUMS="${EVO_CHECKSUMS:-}"

log()  { printf '%s\n' "evolve-install: $*"; }
warn() { printf '%s\n' "evolve-install: WARNING: $*" >&2; }
die()  { printf '%s\n' "evolve-install: ERROR: $*" >&2; exit 1; }

usage() {
	cat <<'EOF'
evolve-loop installer.

Usage:
  curl -fsSL .../install.sh | sh                 # default: prebuilt, else build
  sh install.sh --binary <path> [--checksums <f>] # air-gap: install a local artifact

Options:
  --binary <path>      Install a pre-approved local artifact (a release
                       evolve_<os>_<arch>.tar.gz, or a raw evolve binary) instead
                       of downloading or building. No compiler, no network.
  --checksums <path>   checksums.txt to verify --binary against (default: a
                       checksums.txt beside <path>, or $EVO_CHECKSUMS). Only used
                       with --binary. The corporate single-fingerprint path.
  -h, --help           Show this help.

Env: EVO_VERSION, EVO_INSTALL_LIB, EVO_CHECKSUMS, EVO_NO_MODIFY_PATH, EVO_FORCE_BUILD.
EOF
}

# parse_args consumes the optional flags (--binary/--checksums/--help). The
# default curl|sh invocation passes none, so this is a no-op there.
parse_args() {
	while [ $# -gt 0 ]; do
		case "$1" in
			--binary)      shift; [ $# -gt 0 ] || die "--binary requires a <path> argument"; BINARY_PATH="$1" ;;
			--binary=*)    BINARY_PATH="${1#--binary=}" ;;
			--checksums)   shift; [ $# -gt 0 ] || die "--checksums requires a <path> argument"; EVO_CHECKSUMS="$1" ;;
			--checksums=*) EVO_CHECKSUMS="${1#--checksums=}" ;;
			-h|--help)     usage; exit 0 ;;
			*)             die "unknown argument: $1 (try --help)" ;;
		esac
		shift
	done
}

SUDO=""
need_sudo() {
	if [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1; then SUDO="sudo"; else SUDO=""; fi
}

# windows_help fires under native Windows (Git Bash / MSYS / Cygwin). The loop
# runtime needs a Unix shell (tmux, bash, OS sandbox), so WSL2 is the path —
# inside WSL, uname reports Linux and this same one-liner just works.
windows_help() {
	warn "Windows detected ($uname_s)."
	warn "evolve-loop's runtime needs a Unix shell (tmux, bash), so install it under WSL2:"
	warn "  1. Install WSL2:  https://learn.microsoft.com/windows/wsl/install"
	warn "  2. Open your WSL (e.g. Ubuntu) shell, then run this same one-liner there:"
	warn "       curl -fsSL https://mickeyyaya.github.io/evolve-loop/install.sh | sh"
	warn "Tip: the /evo:* skills also work in native Claude Code on Windows —"
	warn "     /plugin marketplace add mickeyyaya/evolve-loop  then  /plugin install evo@evo"
	die "use WSL2 for the runtime (steps above)."
}

# ---- 1. OS / arch ----------------------------------------------------------
# Sets PLATFORM (= goreleaser <os>_<arch>) for the prebuilt download. An OS/arch
# with no prebuilt asset falls through to building from source (FORCE_BUILD), so
# the installer always works even on platforms outside the release matrix.
detect_platform() {
	uname_s="$(uname -s)"
	uname_m="$(uname -m)"
	case "$uname_s" in
		Darwin)  OS=darwin ;;
		Linux)   OS=linux ;; # WSL2 reports Linux — works unchanged
		FreeBSD) OS=freebsd ;;
		OpenBSD) OS=openbsd ;;
		NetBSD)  OS=netbsd ;;
		MINGW*|MSYS*|CYGWIN*|Windows_NT) windows_help ;;
		*) OS="" ;; # unknown OS — build from source
	esac
	case "$uname_m" in
		x86_64|amd64)            ARCH=amd64 ;;
		aarch64|arm64)           ARCH=arm64 ;;
		armv7l|armv6l|armhf|arm) ARCH=arm ;;
		i386|i486|i586|i686)     ARCH=386 ;;
		riscv64)                 ARCH=riscv64 ;;
		ppc64le)                 ARCH=ppc64le ;;
		s390x)                   ARCH=s390x ;;
		*) ARCH="" ;; # unknown arch — build from source
	esac
	# Rosetta 2 makes uname -m report x86_64 on Apple Silicon; prefer the native
	# arm64 binary (rustup's hw.optional.arm64 probe — also catches x86_64 shells
	# on arm64 kernels, unlike proc_translated).
	if [ "$OS" = darwin ] && [ "$ARCH" = amd64 ]; then
		if (sysctl -n hw.optional.arm64 2>/dev/null || true) | grep -q '^1$'; then
			log "Rosetta 2 detected — installing native arm64 binary"
			ARCH=arm64
		fi
	fi
	if [ -z "$OS" ] || [ -z "$ARCH" ]; then
		warn "no prebuilt binary for $uname_s/$uname_m — building from source"
		FORCE_BUILD=1
		PLATFORM="unsupported"
	else
		PLATFORM="${OS}_${ARCH}" # matches goreleaser <os>_<arch> (lowercase)
	fi
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
		# Pin the transport (rustup's flags), feature-detected ONCE so ancient
		# curls degrade with a warning instead of failing every fetch.
		if [ -z "${CURL_TLS_CHECKED:-}" ]; then
			CURL_TLS_CHECKED=1
			if curl --proto =https --tlsv1.2 --version >/dev/null 2>&1; then
				CURL_TLS="--proto =https --tlsv1.2"
			else
				CURL_TLS=""
				warn "curl lacks --proto/--tlsv1.2 — not enforcing TLS 1.2 (old curl)"
			fi
		fi
		# $CURL_TLS is deliberately unquoted: word-splitting yields the two flags.
		# shellcheck disable=SC2086
		curl -fsSL $CURL_TLS --retry 3 -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$2" "$1"
	else
		die "neither curl nor wget available to download $1"
	fi
}

# have_sha256_tool reports whether any SHA256 tool is on PATH — lets a caller
# distinguish "cannot verify" from "verified" (a false "verified" claim is worse
# than an honest "unverified" for a corporate approval fingerprint).
have_sha256_tool() {
	command -v sha256sum >/dev/null 2>&1 ||
		command -v shasum >/dev/null 2>&1 ||
		command -v openssl >/dev/null 2>&1
}

# sha256_of <file> — echo the file's SHA256 hex, or nothing if no tool is found.
sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	elif command -v openssl >/dev/null 2>&1; then
		openssl dgst -sha256 "$1" | awk '{print $NF}'
	fi
}

verify_checksum() {  # verify_checksum <file> <name-in-sums> <checksums.txt>
	got="$(sha256_of "$1")"
	if [ -z "$got" ]; then
		warn "no sha256 tool; skipping integrity check"
		return 0
	fi
	want="$(awk -v n="$2" '$2 == n {print $1}' "$3")"
	[ -n "$want" ] && [ "$want" = "$got" ]
}

try_prebuilt() {
	if [ "$FORCE_BUILD" = 1 ]; then log "skipping prebuilt — building from source"; return 1; fi
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

# ---- 4b. Air-gap local artifact (--binary) --------------------------------
# Install a pre-approved local artifact: a release evolve_<os>_<arch>.tar.gz
# (binary + skill payload, fingerprinted in checksums.txt) or a raw evolve
# binary. Verified against a checksums.txt when available — the corporate
# single-fingerprint path: no compiler, no network, one approved fingerprint.
install_local_binary() {
	[ -f "$BINARY_PATH" ] || die "--binary artifact not found: $BINARY_PATH"
	name="$(basename "$BINARY_PATH")"
	sums="$EVO_CHECKSUMS"
	[ -z "$sums" ] && sums="$(dirname "$BINARY_PATH")/checksums.txt"
	if [ ! -f "$sums" ]; then
		warn "no checksums.txt for $name (pass --checksums or set EVO_CHECKSUMS) — installing WITHOUT fingerprint verification"
	elif ! have_sha256_tool; then
		# Honest "cannot verify" instead of a false "verified" — a corporate
		# approval fingerprint must never be claimed when nothing was hashed.
		warn "no sha256 tool (sha256sum/shasum/openssl) — cannot verify $name against $sums; installing UNVERIFIED"
	elif verify_checksum "$BINARY_PATH" "$name" "$sums"; then
		log "checksum verified: $name matches $sums"
	else
		die "checksum verification failed for $name against $sums (mismatch or not listed) — refusing to install (not the approved artifact)"
	fi
	mkdir -p "$INSTALL_LIB"
	case "$name" in
		*.tar.gz|*.tgz)
			tar -xzf "$BINARY_PATH" -C "$INSTALL_LIB" || die "could not extract $BINARY_PATH"
			[ -x "$INSTALL_LIB/evolve" ] || die "archive $name has no evolve binary at its root"
			;;
		*)
			# Raw binary: the operator supplies the skill payload separately (or it
			# is already staged in $INSTALL_LIB from a prior install).
			cp "$BINARY_PATH" "$INSTALL_LIB/evolve" || die "could not copy $BINARY_PATH"
			chmod +x "$INSTALL_LIB/evolve"
			[ -d "$INSTALL_LIB/skills" ] || warn "no skill payload in $INSTALL_LIB — a raw --binary needs skills/ staged there; prefer the release .tar.gz"
			;;
	esac
	INSTALLED_BIN="$INSTALL_LIB/evolve"
	log "installed approved binary → $INSTALLED_BIN"
}

# print_fingerprint echoes the installed binary's SHA256 — the value to record
# in a corporate approval request. There is no global pin to set at install time:
# the ship-SHA integrity pin is per-project and version-aware, so it auto-adopts
# on the first cycle in each target repo (a cross-version SHA change re-pins itself).
print_fingerprint() {
	[ -n "${INSTALLED_BIN:-}" ] && [ -x "$INSTALLED_BIN" ] || return 0
	fp="$(sha256_of "$INSTALLED_BIN")"
	[ -n "$fp" ] && log "installed binary fingerprint (SHA256): $fp"
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
		*) add_to_shell_rc "$bindir" ;;
	esac
	log "linked evolve → $EVO_BIN"
}

# add_to_shell_rc <bindir> — append an idempotent, marked PATH line to the
# user's shell rc (bun-style $SHELL switch). Writes $HOME as a literal so the
# rc file stays machine-portable. Opt out: EVO_NO_MODIFY_PATH=1.
add_to_shell_rc() {
	# Portable $HOME-literal form when bindir lives under $HOME.
	case "$1" in
		"$HOME"/*) path_line="export PATH=\"\$HOME${1#"$HOME"}:\$PATH\" # evolve-loop" ;;
		*)         path_line="export PATH=\"$1:\$PATH\" # evolve-loop" ;;
	esac
	if [ "${EVO_NO_MODIFY_PATH:-0}" = 1 ]; then
		warn "$1 is not on your PATH (EVO_NO_MODIFY_PATH=1 — not touching shell config)."
		warn "  add manually:  $path_line"
		return 0
	fi
	case "$(basename "${SHELL:-sh}")" in
		zsh)  rc="${ZDOTDIR:-$HOME}/.zshrc" ;;
		bash) rc="$HOME/.bashrc"; [ -f "$HOME/.bash_profile" ] && rc="$HOME/.bash_profile" ;;
		fish) rc="$HOME/.config/fish/conf.d/evolve.fish"
		      mkdir -p "$(dirname "$rc")" 2>/dev/null || true
		      path_line="fish_add_path '$1' # evolve-loop" ;;
		*)    rc="$HOME/.profile" ;;
	esac
	if [ -f "$rc" ] && grep -F "# evolve-loop" "$rc" >/dev/null 2>&1; then
		: # already added by a previous run
	elif printf '\n%s\n' "$path_line" >> "$rc" 2>/dev/null; then
		log "added $1 to PATH in $rc"
	else
		warn "$1 is not on your PATH and $rc is not writable."
		warn "  add manually:  $path_line"
		return 0
	fi
	warn "restart your shell, or run now:  export PATH=\"$1:\$PATH\""
}

# ---- 7. Install skills + verify --------------------------------------------
run_evolve_install() {
	log "installing skills for your CLI(s)..."
	# EVOLVE_PROJECT_ROOT points evolve at the bundled payload. Do NOT set CI
	# (that flips install into validate-only). </dev/null keeps it non-interactive.
	EVOLVE_PROJECT_ROOT="$INSTALL_LIB" "$INSTALLED_BIN" install </dev/null || die "evolve install failed"
}

final_checks() {
	# Actually executing the binary catches wrong-arch downloads, quarantine,
	# and corrupt extractions that a mere existence check misses.
	ver="$("$EVO_BIN" version 2>/dev/null || true)"
	[ -n "$ver" ] || warn "evolve did not report a version"
	"$EVO_BIN" doctor probe tmux >/dev/null 2>&1 || warn "tmux probe failed (run 'evolve doctor' to diagnose)"
	log "installed: ${ver:-evolve} → $EVO_BIN"
	if command -v evolve >/dev/null 2>&1; then
		log "done. Next: evolve doctor    then    /evo:loop --cycles 3 \"your goal\""
	else
		log "done. Next (new shells will just say 'evolve'): $EVO_BIN doctor"
	fi
	log "uninstall: evolve uninstall   (and: rm -rf $INSTALL_LIB)"
}

main() {
	parse_args "$@"
	detect_platform
	detect_pkgmgr
	need_sudo
	detect_llm_cli
	if [ -n "$BINARY_PATH" ]; then
		# Air-gap corporate path: no compiler, no download, no package-manager
		# network calls. tmux is a runtime need, but the locked-down env manages
		# its own tools — probe it (pure, no pkg_install network attempt) and only
		# warn, since ensure_tool would try to apt/dnf-install a missing tmux.
		command -v tmux >/dev/null 2>&1 ||
			warn "tmux not found — evolve's runtime needs it; install it via your environment's own tooling"
		install_local_binary
	else
		ensure_tool git git soft
		ensure_tool jq jq soft
		ensure_tool tmux tmux hard
		if ! try_prebuilt; then build_from_source; fi
	fi
	place_on_path
	run_evolve_install
	final_checks
	print_fingerprint
}

main "$@"
