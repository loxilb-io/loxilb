#!/bin/bash
# Build the eBPF datapath from local source with the loxilb toolchain (clang 10 in
# ossca-build), bake into a derived image, and rebuild the testbed.
# Usage: sudo ./build_deploy.sh
set -e
# repo root = three levels up from this script (cicd/tlsproxyprotov2/)
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
KDIR=$ROOT/loxilb-ebpf/kernel

echo "== [1/4] build .o in ossca-build (clang 10) =="
# NOTE: the Makefile does NOT list llb_kern_cdefs.h as a prerequisite, so editing
# that header alone does NOT trigger a rebuild. Force it by removing the objects.
rm -f $KDIR/llb_ebpf_main.o $KDIR/llb_ebpf_emain.o
docker run --rm -v $ROOT/loxilb-ebpf:/be --entrypoint /bin/bash ghcr.io/loxilb-io/ossca-build:latest \
  -c "cd /be/kernel && rm -f llb_ebpf_main.o llb_ebpf_emain.o && make llb_ebpf_main.o llb_ebpf_emain.o 2>&1 | grep -iE 'error|clang' | head"
test -f $KDIR/llb_ebpf_main.o
echo "   PPV2DBG symbols in built .o: $(strings $KDIR/llb_ebpf_main.o 2>/dev/null | grep -c PPV2DBG)"

echo "== [2/4] build derived image loxilb:ppv2test =="
rm -rf /tmp/ppv2img; mkdir -p /tmp/ppv2img
cp $KDIR/llb_ebpf_main.o $KDIR/llb_ebpf_emain.o /tmp/ppv2img/
printf 'FROM ghcr.io/loxilb-io/loxilb:latest\nCOPY llb_ebpf_main.o llb_ebpf_emain.o /opt/loxilb/\n' > /tmp/ppv2img/Dockerfile
docker build -t loxilb:ppv2test /tmp/ppv2img >/dev/null 2>&1
echo "   image built"

echo "== [3/4] rebuild testbed =="
cd $ROOT/cicd/tlsproxyprotov2
./rmconfig.sh >/dev/null 2>&1
LOXILB_IMAGE=loxilb:ppv2test ./config.sh >config.run.log 2>&1
grep -q "Setup done" config.run.log && echo "   testbed up" || { echo "   CONFIG FAILED"; tail -5 config.run.log; exit 1; }

echo "== [4/4] done. run ./validation.sh or ./trace_repro.sh =="
