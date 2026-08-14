# loxilb release qcow2 image

This directory contains the release image builder used by the GitHub Actions release workflow.

What the builder does:

- Downloads the Ubuntu 22.04 cloud image as the base disk.
- Expands the disk to 16G.
- Extracts the loxilb Debian package on the host and copies its payload into the image.
- Installs a small set of guest packages needed to run loxilb in a VM.
- Enables the loxilb systemd unit for boot.
- Writes a small release metadata file at /etc/loxilb-release.
- Emits a normalized artifact name and a matching sha256 file.

Why the builder uses payload copy instead of guest-side dpkg install:

- In headless CI and some local environments, the Ubuntu guest image may not have working DNS during virt-customize package installation.
- The current loxilb Debian package preinst script requires go to exist inside the guest, which blocks a normal dpkg install during image creation.
- Copying the package payload directly keeps the qcow2 build self-contained and allows the image to boot successfully without guest-side package resolution.

Expected artifact names:

- loxilb-vX.Y.Z-ubuntu-22.04-amd64.qcow2
- loxilb-vX.Y.Z-ubuntu-22.04-amd64.qcow2.sha256

For the rolling release, the artifact name becomes:

- loxilb-vlatest-ubuntu-22.04-amd64.qcow2

The image is built from the Ubuntu cloud image, so the recommended boot flow is cloud-init with an SSH public key.
Example seed files are provided under tools/vm/cloud-init.

Validation without release:

- Run the package workflow with buildVmImage=true, smokeTestVmImage=true, publishRelease=false.
- The workflow will build the Debian package, create the qcow2 image, boot it in QEMU, and verify that the loxilb systemd service reaches active state.
- The generated deb and qcow2 files are uploaded to the workflow run as GitHub Actions artifacts, not to the release page.