[![eBPF Emerging Project](https://img.shields.io/badge/ebpf.io-Emerging--Project-success)](https://ebpf.io/projects#loxilb) [![Go Report Card](https://goreportcard.com/badge/github.com/loxilb-io/loxilb)](https://goreportcard.com/report/github.com/loxilb-io/loxilb) ![build workflow](https://github.com/loxilb-io/loxilb/actions/workflows/docker-image.yml/badge.svg) ![sanity workflow](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity.yml/badge.svg) ![apache](https://img.shields.io/badge/license-Apache-blue.svg)
## What is loxilb

loxilb is a cloud-native "edge" load-balancer stack built from grounds up using eBPF at its core. loxilb aims to provide the following :

- Service type external load-balancer for kubernetes
- L4/NAT stateful loadbalancer
   * NAT44, NAT66, NAT64 with One-ARM, FullNAT, DSR etc
   * High-availability support
   * K8s CCM compliance
   * High-perf replacement for the *aging* iptables/ipvs 
-  Optimized SRv6 implementation in eBPF 
-  L7 proxy support
-  Make GTP tunnels first class citizens of the Linux world 
   * Support for QFI and other extension headers
-  eBPF based data-path forwarding (Dual BSD/GPLv2 license)
   * Complete kernel networking bypass with home-grown stack for advanced features like [Conntrack](https://thermalcircle.de/doku.php?id=blog:linux:connection_tracking_1_modules_and_hooks), QoS etc
   * Highly scalable with low-latency & high througput 
-  goLang based control plane components (Apache license)
-  Seamless integration with goBGP based routing stack
-  GoLang based easy to use APIs/Interfaces for developers

## How to build/run/use

We encourage loxilb users to follow various guides in loxilb docs [repository](https://github.com/loxilb-io/loxilbdocs).
