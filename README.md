[![eBPF Emerging Project](https://img.shields.io/badge/ebpf.io-Emerging--App-success)](https://ebpf.io/projects#loxilb) [![Go Report Card](https://goreportcard.com/badge/github.com/loxilb-io/loxilb)](https://goreportcard.com/report/github.com/loxilb-io/loxilb) ![build workflow](https://github.com/loxilb-io/loxilb/actions/workflows/docker-image.yml/badge.svg) ![sanity workflow](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity.yml/badge.svg) ![apache](https://img.shields.io/badge/license-Apache-blue.svg) [![Info][docs-shield]][docs-url] [![Slack](https://img.shields.io/badge/community-join%20slack-blue)](https://www.loxilb.io/members)  
## What is loxilb

loxilb is an open source hyper-scale software load-balancer for cloud-native workloads. It uses eBPF as its core-engine and is based on Golang. It is designed to power on-premise, edge and public-cloud Kubernetes cluster deployments.

###  🚀 loxilb aims to provide the following :   

- Service type load-balancer for kubernetes   
    * L4/NAT stateful loadbalancer   
    * NAT44, NAT66, NAT64 with One-ARM, FullNAT, DSR etc   
    * Support for TCP, UDP, SCTP (w/ multi-homing), FTP, TFTP etc   
    * High-availability support with hitless/maglev clustering   
    * Full compliance for K8s loadbalancer Spec
    * Multi-cluster support      
-  Extensive and scalable liveness probes for cloud-native environments    
-  High-perf replacement for the *aging* iptables/ipvs   
-  L7 proxy support   
-  Telco/5G/6G friendly features    
    * GTP tunnels as first class citizens     
    * Optimized SRv6 implementation    
    * Support for UL-CL with LB, QFI and other utility extensions  

### 🧿 loxilb is powered by :   
- Bespoke GoLang based control plane components     
- [eBPF](https://ebpf.io/) based data-path forwarding   
   * Home-grown stack with advanced features like [Conntrack](https://thermalcircle.de/doku.php?id=blog:linux:connection_tracking_1_modules_and_hooks), QoS etc
   * Complete kernel networking bypass    
   * Highly scalable with low-latency & high throughput   
- GoLang based easy to use APIs/Interfaces infra   
- Seamless integration with goBGP based routing stack    

### 📚 Check loxilb [Documentation](https://loxilb-io.github.io/loxilbdocs/) for more info.   

[docs-shield]: https://img.shields.io/badge/info-docs-blue
[docs-url]: https://loxilb-io.github.io/loxilbdocs/
[slack=shield]: https://img.shields.io/badge/Community-Join%20Slack-blue
[slack-url]: https://www.loxilb.io/members

