
[![eBPF Emerging Project](https://img.shields.io/badge/ebpf.io-Emerging--Project-success)](https://ebpf.io/projects#loxilb) ![apache](https://img.shields.io/badge/license-Apache-blue.svg) ![bsd](https://img.shields.io/badge/license-BSD-blue.svg) ![gpl](https://img.shields.io/badge/license-GPL-blue.svg)

![build workflow](https://github.com/loxilb-io/loxilb/actions/workflows/docker-image.yml/badge.svg) ![sanity workflow](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity.yml/badge.svg)    

## What is loxilb

loxilb is a cloud-native "edge" load-balancer stack built from grounds up using eBPF at its core. loxilb aims to provide the following :

- Service type external load-balancer for kubernetes (hence the name loxilb)
- L4/NAT stateful loadbalancer 
   * High-availability support
   * K8s CCM compliance
-  Optimized SRv6 implementation in eBPF 
-  L7 proxy support
-  Make GTP tunnels first class citizens of the Linux world 
   * Support for QFI and other extension headers
-  eBPF based kernel forwarding (GPLv2 license)
   * Complete kernel bypass with home-grown stack for advanced features like [Conntrack](https://thermalcircle.de/doku.php?id=blog:linux:connection_tracking_1_modules_and_hooks), QoS etc
   * Highly scalable with low-latency & high througput 
   * Mainly uses TC-eBPF hooks
-  goLang based control plane components (Apache license)
-  Seamless integration with goBGP based routing stack
-  Easily cuztomizable to run in DPU environments
   * goLang based easy to use APIs/Interfaces


## How to build/run/use

We encourage loxilb users to follow various guides in loxilb docs [repository](https://github.com/loxilb-io/loxilbdocs)
