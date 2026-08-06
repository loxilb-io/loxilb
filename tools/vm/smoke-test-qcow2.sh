#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <qcow2-path> [timeout-seconds] [expected-version]" >&2
  exit 1
fi

image_path="$1"
timeout_seconds="${2:-900}"
# When given, the booted image must report this version from `loxilb --version`.
# This is the end-to-end assertion that the packaged binary is not stamped with
# a stale common.Version. Leave empty for rolling/latest builds.
expected_version="${3:-}"
expected_version="${expected_version#refs/tags/}"
expected_version="${expected_version#v}"
if [[ "$expected_version" == "latest" ]]; then
  expected_version=""
fi

if [[ ! -f "$image_path" ]]; then
  echo "qcow2 image not found: $image_path" >&2
  exit 1
fi

if ! command -v qemu-system-x86_64 >/dev/null 2>&1; then
  echo "qemu-system-x86_64 is required" >&2
  exit 1
fi

if ! command -v cloud-localds >/dev/null 2>&1; then
  echo "cloud-localds is required" >&2
  exit 1
fi

workdir="$(mktemp -d)"
console_log="$workdir/console.log"
seed_img="$workdir/seed.img"

cleanup() {
  rm -rf "$workdir"
}
trap cleanup EXIT

cat > "$workdir/user-data" <<'EOF'
#cloud-config
output:
  all: '| tee -a /var/log/cloud-init-output.log /dev/ttyS0'
runcmd:
  - |
    expected_version="@EXPECTED_VERSION@"
    for _ in $(seq 1 24); do
      if systemctl is-active --quiet loxilb; then
        cat /etc/loxilb-release 2>/dev/null || true
        # `loxilb --version` emits a "loxilb start" banner before the version
        # line, so select the line rather than taking the first one.
        version_out="$(/usr/local/sbin/loxilb --version 2>/dev/null)"
        reported="$(printf '%s\n' "$version_out" | awk '/^loxilb version:/ {print $3; exit}')"
        echo "LOXILB_VERSION_REPORTED: ${reported}"
        if [ -n "$expected_version" ]; then
          case "$reported" in
            "$expected_version"|"$expected_version"-*) ;;
            *)
              echo "LOXILB_SMOKETEST_FAIL_VERSION expected=${expected_version} reported=${reported}"
              poweroff
              exit 1
              ;;
          esac
        fi
        echo LOXILB_SMOKETEST_PASS
        poweroff
        exit 0
      fi
      sleep 5
    done
    echo LOXILB_SMOKETEST_FAIL
    systemctl status loxilb --no-pager || true
    poweroff
    exit 1
EOF

# The heredoc above is quoted so the guest script survives verbatim; the one
# host-side value it needs is substituted here.
sed -i "s|@EXPECTED_VERSION@|${expected_version}|" "$workdir/user-data"

cat > "$workdir/meta-data" <<'EOF'
instance-id: loxilb-smoketest
local-hostname: loxilb-smoketest
EOF

cloud-localds "$seed_img" "$workdir/user-data" "$workdir/meta-data"

set +e
timeout "$timeout_seconds" qemu-system-x86_64 \
  -machine accel=tcg \
  -cpu max \
  -smp 2 \
  -m 4096 \
  -nographic \
  -monitor none \
  -serial file:"$console_log" \
  -drive file="$image_path",if=virtio,format=qcow2,snapshot=on \
  -drive file="$seed_img",if=virtio,format=raw \
  -netdev user,id=n1 \
  -device virtio-net-pci,netdev=n1
qemu_rc=$?
set -e

cat "$console_log"

if [[ $qemu_rc -eq 124 ]]; then
  echo "Timed out waiting for qcow2 smoke test to complete" >&2
  exit 1
fi

if [[ $qemu_rc -ne 0 ]]; then
  echo "QEMU exited with status $qemu_rc" >&2
  exit 1
fi

if grep -q 'LOXILB_SMOKETEST_FAIL_VERSION' "$console_log"; then
  echo "Packaged loxilb reports the wrong version (expected $expected_version) -- see LOXILB_SMOKETEST_FAIL_VERSION above" >&2
  exit 1
fi

if ! grep -q 'LOXILB_SMOKETEST_PASS' "$console_log"; then
  echo "Did not observe LOXILB_SMOKETEST_PASS in console output" >&2
  exit 1
fi

if [[ -n "$expected_version" ]]; then
  echo "qcow2 smoke test passed (version $expected_version verified)"
else
  echo "qcow2 smoke test passed (no expected version given, version check skipped)"
fi