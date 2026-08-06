#!/usr/bin/env bash
#
# Verify that the loxilb binary inside a built deb reports the version that was
# actually requested for the release.
#
# usage: check-deb-version.sh <deb-path> <expected-version>
#
# The release version is carried by the git tag and stamped into the binary at
# build time, so nothing in the source tree records it. That makes this check
# the thing that decides whether the packaging pipeline did its job: it opens
# the artifact that is about to be published and asks the binary itself.
#
# An expected version of "latest"/"" is a rolling build with no fixed version
# and is skipped.

set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <deb-path> <expected-version>" >&2
  exit 1
fi

deb_path="$1"
expected="$2"

expected="${expected#refs/tags/}"
expected="${expected#v}"

if [[ -z "$expected" || "$expected" == "latest" ]]; then
  echo "check-deb-version: rolling build, nothing to verify"
  exit 0
fi

if [[ ! -f "$deb_path" ]]; then
  echo "check-deb-version: deb package not found: $deb_path" >&2
  exit 1
fi

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

dpkg-deb -x "$deb_path" "$workdir"

binary="$workdir/usr/local/sbin/loxilb"
if [[ ! -x "$binary" ]]; then
  echo "check-deb-version: no loxilb binary at usr/local/sbin/loxilb inside $deb_path" >&2
  exit 1
fi

# `loxilb --version` prints a "loxilb start" banner before the version line, so
# select the line rather than taking the first one.
if ! version_out="$("$binary" --version 2>&1)"; then
  echo "check-deb-version: could not run the packaged binary" >&2
  printf '%s\n' "$version_out" >&2
  exit 1
fi

reported="$(printf '%s\n' "$version_out" | awk '/^loxilb version:/ {print $3; exit}')"

if [[ -z "$reported" ]]; then
  echo "check-deb-version: could not parse a version out of:" >&2
  printf '%s\n' "$version_out" >&2
  exit 1
fi

# The tag carries no pre-release suffix while a dev fallback build does, so
# 0.9.8.8 is satisfied by either 0.9.8.8 or 0.9.8.8-<suffix>, but not by
# 0.9.8.85 and not by the in-source dev placeholder.
if [[ "$reported" == "$expected" || "$reported" == "$expected"-* ]]; then
  echo "check-deb-version: OK -- $(basename "$deb_path") reports '$reported'"
  exit 0
fi

cat >&2 <<EOF
check-deb-version: VERSION MISMATCH
  requested release : $expected
  binary reports    : $reported
  package           : $deb_path

The build did not stamp the release version into the binary. The packaging
build must compile from an exact tag checkout, or pass it explicitly with
'make VERSION=$expected'. Publishing this artifact would ship a binary that
misreports its own version.
EOF
exit 1
