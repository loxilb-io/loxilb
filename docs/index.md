# loxilbdocs - Documentation about loxilb

## loxilb Background 
loxilb started as a project to ease deployments of cloud-native/kubernetes workloads for the edge. When we deploy services in public clouds like AWS/GCP, the services becomes easily accessible or exported to the outside world. The public cloud providers, usually by default, associate load-balancer instances for incoming requests to these services to ensure everything is quite smooth. 

However, for on-prem and edge deployments, there is no *service type - external load balancer* provider by default. For a long time, MetalLB from google was the only choice for the needy. But edge services are a different ball game altogether due to the fact that there are so many exotic protocols in play like GTP, SCTP, SRv6 etc and integrating everything into a seamlessly working solution has been quite difficult.

loxilb dev team was approached by many people who wanted to solve this problem. As a first step to solve the problem, it became apparent that networking stack provided by Linux kernel, although very solid,  really lacked the development process agility to quickly provide the permutations and combinations of protocols and the necessary encap, decap and stateful load-balancing. Our search led us to the awesome tech developed by the Linux community - eBPF. The flexibility to introduce new functionality into Kernel as a sandbox program was a complete fit to our design philosophy. Although we did consider DPDK for a while, but the fact that it needs dedicated cores/CPUs really defeats the whole purpose of making energy-efficient edge architectures.

During the journey of loxilb's development, we developed many other generic networking/security/visibility features in it using eBPF which can be used for various other purposes not specific to load-balancer only. But we decided to stick our original name *loxilb*. loxilb team hopes the open-source community finds it helpful.

## loxilb Aim/Goals

loxilb aims to provide the following :

-  Service type external load-balancer for kubernetes (hence the name loxilb)
   - *L4/NAT stateful loadbalancer*
   - *High-availability support*
   - *K8s CCM compliance*
-  Optimized SRv6 implementation in eBPF 
-  Make GTP tunnels first class citizens of the Linux world 
   - *Support for QFI and other extension headers*  
-  eBPF/XDP based kernel forwarding (GPLv2 license)
   - *Complete kernel bypass with built-in advanced features like conntrack, QoS etc*
   - *Highly scalable with low-latency & high througput*
   - *Hybrid stack utilizing both XDP and TC-eBPF* 
-  goLang based control plane components (Apache license)
-  Seamless integration with goBGP based routing stack
-  Easily cuztomizable to run in DPU environments
   - *goLang based easy to use APIs/Interfaces*
-  Cloud-Native Network Function (CNF) form-factor by default

## Documentation

- [What is eBPF](ebpf.md)
- [What is service type - external load-balancer](lb.md)
- [Architecture in brief](arch.md)
- [Code organization](code.md)
- [eBPF/XDP internals of loxilb](loxilbebpf.md)
- [Howto - build/run](/run.md)
- [Howto - ccm plugin](ccm.md)
- [Howto - debug](debugging.md)
- [Cmd/Config guide](cmd.md)
- [Api guide](api.md)
- [Performance](perf.md)
- [Development Roadmap](roadmap.md)
- [Contribute](contribute.md)
- [FAQs](faq.md)

### Host OS requirements
To install Loxilight software packages, you need the 64-bit version of one of these OS versions:
* Ubuntu Focal 20.04(LTS)
* Ubuntu Hirsute 21.04
* RockyOS
* Enterprise Redhat (Planned)
* Windows Server(Planned)

### Linux Kernel Requirements
* Linux Kernel Version >= 5.1.0

### Compatible Kubernetes Versions
* Kubernetes 1.19
* Kubernetes 1.20
* Kubernetes 1.21
* Kubernetes 1.22


