#!/bin/bash
# kind-kubevirt-multus: bring up the testbed for the KubeVirt + kube-loxilb + loxilb scenarios.
#
#   runner docker
#   ├─ network "kind"    : node eth0  (API server, kindnet pod network, NodePort)
#   └─ network "secnet"  : node eth1 → linux bridge br-sec inside every node (secondary L2, 123.123.123.0/24)
#        ├─ loxilb pod (control-plane, multus net1 on br-sec, VIP 123.123.123.205 via kube-loxilb)
#        ├─ KubeVirt VMs / pods with a secondary NIC (bridge CNI + whereabouts 123.123.123.192/28)
#        └─ client container 123.123.123.206
#
# Env knobs: CLUSTER, LOXILB_IMAGE, KUBE_LOXILB_IMAGE, CLIENT_IMAGE, CNI_PLUGINS_VERSION,
#            KUBEVIRT_VERSION, KUBEVIRT_EMULATION (auto|0|1), MTU
set -euo pipefail
cd "$(dirname "$0")"

CLUSTER=${CLUSTER:-kv}
SECNET=${SECNET:-secnet}
SECNET_SUBNET=${SECNET_SUBNET:-123.123.123.0/24}
SECNET_DOCKER_RANGE=${SECNET_DOCKER_RANGE:-123.123.123.0/25}   # docker IPAM (node eth1, client); whereabouts uses .192/28
CLIENT_IP=${CLIENT_IP:-123.123.123.206}
CLIENT_IMAGE=${CLIENT_IMAGE:-ghcr.io/loxilb-io/nettest:latest}
LOXILB_IMAGE=${LOXILB_IMAGE:-ghcr.io/loxilb-io/loxilb:latest}
KUBE_LOXILB_IMAGE=${KUBE_LOXILB_IMAGE:-ghcr.io/loxilb-io/kube-loxilb:latest}
CNI_PLUGINS_VERSION=${CNI_PLUGINS_VERSION:-v1.9.1}
KUBEVIRT_VERSION=${KUBEVIRT_VERSION:-$(curl -sf --max-time 10 https://storage.googleapis.com/kubevirt-prow/release/kubevirt/kubevirt/stable.txt || echo v1.9.0)}
KUBEVIRT_EMULATION=${KUBEVIRT_EMULATION:-auto}
MTU=${MTU:-1500}

log(){ echo; echo "### [$(date +%T)] $*"; }
T_START=$(date +%s)

host_sysctl(){
  # virt-handler creates many inotify watchers inside the kind nodes; the Linux default of 128
  # user instances makes it crash-loop with "too many open files".
  if [ "$(uname -s)" = Darwin ]; then
    ctx=$(docker context show); profile=${ctx#colima-}; [ "$ctx" = colima ] && profile=default
    if [[ "$ctx" == colima* ]]; then colima ssh -p "$profile" -- sudo sysctl -w "$@"; else echo "WARN: cannot set $* on $ctx; set it in the docker VM manually"; fi
  elif [ "$(id -u)" = 0 ]; then sysctl -w "$@"; else sudo sysctl -w "$@"; fi
}

log "0. clean up any previous run"
./rmconfig.sh

log "1. host preparation"
host_sysctl fs.inotify.max_user_instances=1024 fs.inotify.max_user_watches=1048576
if [ "$(uname -s)" = Darwin ] && ! docker network inspect kind >/dev/null 2>&1; then
  # Docker Desktop defaults to MTU 65535, which KubeVirt cannot put on a tap device. kind reuses an existing "kind" network.
  docker network create -d bridge -o com.docker.network.driver.mtu=$MTU -o com.docker.network.bridge.enable_ip_masquerade=true kind >/dev/null
fi
if docker run --rm --privileged alpine:3.20 test -c /dev/kvm 2>/dev/null; then
  echo "/dev/kvm: available in docker"; [ "$KUBEVIRT_EMULATION" = auto ] && KUBEVIRT_EMULATION=0
else
  echo "/dev/kvm: NOT available in docker"; [ "$KUBEVIRT_EMULATION" = auto ] && KUBEVIRT_EMULATION=1
fi
ARCH=$(docker info -f '{{.Architecture}}' | sed 's/aarch64/arm64/;s/x86_64/amd64/')
CNI_URL="https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/cni-plugins-linux-${ARCH}-${CNI_PLUGINS_VERSION}.tgz"
echo "arch=$ARCH kubevirt=$KUBEVIRT_VERSION emulation=$KUBEVIRT_EMULATION cni-plugins=$CNI_PLUGINS_VERSION"

log "2. kind cluster $CLUSTER (control-plane + 2 workers)"
kind create cluster --name "$CLUSTER" --config kind-config.yaml --wait 180s
kubectl config use-context "kind-$CLUSTER" >/dev/null

log "3. secondary L2: docker network $SECNET, eth1 + br-sec + CNI plugins on every node"
docker network create -o com.docker.network.driver.mtu=$MTU --subnet "$SECNET_SUBNET" --ip-range "$SECNET_DOCKER_RANGE" "$SECNET" >/dev/null
for n in $(kind get nodes --name "$CLUSTER"); do
  docker network connect "$SECNET" "$n"
  docker exec "$n" sh -c "curl -sL $CNI_URL | tar -C /opt/cni/bin -xz"
  # eth1 must be enslaved before any secnet pod exists (a NIC with children cannot join a bridge).
  docker exec "$n" sh -c 'ip link add br-sec type bridge && ip link set br-sec up && ip link set eth1 master br-sec'
  echo "  $n: $(docker exec "$n" ip -br -4 addr show eth1 | awk '{print $3}') → br-sec, plugins: $(docker exec "$n" sh -c 'ls /opt/cni/bin | wc -l')"
done

log "4. multus + whereabouts + NAD secnet"
kubectl apply -f multus/multus-daemonset.yml >/dev/null
kubectl apply -f multus/whereabouts/ >/dev/null
kubectl -n kube-system rollout status ds/kube-multus-ds --timeout=300s
kubectl -n kube-system rollout status ds/whereabouts --timeout=300s
kubectl apply -f multus/nad-secnet.yml

log "5. in-cluster loxilb ($LOXILB_IMAGE) + kube-loxilb ($KUBE_LOXILB_IMAGE)"
sed "s#ghcr.io/loxilb-io/loxilb:latest#${LOXILB_IMAGE}#" yaml/loxilb.yaml | kubectl apply -f - >/dev/null
sed "s#ghcr.io/loxilb-io/kube-loxilb:latest#${KUBE_LOXILB_IMAGE}#" yaml/kube-loxilb.yaml | kubectl apply -f - >/dev/null
kubectl rollout status ds/loxilb-lb --timeout=300s
kubectl -n kube-system rollout status deploy/kube-loxilb --timeout=300s
LP=$(kubectl get pod -l app=loxilb-app -o jsonpath='{.items[0].metadata.name}')
for i in $(seq 1 30); do
  LOXI_SEC_IP=$(kubectl get pod "$LP" -o jsonpath='{.metadata.annotations.k8s\.v1\.cni\.cncf\.io/network-status}' 2>/dev/null \
    | python3 -c 'import json,sys; print([n for n in json.load(sys.stdin) if n["name"]=="default/secnet"][0]["ips"][0])' 2>/dev/null) && [ -n "$LOXI_SEC_IP" ] && break
  sleep 2
done
echo "  loxilb pod $LP on $(kubectl get pod "$LP" -o jsonpath='{.spec.nodeName}'), secnet ip: ${LOXI_SEC_IP:-<none>}"

log "6. KubeVirt $KUBEVIRT_VERSION (emulation=$KUBEVIRT_EMULATION)"
kubectl apply -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml" >/dev/null
kubectl apply -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml" >/dev/null
if [ "$KUBEVIRT_EMULATION" = 1 ]; then
  kubectl -n kubevirt patch kubevirt kubevirt --type=merge -p '{"spec":{"configuration":{"developerConfiguration":{"useEmulation":true}}}}' >/dev/null
fi
kubectl -n kubevirt wait kv kubevirt --for condition=Available --timeout=900s
kubectl get nodes -o jsonpath='{range .items[*]}{"  "}{.metadata.name}{" kvm="}{.status.allocatable.devices\.kubevirt\.io/kvm}{" tun="}{.status.allocatable.devices\.kubevirt\.io/tun}{"\n"}{end}'

log "7. client container $CLIENT_IP on $SECNET"
# nettest has ENTRYPOINT /bin/bash; keep it alive with -it like cicd/common.sh does (no explicit command).
docker run -u root --cap-add SYS_ADMIN -dit --net "$SECNET" --ip "$CLIENT_IP" --name client "$CLIENT_IMAGE" >/dev/null
sleep 1; docker ps --filter name=client --format '  client: {{.Status}} ({{.Image}})' | grep -q Up || { echo "client container is not running"; docker logs client; exit 1; }
docker exec client ip -br -4 addr show eth0

log "testbed ready in $(( $(date +%s)-T_START ))s"
kubectl get nodes -o wide | awk '{print "  "$1, $2, $6}'
kubectl get pods -A --no-headers | awk '{c[$4]++} END {for (s in c) printf "  pods %s: %d\n", s, c[s]}'
