[![eBPF Emerging Project](https://img.shields.io/badge/ebpf.io-Emerging--App-success)](https://ebpf.io/projects#loxilb) [![Go Report Card](https://goreportcard.com/badge/github.com/loxilb-io/loxilb)](https://goreportcard.com/report/github.com/loxilb-io/loxilb) ![build workflow](https://github.com/loxilb-io/loxilb/actions/workflows/docker-image.yml/badge.svg) ![sanity workflow](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity.yml/badge.svg) ![apache](https://img.shields.io/badge/license-Apache-blue.svg) [![Info][docs-shield]][docs-url] [![Slack](https://img.shields.io/badge/community-join%20slack-blue)](https://join.slack.com/t/loxilb/shared_invite/zt-2b3xx14wg-P7WHj5C~OEON_jviF0ghcQ)  

## What is loxilb

loxilb is an open source cloud-native load-balancer based on GoLang/eBPF with the goal of achieving cross-compatibity across a wide range of on-prem, public-cloud or hybrid K8s environments.

## Kubernetes with loxilb

Kubernetes defines many service constructs like cluster-ip, node-port, load-balancer etc for pod to pod, pod to service and service from outside communication possible in a seamless manner.

<img src="https://github.com/UltraInstinct14/loxilb/assets/75648333/2a6ef6fd-e1a8-4377-94bd-a35a15555761" width=50% height=50%>

All these services are provided by load-balancers/proxies operating at Layer4/Layer7. Due to Kubernetes's highly modular architecture,  these services can be provided by different software modules. For example, kube-proxy is used to provide cluster-ip and node-port services by default. Service type load-balancer is usually provided by public cloud-provider as a managed service. But for on-prem and self-managed clusers, there are only a few good options. loxilb supports <b>service type load-balancer</b> as its main use-case. 

Additionally, loxilb can also support cluster-ip and node-port services and thereby providing end-to-end networking requirements for Kubernetes.

## Why choose loxilb?
   
- ```Performs``` much better compared to its competitors across various architectures
    * [Single-Node Performance](https://loxilb-io.github.io/loxilbdocs/perf-single/)  
    * [Multi-Node Performance](https://loxilb-io.github.io/loxilbdocs/perf-multi/) 
    * [Performance on ARM](https://www.loxilb.io/post/running-loxilb-on-aws-graviton2-based-ec2-instance)
    * [Short Demo on Performance](https://www.youtube.com/watch?v=MJXcM0x6IeQ)
- Utitlizes ebpf which makes it ```flexible``` as well as ```customizable```
- Advanced ```quality of service``` for workloads (per LB, per end-point or per client)
- Works with ```any``` Kubernetes distribution/CNI - k8s/k3s/k0s/kind/OpenShift + Calico/Flannel/Cilium/Weave/Multus etc
- Extensive support for ```SCTP workloads``` (with multi-homing) on k8s
- Dual stack with ```NAT66, NAT64``` support for k8s
- k8s ```multi-cluster``` support (planned 🚧)
- Runs in ```any``` cloud (public cloud/on-prem) or ```standalone``` environments

## Overall features of loxilb
- L4/NAT stateful loadbalancer
    * NAT44, NAT66, NAT64 with One-ARM, FullNAT, DSR etc
    * Support for TCP, UDP, SCTP (w/ multi-homing), QUIC, FTP, TFTP etc
- High-availability support with hitless/maglev/cgnat clustering
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

## Layer4 or Layer7
loxilb works as a L4 load-balancer/service-mesh by default. Although it provides great performance, at times L7 load-balancing is necessary in K8s. There are many good L7 proxies already available for k8s. Still, we are working on providing a great L7 solution natively in eBPF. It is a tough endeavor one which which should reap great benefits once completed. Please keep an eye for updates on this.

## Telco-Cloud with loxilb
For deploying telco-cloud with cloud-native functions, loxilb can be used as a SCP(service communication proxy). SCP is nothing but a glorified term for Kubernetes load-balancing. But telco-cloud required load-balancing across various interfaces/standards like N2, N4, E2(ORAN), S6x, 5GLAN, GTP etc. Each of these interfaces present its own unique challenges while doing load-balancing e.g.
- N4 requires PFCP level session-intelligence
- N2 requires NGAP parsing capability
- S6x requires Diameter/SCTP multi-homing LB support
- MEC use-cases might require UL-CL understanding
- Hitless failover support might be essential for mission-critical applications
- E2 might require SCTP-LB with OpenVPN bundled together

## 📚 Please check loxilb [Documentation](https://loxilb-io.github.io/loxilbdocs/) for more detailed info.   

[docs-shield]: https://img.shields.io/badge/info-docs-blue
[docs-url]: https://loxilb-io.github.io/loxilbdocs/
[slack=shield]: https://img.shields.io/badge/Community-Join%20Slack-blue
[slack-url]: https://www.loxilb.io/members

