#!/bin/bash
# Shared helpers for the kind-kubevirt-multus validation scripts. Source after cd to the test dir.
VIP=${VIP:-123.123.123.205}
CLIENT=${CLIENT:-client}
CURL_COUNT=${CURL_COUNT:-20}
FAILS=0

log(){ echo; echo "--- [$(date +%T)] $*"; }
check(){ # check <name> <0|1 ok>
  if [ "$2" = 0 ]; then echo -e "kind-kubevirt-multus $1\t[OK]"; else echo -e "kind-kubevirt-multus $1\t[FAILED]"; FAILS=$((FAILS+1)); fi
}
client_curl(){ docker exec "$CLIENT" curl -s -m 3 "http://${2:-$VIP}:$1/" 2>/dev/null; }
loxilb_pod(){ kubectl get pod -l app=loxilb-app -o jsonpath='{.items[0].metadata.name}' 2>/dev/null; }
loxicmd(){ kubectl exec "$(loxilb_pod)" -- loxicmd "$@" 2>/dev/null; }
# Endpoint IPs of the loxilb rule for VIP port $1 (space separated). Parsed from JSON: the "-o wide" table
# prints additional endpoints as continuation rows with empty EXT IP/PORT cells, so grep would undercount.
lb_ep_list(){ loxicmd get lb -o json | python3 -c '
import json,sys
port=int(sys.argv[1]); d=json.load(sys.stdin)
for it in d.get("lbAttr",[]):
    if it.get("serviceArguments",{}).get("port")==port:
        print(" ".join(e.get("endpointIP","") for e in it.get("endpoints",[])))' "$1" 2>/dev/null; }
lb_ep_count(){ lb_ep_list "$1" | wc -w | tr -d " "; }
vm_pod(){ kubectl get pod -l vm=web --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}' 2>/dev/null; }
net_status(){ kubectl get pod "$1" -o jsonpath='{.metadata.annotations.k8s\.v1\.cni\.cncf\.io/network-status}' 2>/dev/null; }
secnet_ip_of_pod(){ net_status "$1" | python3 -c 'import json,sys; print([n for n in json.load(sys.stdin) if n["name"]=="default/secnet"][0]["ips"][0])' 2>/dev/null; }
vmi_secnet_ip(){ kubectl get vmi vm-web -o jsonpath='{.status.interfaces[?(@.name=="secnet")].ipAddress}' 2>/dev/null; }

# wait_for <description> <timeout-s> <command...>   (command is retried until it succeeds)
wait_for(){
  local desc=$1 timeout=$2 t0=$(date +%s); shift 2
  while ! "$@" >/dev/null 2>&1; do
    if [ $(( $(date +%s)-t0 )) -ge "$timeout" ]; then echo "    timeout: $desc (${timeout}s)"; return 1; fi
    sleep 3
  done
  echo "    $desc: $(( $(date +%s)-t0 ))s"
}
wait_external_ip(){ wait_for "$1 external IP" "${2:-120}" bash -c "kubectl get svc $1 -o jsonpath='{.status.loadBalancer.ingress[0].hostname}{.status.loadBalancer.ingress[0].ip}' | grep -q ."; }
wait_lb_eps(){ wait_for "loxilb endpoints for :$1 >= $2" "${3:-240}" bash -c "[ \$(lb_ep_count $1) -ge $2 ]"; }
export -f lb_ep_count lb_ep_list loxicmd loxilb_pod client_curl secnet_ip_of_pod net_status vm_pod 2>/dev/null || true

# count_responses <port> → sets VM_N POD_N OTHER_N
count_responses(){
  VM_N=0; POD_N=0; OTHER_N=0
  for i in $(seq 1 "$CURL_COUNT"); do
    case "$(client_curl "$1")" in vm) VM_N=$((VM_N+1));; pod) POD_N=$((POD_N+1));; *) OTHER_N=$((OTHER_N+1));; esac
  done
  echo "    :$1 responses  vm=$VM_N pod=$POD_N other=$OTHER_N (of $CURL_COUNT)"
}

# sustained_ok <port> <rounds> <requests-per-round> <sleep-s>: every request in every round must answer "vm"
sustained_ok(){
  local port=$1 rounds=$2 n=$3 gap=$4 r i ok bad=0
  for r in $(seq 1 "$rounds"); do
    ok=0; for i in $(seq 1 "$n"); do [ "$(client_curl "$port")" = vm ] && ok=$((ok+1)); done
    echo "    round $r: $ok/$n ok  (loxilb eps: $(lb_ep_list "$port"))"; [ $ok -eq "$n" ] || bad=$((bad+1))
    [ "$r" -lt "$rounds" ] && sleep "$gap"
  done
  [ $bad -eq 0 ]
}

ensure_workloads(){
  log "workloads: pod-web (kv-worker2) + vm-web (KubeVirt, 2 NICs)"
  kubectl apply -f kubevirt/pod-web.yml -f kubevirt/vm-web.yml >/dev/null
  kubectl wait pod/pod-web --for=condition=Ready --timeout=300s >/dev/null && echo "    pod-web Ready on $(kubectl get pod pod-web -o jsonpath='{.spec.nodeName}')"
  wait_for "vmi vm-web exists" 120 kubectl get vmi vm-web
  kubectl wait vmi vm-web --for=condition=Ready --timeout=600s >/dev/null && echo "    vmi vm-web Ready on $(kubectl get vmi vm-web -o jsonpath='{.status.nodeName}') (launcher $(vm_pod))"
  # VMI Ready only means the domain runs; cloud-init starts the http server ~60 s later. Wait for it via the
  # secnet ip (reachable from the client) so that neither case counts requests against a booting guest.
  local vmip; vmip=$(secnet_ip_of_pod "$(vm_pod)")
  wait_for "guest http up (client → $vmip:80)" 300 bash -c "docker exec $CLIENT curl -s -m 3 http://$vmip/ | grep -q '^vm'"
}

dump_debug(){
  echo; echo "===== DEBUG DUMP ====="
  kubectl get nodes -o wide; echo
  kubectl get pods -o wide; echo
  kubectl get vmi -o wide 2>/dev/null; kubectl describe vmi vm-web 2>/dev/null | sed -n '/^Status:/,$p' | tail -40; echo
  kubectl get svc; kubectl get endpoints 2>/dev/null; echo
  P=$(vm_pod); [ -n "$P" ] && { echo "virt-launcher $P annotations:"; kubectl get pod "$P" -o jsonpath='{.metadata.annotations.k8s\.v1\.cni\.cncf\.io/networks}'; echo; net_status "$P"; echo; }
  echo "loxicmd get lb:"; loxicmd get lb -o wide; echo "loxicmd get ep:"; loxicmd get ep; echo "loxicmd get ct:"; loxicmd get ct
  echo "kube-loxilb log tail:"; kubectl -n kube-system logs deploy/kube-loxilb --tail=40 2>/dev/null
  echo "loxilb log tail:"; kubectl logs "$(loxilb_pod)" --tail=30 2>/dev/null
  echo "client neigh:"; docker exec "$CLIENT" ip neigh 2>/dev/null
  echo "===== END DUMP ====="
}
