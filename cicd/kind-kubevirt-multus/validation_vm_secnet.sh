#!/bin/bash
# Case 2: a service with loxilb.io/multus-nets sends client traffic through loxilb to the VM's secondary NIC.
#   vm-secnet-lb :55003 → loxilb net1 (onearm) → VM secnet IP :80
# EXTENDED=1    also run the extended checks (VMI recreation, live migration) after the base check.
# EXTENDED=only run only the extended checks (base resources must already exist).
cd "$(dirname "$0")"; source ./lib.sh
EXTENDED=${EXTENDED:-0}
echo "kind-kubevirt-multus case 2: VM secondary interface via loxilb.io/multus-nets"
ensure_workloads

dump_vm_net(){
  local p; p=$(vm_pod)
  echo "    virt-launcher: $p on $(kubectl get vmi vm-web -o jsonpath='{.status.nodeName}')"
  echo "    networks annotation: $(kubectl get pod "$p" -o jsonpath='{.metadata.annotations.k8s\.v1\.cni\.cncf\.io/networks}')"
  echo "    network-status secnet ip: $(secnet_ip_of_pod "$p")   vmi status secnet ip: $(vmi_secnet_ip)"
}

base_check(){
  log "VM network facts (evidence for gap analysis)"
  dump_vm_net
  VMIP=$(secnet_ip_of_pod "$(vm_pod)")
  [ -n "$VMIP" ]; check "case2 secnet ip in network-status ($VMIP)" $?
  wait_for "direct client → VM $VMIP:80" 300 bash -c "docker exec $CLIENT curl -s -m 3 http://$VMIP/ | grep -q '^vm'"; check "case2 direct L2 reachability" $?

  log "service vm-secnet-lb :55003"
  kubectl apply -f kubevirt/vm-secnet-lb.yml >/dev/null
  wait_external_ip vm-secnet-lb 120 || true
  wait_for "loxilb endpoint == $VMIP for :55003" 240 bash -c "lb_ep_list 55003 | grep -qw $VMIP"; check "case2 endpoint is the VM secnet ip" $?
  loxicmd get lb -o wide | grep -E "EXT IP|55003"
  ok=0; for i in 1 2 3 4 5; do [ "$(client_curl 55003)" = vm ] && ok=$((ok+1)); done
  [ $ok -eq 5 ]; check "case2 client → VIP:55003 → VM secnet ($ok/5)" $?
}

extended_checks(){
  log "extended 1/2: recreate the VMI (new virt-launcher pod, possibly new secnet ip)"
  old=$(secnet_ip_of_pod "$(vm_pod)"); t0=$(date +%s)
  kubectl delete vmi vm-web --wait=true >/dev/null
  wait_for "vmi vm-web exists again" 120 kubectl get vmi vm-web
  kubectl wait vmi vm-web --for=condition=Ready --timeout=600s >/dev/null; dump_vm_net
  new=$(secnet_ip_of_pod "$(vm_pod)"); echo "    secnet ip $old → $new"
  wait_for "loxilb endpoint == $new after recreate" 300 bash -c "lb_ep_list 55003 | grep -qw $new"; check "case2-ext endpoint refreshed after VMI recreate" $?
  wait_for "VIP:55003 serves vm again" 300 bash -c "[ \"\$(docker exec $CLIENT curl -s -m 3 http://$VIP:55003/)\" = vm ]"; rc=$?
  check "case2-ext traffic recovers after VMI recreate ($(( $(date +%s)-t0 ))s total)" $rc

  log "extended 2/2: live migration to the other worker"
  src=$(kubectl get vmi vm-web -o jsonpath='{.status.nodeName}'); old_secnet=$(secnet_ip_of_pod "$(vm_pod)"); t0=$(date +%s)
  kubectl apply -f - >/dev/null <<EOM
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstanceMigration
metadata: { name: vm-web-migration }
spec: { vmiName: vm-web }
EOM
  wait_for "migration finished" 600 bash -c "kubectl get vmim vm-web-migration -o jsonpath='{.status.phase}' | grep -Eq 'Succeeded|Failed'"
  ph=$(kubectl get vmim vm-web-migration -o jsonpath='{.status.phase}'); dst=$(kubectl get vmi vm-web -o jsonpath='{.status.nodeName}')
  echo "    migration $src → $dst: $ph"; [ "$ph" = Succeeded ]; check "case2-ext live migration succeeded" $?
  # The source launcher pod lingers as "Completed" (still matching the selector); wait for a single Running pod
  # so that vm_pod() returns the migration target.
  wait_for "single Running launcher pod" 180 bash -c "[ \$(kubectl get pods -l vm=web --field-selector=status.phase=Running --no-headers | wc -l) -eq 1 ]"
  echo "    launcher pods now: $(kubectl get pods -l vm=web --no-headers | awk '{print $1"="$3}' | tr '\n' ' ')"
  dump_vm_net
  # Evidence: which secnet IP does the guest really answer on, and which does loxilb target?
  newip=$(secnet_ip_of_pod "$(vm_pod)")
  for ip in $old_secnet $newip; do printf "    direct client → %s: %s\n" "$ip" "$(docker exec $CLIENT curl -s -m 2 http://$ip/ || echo no-answer)"; done
  # Let kube-loxilb reconcile the target pod (its Endpoints/pod refresh takes up to ~60 s), then measure.
  wait_for "kube-loxilb reconciled target pod ip $newip into :55003" 180 bash -c "lb_ep_list 55003 | grep -qw $newip" || true
  echo "    loxilb endpoints for :55003: [$(lb_ep_list 55003)]"
  # A single successful request is not enough (round-robin can hit a live endpoint by luck):
  # require a sustained window of all-good responses.
  sustained_ok 55003 6 10 5; rc=$?     # capture first: a $(...) inside the check message would reset $?
  check "case2-ext traffic stays up after migration (6 rounds x 10 requests, $(( $(date +%s)-t0 ))s total)" $rc
  kubectl delete vmim vm-web-migration --ignore-not-found >/dev/null
}

[ "$EXTENDED" != only ] && base_check
[ "$EXTENDED" = 1 ] || [ "$EXTENDED" = only ] && extended_checks
[ $FAILS -gt 0 ] && dump_debug
exit $FAILS
