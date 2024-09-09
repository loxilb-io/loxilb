![image](https://github.com/loxilb-io/loxilb/assets/75648333/87da0183-1a65-493f-b6fe-5bc738ba5468)


[![Website](https://img.shields.io/static/v1?label=www&message=loxilb.io&color=blue?style=for-the-badge&logo=appveyor)](https://www.loxilb.io) [![eBPF Emerging Project](https://img.shields.io/badge/ebpf.io-Emerging--App-success)](https://ebpf.io/projects#loxilb) [![Go Report Card](https://goreportcard.com/badge/github.com/loxilb-io/loxilb)](https://goreportcard.com/report/github.com/loxilb-io/loxilb) [![OpenSSF Best Practices](https://www.bestpractices.dev/projects/8472/badge)](https://www.bestpractices.dev/projects/8472) ![build workflow](https://github.com/loxilb-io/loxilb/actions/workflows/docker-image.yml/badge.svg) ![sanity workflow](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity.yml/badge.svg)   
![apache](https://img.shields.io/badge/license-Apache-blue.svg) [![Info][docs-shield]][docs-url] [![Slack](https://img.shields.io/badge/community-join%20slack-blue)](https://join.slack.com/t/loxilb/shared_invite/zt-2b3xx14wg-P7WHj5C~OEON_jviF0ghcQ) 

## What is loxilb
loxilb is an open source cloud-native load-balancer based on GoLang/eBPF with the goal of achieving cross-compatibility across a wide range of on-prem, public-cloud or hybrid K8s environments. loxilb is being developed to support the adoption of cloud-native tech in telco, mobility, and edge computing.

## Kubernetes with loxilb

Kubernetes defines many service constructs like cluster-ip, node-port, load-balancer, ingress etc for pod to pod, pod to service and outside-world to service communication. 

![LoxiLB Cover](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/photos/loxilb-cover.png)

All these services are provided by load-balancers/proxies operating at Layer4/Layer7. Since Kubernetes's is highly modular,  these services can be provided by different software modules. For example, kube-proxy is used by default to provide cluster-ip and node-port services. For some services like LB and Ingress, no default is usually provided.

Service type load-balancer is usually provided by public cloud-provider(s) as a managed entity. But for on-prem and self-managed clusters, there are only a few good options available. Even for provider-managed K8s like EKS, there are many who would want to bring their own LB to clusters running anywhere. Additionally, Telco 5G and edge services introduce unique challenges due to the variety of exotic protocols involved, including GTP, SCTP, SRv6, SEPP, and DTLS, making seamless integration particularly challenging. <b>loxilb provides service type load-balancer as its main use-case</b>. loxilb can be run in-cluster or ext-to-cluster as per user need.

loxilb works as a L4 load-balancer/service-proxy by default. Although L4 load-balancing provides great performance and functionality, an equally performant L7 load-balancer is also necessary in K8s for various use-cases. loxilb also supports L7 load-balancing in the form of Kubernetes Ingress implementation which is enhanced with eBPF sockmap helpers. This also benefit users who need L4 and L7 load-balancing under the same hood.

Additionally, loxilb also supports:
- [x] kube-proxy replacement with eBPF(full cluster-mesh implementation for Kubernetes)
- [x] Ingress Support
- [x] Kubernetes Gateway API
- [ ] Kubernetes Network Policies 

## Telco-Cloud with loxilb
For deploying telco-cloud with cloud-native functions, loxilb can be used as an enhanced SCP(service communication proxy). SCP is a communication proxy defined by [3GPP](https://www.etsi.org/deliver/etsi_ts/129500_129599/129500/16.04.00_60/ts_129500v160400p.pdf) and aimed at telco micro-services running in cloud-native environment. Read more in this [blog](https://dev.to/nikhilmalik/5g-service-communication-proxy-with-loxilb-4242) 
![image](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/photos/scp.svg)

Telco-cloud requires load-balancing and communication across various interfaces/standards like N2, N4, E2(ORAN), S6x, 5GLAN, GTP etc. Each of these present its own unique challenges which loxilb aims to solve e.g.:    
- N4 requires PFCP level session-intelligence
- N2 requires NGAP parsing capability(Related Blogs - [Blog-1](https://www.loxilb.io/post/ngap-load-balancing-with-loxilb), [Blog-2](https://futuredon.medium.com/5g-sctp-loadbalancer-using-loxilb-b525198a9103), [Blog-3](https://medium.com/@ben0978327139/5g-sctp-loadbalancer-using-loxilb-applying-on-free5gc-b5c05bb723f0))
- S6x requires Diameter/SCTP multi-homing LB support(Related [Blog](https://www.loxilb.io/post/k8s-introducing-sctp-multihoming-functionality-with-loxilb))
- MEC use-cases might require UL-CL understanding(Related [Blog](https://futuredon.medium.com/5g-uplink-classifier-using-loxilb-7593a4d66f4c))
- Hitless failover support might be essential for mission-critical applications
- E2 might require SCTP-LB with OpenVPN bundled together
- SIP support is needed to enable cloud-native VOIP
- N32 requires support for Security Edge Protection Proxy(SEPP)

## Why choose loxilb?
   
- ```Performs``` much better compared to its competitors across various architectures
    * [Single-Node Performance](https://loxilb-io.github.io/loxilbdocs/perf-single/)  
    * [Multi-Node Performance](https://loxilb-io.github.io/loxilbdocs/perf-multi/) 
    * [Performance on ARM](https://www.loxilb.io/post/running-loxilb-on-aws-graviton2-based-ec2-instance)
    * [Short Demo on Performance](https://www.youtube.com/watch?v=MJXcM0x6IeQ)
- Utitlizes ebpf which makes it ```flexible``` as well as ```customizable```
- Advanced ```quality of service``` for workloads (per LB, per end-point or per client)
- Works with ```any``` Kubernetes distribution/CNI - k8s/k3s/k0s/kind/OpenShift + Calico/Flannel/Cilium/Weave/Multus etc
- Kube-proxy replacement with loxilb allows ```simple plug-in``` with any existing/deployed pod-networking software
- Extensive support for ```SCTP workloads``` (with multi-homing) on K8s
- Dual stack with ```NAT66, NAT64``` support for K8s
- K8s ```multi-cluster``` support (planned 🚧)
- Runs in ```any``` cloud (public cloud/on-prem) or ```standalone``` environments

## Overall features of loxilb
- L4/NAT stateful loadbalancer
    * NAT44, NAT66, NAT64 with One-ARM, FullNAT, DSR etc
    * Support for TCP, UDP, SCTP (w/ multi-homing), QUIC, FTP, TFTP etc
- High-availability support with BFD detection for hitless/maglev/cgnat clustering
- Extensive and scalable end-point liveness probes for cloud-native environments
- Stateful firewalling and IPSEC/Wireguard support
- Optimized implementation for features like [Conntrack](https://thermalcircle.de/doku.php?id=blog:linux:connection_tracking_1_modules_and_hooks), QoS etc
- Full compatibility for ipvs (ipvs policies can be auto inherited)
- Policy oriented L7 proxy support - HTTP1.0, 1.1, 2.0, 3.0   

## Components of loxilb 
- GoLang based control plane components
- A scalable/efficient [eBPF](https://ebpf.io/) based data-path implementation
- Integrated goBGP based routing stack
- A kubernetes operator [kube-loxilb](https://github.com/loxilb-io/kube-loxilb) written in Go
- A kubernetes ingress [implementation](https://github.com/loxilb-io/loxilb-ingress)

## Architectural Considerations   
- [Understanding loxilb modes and deployment in K8s with kube-loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/kube-loxilb.md)
- [Understanding High-availability with loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/ha-deploy.md)

## Getting Started  
#### loxilb as ext-cluster pod  
- [K3s : loxilb with default flannel](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k3s_quick_start_flannel.md)
- [K3s : loxilb with calico](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k3s_quick_start_calico.md)
- [K3s : loxilb with cilium](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/quick_start_with_cilium.md)
- [K0s : loxilb with default kube-router networking](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k0s_quick_start.md)
- [EKS : loxilb ext-mode](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/eks-external.md)

#### loxilb as in-cluster pod   
- [K3s : loxilb in-cluster mode](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k3s_quick_start_incluster.md)
- [K0s : loxilb in-cluster mode](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k0s_quick_start_incluster.md)
- [MicroK8s : loxilb in-cluster mode](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/microk8s_quick_start_incluster.md)
- [EKS : loxilb in-cluster mode](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/eks-incluster.md)

#### loxilb as service-proxy (kube-proxy replacement)
- [K3s : loxilb service-proxy with flannel](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/service-proxy-flannel.md)
- [K3s : loxilb service-proxy with calico](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/service-proxy-calico.md)

#### loxilb as Kubernetes Ingress
- [K3s: How to run loxilb-ingress](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/loxilb-ingress.md)

#### loxilb in standalone mode
- [Run loxilb standalone](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/standalone.md)

## Advanced Guides    
- [How-To : Service-group zones with loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/service-zones.md)
- [How-To : Access end-points outside K8s](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/ext-ep.md)
- [How-To : Deploy multi-server K3s HA with loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k3s-multi-master.md)
- [How-To : Deploy loxilb with multi-AZ HA support in AWS](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/aws-multi-az.md)
- [How-To : Deploy loxilb with multi-cloud HA support](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/multi-cloud-ha.md)
- [How-To : Deploy loxilb with ingress-nginx](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/loxilb-nginx-ingress.md)

## Knowledge-Base   
- [What is eBPF](ebpf.md)
- [What is k8s service - load-balancer](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/lb.md)
- [Architecture in brief](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/arch.md)
- [Code organization](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/code.md)
- [eBPF internals of loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/loxilbebpf.md)
- [What are loxilb NAT Modes](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/nat.md)
- [loxilb load-balancer algorithms](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/lb-algo.md)
- [Manual steps to build/run](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/run.md)
- [Debugging loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/debugging.md)
- [loxicmd command-line tool usage](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/cmd.md)
- [Developer's guide to loxicmd](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/cmd-dev.md)
- [Developer's guide to loxilb API](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/api-dev.md)
- [API Reference - loxilb web-Api](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/api.md)
- [Performance Reports](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/perf.md)
- [Development Roadmap](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/roadmap.md)
- [Contribute](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/contribute.md)
- [System Requirements](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/requirements.md)
- [Frequenctly Asked Questions- FAQs](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/faq.md)
- [Blogs](https://www.loxilb.io/blog)
- [Demo Videos](https://www.youtube.com/@loxilb697)

## Community 

### Slack 
Join the loxilb [Slack](https://www.loxilb.io/members) channel to chat with loxilb developers and other loxilb users. This is a good place to learn about loxilb, ask questions, and work collaboratively.

### General Discussion
Feel free to post your queries in github [discussion](https://github.com/loxilb-io/loxilb/discussions). If you find any issue/bugs, please raise an [issue](https://github.com/loxilb-io/loxilb/issues) in github and members from loxilb community will be happy to help.

### Community Posts
- [5G SCTP Load Balancer using LoxiLB](https://futuredon.medium.com/5g-sctp-loadbalancer-using-loxilb-b525198a9103)
- [5G Uplink Classifier using LoxiLB](https://futuredon.medium.com/5g-uplink-classifier-using-loxilb-7593a4d66f4c)
- [5G SCTP Load Balancer with free5gc](https://medium.com/@ben0978327139/5g-sctp-loadbalancer-using-loxilb-applying-on-free5gc-b5c05bb723f0)
- [K8s - Bring load balancing to Multus workloads with LoxiLB](https://cloudybytes.medium.com/k8s-bringing-load-balancing-to-multus-workloads-with-loxilb-a0746f270abe)
- [K3s - Using LoxiLB as External Service Load Balancer](https://cloudybytes.medium.com/k3s-using-loxilb-as-external-service-lb-2ea4ce61e159)
- [Kubernetes Services - Achieving Optimal performance is elusive](https://cloudybytes.medium.com/kubernetes-services-achieving-optimal-performance-is-elusive-5def5183c281)

## CICD Workflow Status

| Features(Ubuntu20.04) | Features(Ubuntu22.04)| Features(Ubuntu24.04)| Features(RedHat9)|
|:----------|:-------------|:-------------|:-------------|
| [![build workflow](https://github.com/loxilb-io/loxilb/actions/workflows/docker-image.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/docker-image.yml)  |  [![Docker-Multi-Arch](https://github.com/loxilb-io/loxilb/actions/workflows/docker-multiarch.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/docker-multiarch.yml) |  [![Docker-Multi-Arch](https://github.com/loxilb-io/loxilb/actions/workflows/docker-multiarch.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/docker-multiarch.yml) |  [![Docker-Multi-Arch](https://github.com/loxilb-io/loxilb/actions/workflows/docker-multiarch.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/docker-multiarch.yml) |
| [![simple workflow](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity.yml)  | [![Sanity-CI-Ubuntu-22](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity-ubuntu-22.yml) | [![Sanity-CI-Ubuntu-24](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity-ubuntu-24.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity-ubuntu-24.yml) | [![Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity-rh9.yml) |
| [![tcp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity.yml) | [![tcp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity-ubuntu-22.yml)   | [![tcp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity-ubuntu-24.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity-ubuntu-24.yml)   | [![TCP-LB-Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity-rh9.yml) | 
| [![udp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity.yml) | [![udp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity-ubuntu-22.yml) | [![udp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity-ubuntu-24.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity-ubuntu-24.yml) | [![UDP-LB-Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity-rh9.yml) |
| [![sctp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity.yml)  | [![SCTP-LB-Sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity-ubuntu-22.yml)  | [![SCTP-LB-Sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity-ubuntu-24.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity-ubuntu-24.yml) |[![SCTP-LB-Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity-rh9.yml)  |
|  [![extlb workflow](https://github.com/loxilb-io/loxilb/actions/workflows/advanced-lb-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/advanced-lb-sanity.yml)|  [![extlb workflow](https://github.com/loxilb-io/loxilb/actions/workflows/advanced-lb-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/advanced-lb-sanity-ubuntu-22.yml) |  [![extlb workflow](https://github.com/loxilb-io/loxilb/actions/workflows/advanced-lb-sanity-ubuntu-24.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/advanced-lb-sanity-ubuntu-24.yml) | [![Adv-LB-Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/advanced-lb-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/advanced-lb-sanity-rh9.yml)|
| [![nat66-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity.yml)   | [![nat66-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity-ubuntu-22.yml)  |  [![nat66-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity-ubuntu-24.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity-ubuntu-24.yml)  | [![NAT66-LB-Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity-rh9.yml) | 
|  [![ipsec-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity.yml)   | [![ipsec-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity-ubuntu-22.yml)  |  [![ipsec-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity-ubuntu-24.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity-ubuntu-24.yml)  | [![IPsec-Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity-rh9.yml) |
| [![liveness-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/liveness-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/liveness-sanity.yml)  | [![liveness-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/liveness-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/liveness-sanity-ubuntu-22.yml)  |  [![liveness-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/liveness-sanity-ubuntu-24.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/liveness-sanity-ubuntu-24.yml)   | [![liveness-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/liveness-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/liveness-sanity-rh9.yml) |
|![scale-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/scale-sanity.yml/badge.svg)  | [![Scale-Sanity-CI-Ubuntu-22](https://github.com/loxilb-io/loxilb/actions/workflows/scale-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/scale-sanity-ubuntu-22.yml) |  [![Scale-Sanity-CI-Ubuntu-24](https://github.com/loxilb-io/loxilb/actions/workflows/scale-sanity-ubuntu-24.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/scale-sanity-ubuntu-24.yml)  | |
|[![perf-CI](https://github.com/loxilb-io/loxilb/actions/workflows/perf.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/perf.yml) | [![perf-CI](https://github.com/loxilb-io/loxilb/actions/workflows/perf.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/perf.yml) |[![perf-CI](https://github.com/loxilb-io/loxilb/actions/workflows/perf.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/perf.yml) | |

| K3s Tests | K8s Cluster Tests | EKS Test |
|:-------------|:-------------|:-------------|
|[![K3s-Base-Sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-base-sanity.yml/badge.svg?branch=main)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-base-sanity.yml) | [![K8s-Calico-Cluster-IPVS-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs.yml) | ![EKS](https://github.com/loxilb-io/loxilb/actions/workflows/eks.yaml/badge.svg?branch=main)  |
| [![k3s-flannel-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel.yml) | [![K8s-Calico-Cluster-IPVS2-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs2.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs2.yml) | |
| [![k3s-flannel-ubuntu22-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-ubuntu-22.yml) | [![K8s-Calico-Cluster-IPVS3-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs3.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs3.yml) | |
|[![k3s-flannel-cluster-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-cluster.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-cluster.yml) | [![K8s-Calico-Cluster-IPVS3-HA-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs3-ha.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs3-ha.yml) | |
| [![k3s-flannel-incluster-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-incluster.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-incluster.yml)   |  |  |
|[![k3s-flannel-incluster-l2-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-incluster-l2.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-incluster-l2.yml)  | | |
| [![k3s-calico-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-calico.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-calico.yml)  | | |
| [![k3s-cilium-cluster-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-cilium-cluster.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-cilium-cluster.yml) | |
| [![k3s-sctpmh-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-sctpmh.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-sctpmh.yml)  | | |
| [![k3s-sctpmh-ubuntu22-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-sctpmh-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-sctpmh-ubuntu22.yml) | | |
| [![k3s-sctpmh-2-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-sctpmh-2.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-sctpmh-2.yml)  | | |


## 📚 Please check loxilb [website](https://www.loxilb.io) for more detailed info.   

[docs-shield]: https://img.shields.io/badge/info-docs-blue
[docs-url]: https://loxilb-io.github.io/loxilbdocs/
[slack=shield]: https://img.shields.io/badge/Community-Join%20Slack-blue
[slack-url]: https://www.loxilb.io/members

