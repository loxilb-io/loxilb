<p align="center" width="30%">
    <img width="30%" src="https://github.com/loxilb-io/loxilbdocs/blob/main/docs/photos/loxilb-logo-text.png">
</p>


[![Website](https://img.shields.io/static/v1?label=www&message=loxilb.io&color=blue?style=for-the-badge&logo=appveyor)](https://www.loxilb.io) [![eBPF Emerging Project](https://img.shields.io/badge/ebpf.io-Emerging--App-success)](https://ebpf.io/projects#loxilb) [![Go Report Card](https://goreportcard.com/badge/github.com/loxilb-io/loxilb)](https://goreportcard.com/report/github.com/loxilb-io/loxilb) ![build workflow](https://github.com/loxilb-io/loxilb/actions/workflows/docker-image.yml/badge.svg) ![sanity workflow](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity.yml/badge.svg) ![apache](https://img.shields.io/badge/license-Apache-blue.svg) [![Info][docs-shield]][docs-url] [![Slack](https://img.shields.io/badge/community-join%20slack-blue)](https://join.slack.com/t/loxilb/shared_invite/zt-2b3xx14wg-P7WHj5C~OEON_jviF0ghcQ) [![OpenSSF Best Practices](https://www.bestpractices.dev/projects/8472/badge)](https://www.bestpractices.dev/projects/8472)  

## What is loxilb
loxilb is an open source cloud-native load-balancer based on GoLang/eBPF with the goal of achieving cross-compatibility across a wide range of on-prem, public-cloud or hybrid K8s environments.

## Kubernetes with loxilb

Kubernetes defines many service constructs like cluster-ip, node-port, load-balancer etc for pod to pod, pod to service and service from outside communication. 
<p align="center">
<img src="https://github.com/loxilb-io/loxilb/assets/75648333/6f933bcf-96b7-42ba-bfe2-ea4b85b9a73b" width=50% height=50%>
</p>

All these services are provided by load-balancers/proxies operating at Layer4/Layer7. Since Kubernetes's is highly modular,  these services can be provided by different software modules. For example, kube-proxy is used by default to provide cluster-ip and node-port services. 

Service type load-balancer is usually provided by public cloud-provider(s) as a managed entity. But for on-prem and self-managed clusters, there are only a few good options available. Even for provider-managed K8s like EKS, there are many who would want to bring their own LB to clusters running anywhere. <b>loxilb provides service type load-balancer as its main use-case</b>. loxilb can be run in-cluster or ext-to-cluster as per user need.  

Additionally, loxilb can also support cluster-ip and node-port services, thereby providing end-to-end connectivity for Kubernetes.

## Why choose loxilb?
   
- ```Performs``` much better compared to its competitors across various architectures
    * [Single-Node Performance](https://loxilb-io.github.io/loxilbdocs/perf-single/)  
    * [Multi-Node Performance](https://loxilb-io.github.io/loxilbdocs/perf-multi/) 
    * [Performance on ARM](https://www.loxilb.io/post/running-loxilb-on-aws-graviton2-based-ec2-instance)
    * [Short Demo on Performance](https://www.youtube.com/watch?v=MJXcM0x6IeQ)
- Utitlizes ebpf which makes it ```flexible``` as well as ```customizable```
- Advanced ```quality of service``` for workloads (per LB, per end-point or per client)
- Works with ```any``` Kubernetes distribution/CNI - k8s/k3s/k0s/kind/OpenShift + Calico/Flannel/Cilium/Weave/Multus etc
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
- Policy oriented L7 proxy support - HTTP1.0, 1.1, 2.0 etc (planned 🚧)   

## Components of loxilb 
- GoLang based control plane components
- A scalable/efficient [eBPF](https://ebpf.io/) based data-path implementation
- Integrated goBGP based routing stack
- A kubernetes agent [kube-loxilb](https://github.com/loxilb-io/kube-loxilb) written in Go

## Layer4 Vs Layer7
loxilb works as a L4 load-balancer/service-mesh by default. Although it provides great performance, at times, L7 load-balancing might become necessary in K8s. There are many good L7 proxies already available for K8s. Still, we are working on providing a great L7 solution natively in eBPF. It is a tough endeavor one which should reap great benefits once completed. Please keep an eye for updates on this.

## Telco-Cloud with loxilb
For deploying telco-cloud with cloud-native functions, loxilb can be used as a SCP(service communication proxy). SCP is nothing but a glorified term for Kubernetes load-balancing/proxy. But telco-cloud requires load-balancing across various interfaces/standards like N2, N4, E2(ORAN), S6x, 5GLAN, GTP etc. Each of these interfaces present its own unique challenges(and DPI) for load-balancing which loxilb aims to solve e.g.
- N4 requires PFCP level session-intelligence
- N2 requires NGAP parsing capability
- S6x requires Diameter/SCTP multi-homing LB support
- MEC use-cases might require UL-CL understanding
- Hitless failover support might be essential for mission-critical applications
- E2 might require SCTP-LB with OpenVPN bundled together

## Getting Started
- [How-To : Deploy loxilb in K8s with kube-loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/kube-loxilb.md)
- [How-To : Run in K8s with in-cluster mode](https://www.loxilb.io/post/k8s-nuances-of-in-cluster-external-service-lb-with-loxilb)
- [How-To : High-availability with loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/ha-deploy.md)
- [How-To : Run loxilb in standalone mode](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/standalone.md)
- [How-To : Manual build/run](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/run.md)
- [How-To : Standalone configuration](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/cmd.md)
- [How-To : debug](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/debugging.md)

## Getting started with different K8s distributions & tools

- [K3s : loxilb with default flannel](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k3s_quick_start_flannel.md)
- [K3s : loxilb with cilium](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/quick_start_with_cilium.md)

## Knowledge-Base   
- [What is eBPF](ebpf.md)
- [What is k8s service - load-balancer](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/lb.md)
- [Architecture in brief](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/arch.md)
- [Code organization](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/code.md)
- [eBPF internals of loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/loxilbebpf.md)
- [What are loxilb NAT Modes](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/nat.md)
- [loxilb load-balancer algorithms](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/lb-algo.md)
- [Developer's guide to loxicmd](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/cmd-dev.md)
- [Developer's guide to loxilb API](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/api-dev.md)
- [API Reference - loxilb web-Api](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/api.md)
- [Performance Reports](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/perf.md)
- [Development Roadmap](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/roadmap.md)
- [Contribute](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/contribute.md)
- [System Requirements](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/requirements.md)
- [Frequenctly Asked Questions- FAQs](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/faq.md)
- [Blogs](https://www.loxilb.io/blog)

## Community 

### Slack 
Join the loxilb [Slack](https://www.loxilb.io/members) channel to chat with loxilb developers and other loxilb users. This is a good place to learn about loxilb, ask questions, and work collaboratively.

### General Discussion
Feel free to post your queries in github [discussion](https://github.com/loxilb-io/loxilb/discussions). If you find any issue/bugs, please raise an [issue](https://github.com/loxilb-io/loxilb/issues) in github and members from loxilb community will be happy to help.

## CICD Status

### Features Sanity
![build workflow](https://github.com/loxilb-io/loxilb/actions/workflows/docker-image.yml/badge.svg)  
![simple workflow](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity.yml/badge.svg)   
[![tcp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity.yml)   
[![udp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity.yml)   
[![sctp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity.yml)   
![extlb workflow](https://github.com/loxilb-io/loxilb/actions/workflows/advanced-lb-sanity.yml/badge.svg)   
![ipsec-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity.yml/badge.svg)  
![scale-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/scale-sanity.yml/badge.svg)   
[![liveness-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/liveness-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/liveness-sanity.yml)   
![nat66-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity.yml/badge.svg)   
![perf-CI](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity.yml/badge.svg)   

### Features sanity in Ubuntu22.04
[![tcp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity-ubuntu-22.yml)   
[![udp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity-ubuntu-22.yml)   
![ipsec-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity-ubuntu-22.yml/badge.svg)  
![nat66-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity-ubuntu-22.yml/badge.svg)   

### K3s Sanity
[![K3s-Base-Sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-base-sanity.yml/badge.svg?branch=main)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-base-sanity.yml)  

#### K3s Flannel Sanity  
[![k3s-flannel-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel.yml)   
[![k3s-flannel-ubuntu22-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-ubuntu-22.yml)  
[![k3s-flannel-cluster-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-cluster.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-cluster.yml)   
[![k3s-flannel-incluster-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-incluster.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-incluster.yml)   
[![k3s-flannel-incluster-l2-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-incluster-l2.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-flannel-incluster-l2.yml)   

#### K3s calico Sanity  
[![k3s-calico-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-calico.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-calico.yml)   
[![k3s-calico-ubuntu22-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-calico-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-calico-ubuntu-22.yml)    

#### K3s cilium Sanity  
[![k3s-cilium-cluster-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-cilium-cluster.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-cilium-cluster.yml)  

#### K3s sctp multihoming Sanity  
[![k3s-sctpmh-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-sctpmh.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-sctpmh.yml)   
[![k3s-sctpmh-ubuntu22-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-sctpmh-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-sctpmh-ubuntu22.yml)   
[![k3s-sctpmh-2-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-sctpmh-2.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k3s-sctpmh-2.yml)  

### K8s Sanity  
[![K8s-Calico-Cluster-IPVS-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs.yml)   
[![K8s-Calico-Cluster-IPVS2-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs2.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs2.yml)   
[![K8s-Calico-Cluster-IPVS3-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs3.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs3.yml)   
[![K8s-Calico-Cluster-IPVS3-HA-CI](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs3-ha.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/k8s-calico-ipvs3-ha.yml)   

### EKS Sanity
![EKS](https://github.com/loxilb-io/loxilb/actions/workflows/eks.yaml/badge.svg?branch=main)   


## 📚 Please check loxilb [Documentation](https://loxilb-io.github.io/loxilbdocs/) for more detailed info.   

[docs-shield]: https://img.shields.io/badge/info-docs-blue
[docs-url]: https://loxilb-io.github.io/loxilbdocs/
[slack=shield]: https://img.shields.io/badge/Community-Join%20Slack-blue
[slack-url]: https://www.loxilb.io/members

