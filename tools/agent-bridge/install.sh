#!/usr/bin/env bash
# install.sh — install bridge into a branch-independent system location
#
# Strategy (v0.2.0): COPY tools/agent-bridge/{bin,lib,drivers,VERSION} into
# $HOME/.local/share/agent-bridge/, then symlink $HOME/.local/bin/bridge to
# the copied bin/bridge. This makes the install survive branch switches in
# the source repo — once installed, the bridge command works regardless of
# what's checked out.
#
# Usage:
#   bash install.sh              # install (copy + symlink)
#   bash install.sh --uninstall  # remove copy + symlink
#   bash install.sh --dry-run    # print actions, do not touch FS
#   bash install.sh --check      # verify install is functional
#
# Exit codes:
#   0   success / install OK / check passed
#   1   --uninstall succeeded
#   10  bad flags
#   20  source binary missing (bin/bridge not found in source tree)
#   21  target dir not writable
#   22  --check failed (see stderr for details)

set -uo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SRC_TREE="$SCRIPT_DIR"
readonly SRC_BRIDGE="${SRC_TREE}/bin/bridge"
readonly INSTALL_PREFIX="${BRIDGE_INSTALL_PREFIX:-${HOME}/.local}"
readonly INSTALL_SHARE="${INSTALL_PREFIX}/share/agent-bridge"
readonly INSTALL_BIN_DIR="${INSTALL_PREFIX}/bin"
readonly INSTALL_SYMLINK="${INSTALL_BIN_DIR}/bridge"

mode="install"
dry_run=0

for arg in "$@"; do
  case "$arg" in
    --uninstall) mode="uninstall" ;;
    --dry-run)   dry_run=1 ;;
    --check)     mode="check" ;;
    -h|--help)
      sed -n '2,21p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "[install] unknown arg: $arg" >&2
      exit 10
      ;;
  esac
done

run() {
  if [[ $dry_run -eq 1 ]]; then
    echo "[install:dry-run] $*"
  else
    "$@"
  fi
}

case "$mode" in
  install)
    if [[ ! -f "$SRC_BRIDGE" ]]; then
      echo "[install] FATAL: source $SRC_BRIDGE not found (run from sub-project root)" >&2
      exit 20
    fi
    run mkdir -p "$INSTALL_SHARE" "$INSTALL_BIN_DIR"
    if [[ ! -w "$INSTALL_BIN_DIR" ]] && [[ $dry_run -eq 0 ]]; then
      echo "[install] FATAL: $INSTALL_BIN_DIR not writable" >&2
      exit 21
    fi

    # Copy the sub-project tree (bin, lib, drivers, VERSION)
    for sub in bin lib drivers VERSION; do
      [[ -e "$SRC_TREE/$sub" ]] || continue
      if [[ -d "$SRC_TREE/$sub" ]]; then
        # Remove old subdir before copying to ensure a clean state
        run rm -rf "$INSTALL_SHARE/$sub"
        run cp -R "$SRC_TREE/$sub" "$INSTALL_SHARE/"
      else
        run cp "$SRC_TREE/$sub" "$INSTALL_SHARE/"
      fi
    done
    run chmod +x "$INSTALL_SHARE/bin/bridge"

    # Symlink the bin entry
    if [[ -L "$INSTALL_SYMLINK" || -e "$INSTALL_SYMLINK" ]]; then
      run rm -f "$INSTALL_SYMLINK"
    fi
    run ln -s "$INSTALL_SHARE/bin/bridge" "$INSTALL_SYMLINK"
    echo "[install] copied $SRC_TREE → $INSTALL_SHARE"
    echo "[install] linked $INSTALL_SYMLINK -> $INSTALL_SHARE/bin/bridge"

    case ":${PATH}:" in
      *":${INSTALL_BIN_DIR}:"*) ;;
      *) echo "[install] note: $INSTALL_BIN_DIR is not on PATH — add it to your shell rc" ;;
    esac
    ;;

  uninstall)
    removed=0
    if [[ -L "$INSTALL_SYMLINK" ]]; then
      run rm -f "$INSTALL_SYMLINK"
      echo "[install] removed symlink $INSTALL_SYMLINK"
      removed=1
    fi
    if [[ -d "$INSTALL_SHARE" ]]; then
      run rm -rf "$INSTALL_SHARE"
      echo "[install] removed copied tree $INSTALL_SHARE"
      removed=1
    fi
    if [[ $removed -eq 1 ]]; then exit 1; fi
    echo "[install] nothing to remove"
    ;;

  check)
    ok=1
    if [[ ! -d "$INSTALL_SHARE" ]]; then
      echo "[install:check] FAIL: copied tree missing at $INSTALL_SHARE (run \`bash install.sh\`)" >&2
      ok=0
    elif [[ ! -f "$INSTALL_SHARE/bin/bridge" ]]; then
      echo "[install:check] FAIL: $INSTALL_SHARE/bin/bridge missing" >&2
      ok=0
    fi
    if [[ ! -L "$INSTALL_SYMLINK" ]]; then
      echo "[install:check] FAIL: $INSTALL_SYMLINK is not a symlink (run \`bash install.sh\`)" >&2
      ok=0
    elif [[ "$(readlink "$INSTALL_SYMLINK")" != "$INSTALL_SHARE/bin/bridge" ]]; then
      echo "[install:check] FAIL: symlink points to $(readlink "$INSTALL_SYMLINK") (expected $INSTALL_SHARE/bin/bridge)" >&2
      ok=0
    fi
    if ! command -v bridge >/dev/null 2>&1; then
      echo "[install:check] FAIL: 'bridge' not on PATH (is $INSTALL_BIN_DIR in your PATH?)" >&2
      ok=0
    fi
    schema=""
    if ! version_str=$(bridge --json version 2>/dev/null); then
      echo "[install:check] FAIL: 'bridge --json version' did not run cleanly" >&2
      ok=0
    elif command -v jq >/dev/null 2>&1; then
      schema=$(echo "$version_str" | jq -r '.schema_version' 2>/dev/null)
      if [[ "$schema" != "1" ]]; then
        echo "[install:check] WARN: schema_version=$schema (expected 1)" >&2
      fi
    fi
    if [[ $ok -eq 1 ]]; then
      echo "[install:check] OK: bridge installed at $INSTALL_SYMLINK (share=$INSTALL_SHARE, schema_version=${schema:-?})"
      exit 0
    fi
    exit 22
    ;;
esac

exit 0
