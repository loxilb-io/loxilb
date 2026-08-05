#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <qcow2-path> [timeout-seconds]" >&2
  exit 1
fi

image_path="$1"
timeout_seconds="${2:-900}"

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
    for _ in $(seq 1 24); do
      if systemctl is-active --quiet loxilb; then
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

if ! grep -q 'LOXILB_SMOKETEST_PASS' "$console_log"; then
  echo "Did not observe LOXILB_SMOKETEST_PASS in console output" >&2
  exit 1
fi

echo "qcow2 smoke test passed"