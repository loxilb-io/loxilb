#!/bin/bash
# Runs both cases. EXTENDED=1 adds the extended (initially xfail) checks of case 2.
cd "$(dirname "$0")"
rc=0
./validation_vm_pod.sh || rc=1
echo
./validation_vm_secnet.sh || rc=1
echo
if [ $rc = 0 ]; then echo -e "kind-kubevirt-multus\t[OK]"; else echo -e "kind-kubevirt-multus\t[FAILED]"; fi
exit $rc
