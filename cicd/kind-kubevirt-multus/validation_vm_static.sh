#!/bin/bash
# Case 2b: a VM whose secondary interface has a STATIC address on an IPAM-less network (kubevirt/vm-static.yml).
# The pod's network-status carries no address, so the service works only when kube-loxilb resolves VM
# endpoints from the VMI status. The address survives live migration, so traffic must stay up afterwards.
#   vm-static-lb :55004 → loxilb net1 (onearm) → 123.123.123.204:80 (answers "vm-static")
# EXTENDED=0 skips the live-migration part.
cd "$(dirname "$0")"; source ./lib.sh
STATIC_IP=${STATIC_IP:-123.123.123.204}
EXTENDED=${EXTENDED:-1}
echo "kind-kubevirt-multus case 2b: static secondary address via loxilb.io/multus-nets (VMI status)"

static_pod(){ kubectl get pod -l vm=static --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}' 2>/dev/null; }
dump_static_net(){
  local p; p=$(static_pod)
  echo "    virt-launcher: $p on $(kubectl get vmi vm-static -o jsonpath='{.status.nodeName}')"
  echo "    network-status: $(net_status "$p")"
  echo "    vmi secnet: ip=$(kubectl get vmi vm-static -o jsonpath='{.status.interfaces[?(@.name=="secnet")].ipAddress}') src=$(kubectl get vmi vm-static -o jsonpath='{.status.interfaces[?(@.name=="secnet")].infoSource}')"
}

log "workload: vm-static (static $STATIC_IP on secnet-static)"
kubectl apply -f kubevirt/vm-static.yml >/dev/null
wait_for "vmi vm-static exists" 120 kubectl get vmi vm-static
kubectl wait vmi vm-static --for=condition=Ready --timeout=600s >/dev/null && echo "    vmi vm-static Ready on $(kubectl get vmi vm-static -o jsonpath='{.status.nodeName}')"
wait_for "guest http up (client → $STATIC_IP:80)" 300 bash -c "docker exec $CLIENT curl -s -m 3 http://$STATIC_IP/ | grep -q '^vm-static'"; check "case2b direct L2 reachability ($STATIC_IP)" $?
dump_static_net

log "service vm-static-lb :55004"
kubectl apply -f kubevirt/vm-static-lb.yml >/dev/null
wait_external_ip vm-static-lb 120 || true
wait_for "loxilb endpoint == $STATIC_IP for :55004" 240 bash -c "lb_ep_list 55004 | grep -qw $STATIC_IP"; check "case2b endpoint is the static ip (from VMI status)" $?
echo "    loxilb endpoints for :55004: [$(lb_ep_list 55004)]"
ok=0; for i in 1 2 3 4 5; do [ "$(client_curl 55004)" = vm-static ] && ok=$((ok+1)); done
[ $ok -eq 5 ]; check "case2b client → VIP:55004 → static VM ($ok/5)" $?

if [ "$EXTENDED" != 0 ]; then
  log "case 2b live migration: the static address must survive and stay the only endpoint"
  src=$(kubectl get vmi vm-static -o jsonpath='{.status.nodeName}'); t0=$(date +%s)
  kubectl apply -f - >/dev/null <<EOM
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstanceMigration
metadata: { name: vm-static-migration }
spec: { vmiName: vm-static }
EOM
  wait_for "migration finished" 600 bash -c "kubectl get vmim vm-static-migration -o jsonpath='{.status.phase}' | grep -Eq 'Succeeded|Failed'"
  ph=$(kubectl get vmim vm-static-migration -o jsonpath='{.status.phase}'); dst=$(kubectl get vmi vm-static -o jsonpath='{.status.nodeName}')
  echo "    migration $src → $dst: $ph"; [ "$ph" = Succeeded ]; check "case2b live migration succeeded" $?
  wait_for "single Running launcher pod" 180 bash -c "[ \$(kubectl get pods -l vm=static --field-selector=status.phase=Running --no-headers | wc -l) -eq 1 ]"
  dump_static_net
  wait_for "kube-loxilb resync after migration" 120 bash -c "[ \$(lb_ep_count 55004) -eq 1 ] && lb_ep_list 55004 | grep -qw $STATIC_IP" || true
  echo "    loxilb endpoints for :55004: [$(lb_ep_list 55004)]"
  [ "$(lb_ep_count 55004)" = 1 ] && lb_ep_list 55004 | grep -qw "$STATIC_IP"; rc=$?
  check "case2b exactly one endpoint ($STATIC_IP) after migration" $rc
  sustained_ok 55004 6 10 5 vm-static; rc=$?
  check "case2b traffic stays up after migration (6 rounds x 10 requests, $(( $(date +%s)-t0 ))s total)" $rc
  kubectl delete vmim vm-static-migration --ignore-not-found >/dev/null
fi

[ $FAILS -gt 0 ] && dump_debug
exit $FAILS
