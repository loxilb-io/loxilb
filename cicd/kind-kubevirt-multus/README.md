# kind-kubevirt-multus

CI scenario: KubeVirt VMs and pods behind loxilb LoadBalancer services, on a kind cluster.

| Case | What is verified |
|------|------------------|
| 1 | One `LoadBalancer` service selects a KubeVirt VM (virt-launcher pod) and a normal pod; loxilb delivers to both, in NodePort (`externalTrafficPolicy: Local`) mode and in `loxilb.io/usepodnetwork` mode. |
| 2 | A VM with a secondary interface (multus, bridge binding); a service annotated with `loxilb.io/multus-nets: secnet` sends client traffic through loxilb to the VM's secondary NIC. |

## Testbed (`config.sh`)

```
runner docker
├─ network "kind"    node eth0  : API server, kindnet pod network, NodePort
└─ network "secnet"  node eth1 → linux bridge br-sec inside every node  (secondary L2, 123.123.123.0/24)
     ├─ loxilb pod (control-plane, multus net1 on br-sec)  VIP 123.123.123.205 via kube-loxilb (in-cluster)
     ├─ VMs / pods with a secondary NIC  (bridge CNI + whereabouts 123.123.123.192/28)
     └─ client container  123.123.123.206  (ghcr.io/loxilb-io/nettest)
```

`./config.sh` brings all of this up (3 kind nodes, multus v4.3.0 thin + whereabouts v0.9.4, in-cluster
loxilb + kube-loxilb, KubeVirt stable, client). `./rmconfig.sh` removes it.

## Validation

`./validation.sh` runs `validation_vm_pod.sh` (case 1) and `validation_vm_secnet.sh` (case 2). Both share
one VM (`kubevirt/vm-web.yml`: masquerade default NIC + `secnet` bridge NIC, labels `web=yes` and `vm=web`)
and one pod (`kubevirt/pod-web.yml`). Each check prints `kind-kubevirt-multus <check>\t[OK|FAILED]`; a
failure dumps VMI status, virt-launcher annotations, `loxicmd get lb/ep/ct` and the kube-loxilb/loxilb logs.

| Check | Service | Passes when |
|-------|---------|-------------|
| case1 nodeport-local | `web-lb-nodeport` :55011, `externalTrafficPolicy: Local` | 20 requests answered by both `vm` and `pod` |
| case1 usepodnetwork | `web-lb-podnet` :55012, `loxilb.io/usepodnetwork` | same |
| case2 | `vm-secnet-lb` :55003, `loxilb.io/multus-nets: secnet` | loxilb endpoint is the VM's secnet IP and 5/5 requests answer `vm` |
| case2-ext (`EXTENDED=1`) | same | traffic recovers after `kubectl delete vmi` (new launcher pod, new IP) and after a live migration to the other worker |

`EXTENDED=only ./validation_vm_secnet.sh` runs just the extended checks (the workflow runs them as a
separate, initially `continue-on-error`, step). `CURL_COUNT` (default 20) sets the sample size.

Environment knobs:

| Variable | Default | Purpose |
|----------|---------|---------|
| `LOXILB_IMAGE`, `KUBE_LOXILB_IMAGE` | `ghcr.io/loxilb-io/{loxilb,kube-loxilb}:latest` | test a development image |
| `KUBEVIRT_VERSION` | KubeVirt `stable.txt` (fallback v1.9.0) | pin KubeVirt |
| `KUBEVIRT_EMULATION` | `auto` (1 when `/dev/kvm` is missing in docker) | force TCG emulation |
| `CNI_PLUGINS_VERSION` | v1.9.1 | reference CNI plugins installed into the nodes (bridge etc.) |
| `CLUSTER`, `SECNET`, `CLIENT_IMAGE`, `MTU` | `kv`, `secnet`, nettest, 1500 | rarely needed |

## Things that are not obvious (learned in the Phase 0 probe)

* The secondary NAD must be the linux `bridge` CNI, not macvlan. KubeVirt bridge binding moves the pod
  NIC's MAC into the guest; macvlan demuxes by MAC and drops unicast for the VM (ARP works, ping does not).
* eth1 must be enslaved to `br-sec` before any pod uses the NAD; a NIC with macvlan children cannot join a bridge.
* The host needs `fs.inotify.max_user_instances >= 1024`, otherwise virt-handler crash-loops with
  "too many open files" inside the kind nodes. `config.sh` sets it (via `colima ssh` on macOS).
* Docker networks are created with MTU 1500 explicitly. Docker Desktop on macOS uses 65535, which KubeVirt
  cannot set on a tap device.
* kind nodes are privileged containers, so `/dev/kvm` from the host is usable as root without a udev rule.
* kube-loxilb applies Endpoints changes with a delay of roughly 30–50 s when they happen after the service
  was created; validation polls `loxicmd get lb -o json` for the expected endpoint count before counting.
  (`loxicmd get lb -o wide` prints extra endpoints as continuation rows with empty EXT IP/PORT cells.)
* Live migration with a bridge-bound secondary NIC and whereabouts IPAM: the target virt-launcher pod gets a
  *new* IP from whereabouts, but the guest keeps its old lease. kube-loxilb follows the pod annotation and
  adds the new (dead) IP; the source pod lingers as `Completed` and still matches the selector, so the old
  (live) IP stays too and the VIP answers only ~50 % of requests. The VMI `status.interfaces` also shows the
  new IP (multus-status wins over the guest agent). This is the first Phase 5 gap; the migration check in
  `validation_vm_secnet.sh` requires a sustained all-good window so it fails for this reason.

## Running locally on macOS (Apple Silicon)

Docker Desktop has no nested virtualization and arm64 KubeVirt requires KVM. Use colima:

```
colima start --profile kvm --vm-type vz --nested-virtualization --cpu 6 --memory 10 --disk 50   # M3+ / macOS 15+
docker context use colima-kvm
cd cicd/kind-kubevirt-multus && ./config.sh && ./validation.sh; ./rmconfig.sh
```

`phase0/` keeps the probe scripts and logs from which this scenario was derived.
