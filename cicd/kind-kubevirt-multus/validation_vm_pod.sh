#!/bin/bash
# Case 1: one LoadBalancer service fronting a KubeVirt VM and a plain pod; loxilb must deliver to both.
#   web-lb-nodeport :55011  externalTrafficPolicy Local  (loxilb → node:NodePort → kube-proxy → VM | pod)
#   web-lb-podnet   :55012  loxilb.io/usepodnetwork      (loxilb → pod IP:80 directly; VM via masquerade NAT)
cd "$(dirname "$0")"; source ./lib.sh
echo "kind-kubevirt-multus case 1: VM + pod behind one service"
ensure_workloads

log "services"
kubectl apply -f kubevirt/web-lb-nodeport.yml -f kubevirt/web-lb-podnet.yml >/dev/null
for s in web-lb-nodeport:55011 web-lb-podnet:55012; do
  wait_external_ip "${s%:*}" 120 || true
  # kube-loxilb picks up Endpoints changes with a delay (~50 s observed); wait for both endpoints first.
  wait_lb_eps "${s#*:}" 2 240 || echo "    WARN: only $(lb_ep_count "${s#*:}") endpoint(s) for :${s#*:}: $(lb_ep_list "${s#*:}")"
done
kubectl get svc web-lb-nodeport web-lb-podnet
loxicmd get lb -o wide | grep -E "EXT IP|5501[12]"

log "VM reachable through the VIP (pod answers immediately; the VM path must be up too)"
for s in web-lb-nodeport:55011 web-lb-podnet:55012; do
  wait_for "a 'vm' answer via VIP:${s#*:}" 300 bash -c "for i in 1 2 3 4; do docker exec $CLIENT curl -s -m 3 http://$VIP:${s#*:}/ | grep -q '^vm' && exit 0; done; exit 1" || true
done

log "distribution ($CURL_COUNT requests per service)"
count_responses 55011; [ $VM_N -gt 0 ] && [ $POD_N -gt 0 ]; check "case1 nodeport-local :55011 (vm=$VM_N pod=$POD_N)" $?
count_responses 55012; [ $VM_N -gt 0 ] && [ $POD_N -gt 0 ]; check "case1 usepodnetwork :55012 (vm=$VM_N pod=$POD_N)" $?
loxicmd get ct | grep -E "5501[12]" | head -4

[ $FAILS -gt 0 ] && dump_debug
exit $FAILS
