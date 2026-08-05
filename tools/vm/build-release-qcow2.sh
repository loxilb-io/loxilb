#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 3 ]]; then
  echo "usage: $0 <release-tag> <deb-path> <output-dir> [arch]" >&2
  exit 1
fi

for required_cmd in curl dpkg-deb qemu-img sha256sum virt-customize; do
  if ! command -v "$required_cmd" >/dev/null 2>&1; then
    echo "missing required command: $required_cmd" >&2
    exit 1
  fi
done

release_tag="$1"
deb_path="$2"
output_dir="$3"
arch="${4:-amd64}"

if [[ ! -f "$deb_path" ]]; then
  echo "deb package not found: $deb_path" >&2
  exit 1
fi

if [[ "$arch" != "amd64" ]]; then
  echo "unsupported architecture: $arch" >&2
  exit 1
fi

normalized_tag="${release_tag#refs/tags/}"
normalized_tag="${normalized_tag#v}"
if [[ -z "$normalized_tag" ]]; then
  echo "release tag must not be empty" >&2
  exit 1
fi

asset_tag="v${normalized_tag}"
ubuntu_series="22.04"
base_image_url="https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img"
image_name="loxilb-${asset_tag}-ubuntu-${ubuntu_series}-${arch}.qcow2"
image_path="${output_dir%/}/${image_name}"
sha_path="${image_path}.sha256"

mkdir -p "$output_dir"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

base_image_path="$workdir/jammy-server-cloudimg-amd64.img"
deb_name="$(basename "$deb_path")"
payload_dir="$workdir/rootfs"

curl -fsSL "$base_image_url" -o "$base_image_path"
cp "$base_image_path" "$image_path"
qemu-img resize "$image_path" 16G
dpkg-deb -x "$deb_path" "$payload_dir"

virt_customize_args=(
  -a "$image_path"
  --hostname loxilb
  --run-command "mkdir -p /etc/systemd/system/multi-user.target.wants /opt/loxilb/dp"
  --run-command 'ln -sf /etc/systemd/system/loxilb.service /etc/systemd/system/multi-user.target.wants/loxilb.service'
  --run-command 'chmod +x /usr/local/sbin/loxilb /usr/local/sbin/loxicmd /usr/local/sbin/mkllb_bpffs || true'
  --run-command "printf 'release=%s\nbase_image=ubuntu-%s\npackage=%s\ninstall_mode=offline-payload-copy\n' '$asset_tag' '$ubuntu_series' '$deb_name' > /etc/loxilb-release"
)

for payload_subdir in etc opt usr; do
  if [[ -d "$payload_dir/$payload_subdir" ]]; then
    virt_customize_args+=(--copy-in "$payload_dir/$payload_subdir:/")
  fi
done

sudo env LIBGUESTFS_BACKEND=direct virt-customize "${virt_customize_args[@]}"

(cd "$output_dir" && sha256sum "$image_name" > "${image_name}.sha256")
qemu-img info "$image_path"

echo "Created $image_path"
echo "Created $sha_path"