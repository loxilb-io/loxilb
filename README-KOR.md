![image](https://github.com/loxilb-io/loxilb/assets/75648333/87da0183-1a65-493f-b6fe-5bc738ba5468)


[![Website](https://img.shields.io/static/v1?label=www&message=loxilb.io&color=blue?style=for-the-badge&logo=appveyor)](https://www.loxilb.io) [![eBPF Emerging Project](https://img.shields.io/badge/ebpf.io-Emerging--App-success)](https://ebpf.io/projects#loxilb) [![Go Report Card](https://goreportcard.com/badge/github.com/loxilb-io/loxilb)](https://goreportcard.com/report/github.com/loxilb-io/loxilb) [![OpenSSF Best Practices](https://www.bestpractices.dev/projects/8472/badge)](https://www.bestpractices.dev/projects/8472) ![Docker-Multi-Arch](https://github.com/loxilb-io/loxilb/actions/workflows/docker-multiarch.yml/badge.svg) ![sanity workflow](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity.yml/badge.svg)   
![apache](https://img.shields.io/badge/license-Apache-blue.svg) [![Info][docs-shield]][docs-url] [![Slack](https://img.shields.io/badge/community-join%20slack-blue)](https://join.slack.com/t/loxilb/shared_invite/zt-2b3xx14wg-P7WHj5C~OEON_jviF0ghcQ) 

## loxilb란 무엇인가?
loxilb는 GoLang/eBPF를 기반으로 한 오픈 소스 클라우드 네이티브 로드 밸런서로, 온-프레미스, 퍼블릭 클라우드 또는 하이브리드 K8s 환경 전반에 걸쳐 호환성을 달성하는 것을 목표로 합니다. loxilb는 텔코 클라우드(5G/6G), 모빌리티 및 엣지 컴퓨팅에서 클라우드 네이티브 기술 채택을 지원하기 위해 개발되고 있습니다.

## loxilb와 함께하는 Kubernetes

Kubernetes는 ClusterIP, NodePort, LoadBalancer, Ingress 등 여러 서비스 구조를 정의하여 파드에서 파드로, 파드에서 서비스로, 외부 에서 서비스로의 통신을 가능하게 합니다.

![LoxiLB Cover](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/photos/loxilb-cover.png)

이 모든 서비스는 Layer4/Layer7에서 작동하는 로드 밸런서/프록시가 제공합니다. Kubernetes는 매우 모듈화되어 있으며, 다양한 소프트웨어 모듈이 이러한 서비스를 제공할 수 있습니다. 예를 들어, kube-proxy는 기본적으로 ClusterIP 와 NodePort 서비스를 제공하지만, LoadBalancer 와 Ingress 같은 일부 서비스는 기본적으로 제공되지 않습니다.

로드 밸런서 서비스는 일반적으로 퍼블릭 클라우드 제공자가 관리 구성 요소로 함께 제공합니다. 그러나 온프레미스 및 자체 관리 클러스터의 경우 사용할 수 있는 옵션이 제한적입니다. 매니지드 K8S 서비스(예: EKS)의 경우에도 로드 밸런서를 클러스터 어디서나 가져오려는 사람들이 많습니다. 추가적으로, 텔코 5G/6G 및 엣지 서비스는 GTP, SCTP, SRv6, DTLS와 같은 범용적이지 않은 프로토콜 사용으로 인해 기존 K8S 서비스에서의 원활한 통합이 특히 어렵습니다. <b>loxilb는 로드 밸런서 서비스 유형 기능을 주요 사용 사례로 제공합니다</b>. loxilb는 사용자의 필요에 따라 클러스터 내 또는 클러스터 외부에서 실행할 수 있습니다.

loxilb는 기본적으로 L4 로드 밸런서/서비스 프록시로 작동합니다. L4 로드 밸런싱이 우수한 성능과 기능을 제공하지만, 다양한 사용 사례를 위해 K8s에서 동일하게 성능이 뛰어난 L7 로드 밸런서도 필요합니다. loxilb는 또한 eBPF SOCKMAP Helper를 사용하여 향상된 Kubernetes Ingress 구현 형태로 L7 로드 밸런싱을 지원합니다. 이는 동일한 환경에서 L4와 L7 로드 밸런싱이 필요한 사용자에게도 유리합니다.

추가적으로 loxilb는 다음을 지원합니다:
- [x] eBPF를 통한 kube-proxy 교체(Kubernetes의 전체 클러스터 메쉬 구현)
- [x] 인그레스 지원
- [x] Kubernetes Gateway API
- [ ] Kubernetes 네트워크 정책

## loxilb와 함께하는 텔코 클라우드
클라우드 네이티브 기능으로 텔코-클라우드를 배포하려면 loxilb를 SCP(Service Communication Proxy: 서비스 통신 프록시)로 사용할 수 있습니다. SCP는 [3GPP](https://www.etsi.org/deliver/etsi_ts/129500_129599/129500/16.04.00_60/ts_129500v160400p.pdf)에서 정의한 통신 프록시로, 클라우드 네이티브 환경에서 실행되는 텔코 마이크로 서비스에 목적을 두고 있습니다. 자세한 내용은 이 [블로그](https://dev.to/nikhilmalik/5g-service-communication-proxy-with-loxilb-4242)를 참조하십시오.
![image](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/photos/scp.svg)

텔코-클라우드는 N2, N4, E2(ORAN), S6x, 5GLAN, GTP 등 다양한 인터페이스와 표준을 통한 로드 밸런싱 및 통신을 필요로 합니다. 각각 고유한 챌린지를 요구하며, loxilb는 이를 해결하는 것을 목표로 합니다. 예를 들어:
- N4는 PFCP 수준의 세션 인텔리전스를 요구합니다.
- N2는 NGAP 파싱 기능이 필요합니다(관련 블로그 - [블로그-1](https://www.loxilb.io/post/ngap-load-balancing-with-loxilb), [블로그-2](https://futuredon.medium.com/5g-sctp-loadbalancer-using-loxilb-b525198a9103), [블로그-3](https://medium.com/@ben0978327139/5g-sctp-loadbalancer-using-loxilb-applying-on-free5gc-b5c05bb723f0)).
- S6x는 Diameter/SCTP 멀티-호밍 LB 지원이 필요합니다(관련 [블로그](https://www.loxilb.io/post/k8s-introducing-sctp-multihoming-functionality-with-loxilb)).
- MEC 사용 사례는 UL-CL 이해가 필요할 수 있습니다(관련 [블로그](https://futuredon.medium.com/5g-uplink-classifier-using-loxilb-7593a4d66f4c)).
- 미션 크리티컬 애플리케이션을 위해 히트리스 장애 조치 지원이 필수적일 수 있습니다.
- E2는 OpenVPN과 번들된 SCTP-LB가 필요할 수 있습니다.
- 클라우드 네이티브 VOIP를 가능하게 하는 SIP 지원이 필요합니다.

## loxilb를 선택해야 하는 이유?
   
- 다양한 아키텍처 전반에서 경쟁자보다 ```성능```이 훨씬 뛰어납니다.
    * [싱글 노드 성능](https://loxilb-io.github.io/loxilbdocs/perf-single/)  
    * [멀티 노드 성능](https://loxilb-io.github.io/loxilbdocs/perf-multi/) 
    * [ARM에서의 성능](https://www.loxilb.io/post/running-loxilb-on-aws-graviton2-based-ec2-instance)
    * [성능 관련 데모](https://www.youtube.com/watch?v=MJXcM0x6IeQ)
- ebpf를 활용하여 ```유연```하고 ```사용자 정의```가 가능합니다.
- 워크로드에 대한 고급 ```서비스 품질```(LB별, 엔드포인트별 또는 클라이언트별)
- ```어떤``` Kubernetes 배포판/CNI와도 호환 - k8s/k3s/k0s/kind/OpenShift + Calico/Flannel/Cilium/Weave/Multus 등
- loxilb를 사용한 kube-proxy 교체는 ```간단한 플러그인```으로 기존에 배포된 파드 네트워킹 소프트웨어와 통합이 가능합니다.
- K8s에서 ```SCTP 워크로드```(멀티-호밍 포함)에 대한 광범위한 지원
- ```NAT66, NAT64```를 지원하는 듀얼 스택 K8s
- ```멀티 클러스터``` K8s 지원 (계획 중 🚧)
- ```어떤``` 클라우드(퍼블릭 클라우드/온프레미스) 또는 ```독립형``` 환경에서도 실행 가능

## loxilb의 전반적인 기능
- L4/NAT 상태 저장 로드밸런서
    * NAT44, NAT66, NAT64를 지원하며 One-ARM, FullNAT, DSR 등 다양한 모드 제공
    * TCP, UDP, SCTP(멀티-호밍 포함), QUIC, FTP, TFTP 등 지원
- Hiteless/maglev/cgnat 클러스터링을 위한 BFD 감지로 고가용성 지원
- 클라우드 네이티브 환경을 위한 광범위하고 확장 가능한 엔드포인트 라이브니스 프로브
- 상태 저장 방화벽 및 IPSEC/Wireguard 지원
- [Conntrack](https://thermalcircle.de/doku.php?id=blog:linux:connection_tracking_1_modules_and_hooks), QoS 등 기능의 최적화된 구현
- ipvs와 완전 호환(ipvs 정책 자동 상속 가능)
- 정책 지향 L7 프록시 지원 - HTTP1.0, 1.1, 2.0, 3.0   

## loxilb의 구성 요소 
- GoLang 기반의 제어 평면 구성 요소
- 확장 가능하고 효율적인 [eBPF](https://ebpf.io/) 기반 데이터 경로 구현
- 통합된 goBGP 기반 라우팅 스택
- Go로 작성된 Kubernetes 오퍼레이터 [kube-loxilb](https://github.com/loxilb-io/kube-loxilb)
- Kubernetes 인그레스 구현

## 아키텍처 고려 사항   
- [kube-loxilb와 함께하는 loxilb 모드 및 배포 이해하기](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/kube-loxilb.md)
- [loxilb와 함께하는 고가용성 이해하기](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/ha-deploy.md)

## 시작하기  
#### 클러스터 외부에서 loxilb 실행  
- [K3s : flannel & loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k3s_quick_start_flannel.md)
- [K3s : calico & loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k3s_quick_start_calico.md)
- [K3s : cilium & loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/quick_start_with_cilium.md)
- [K0s : kube-router & loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k0s_quick_start.md)
- [EKS : loxilb 외부 모드](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/eks-external.md)

#### 클러스터 내에서 loxilb 실행   
- [K3s : loxilb 인-클러스터](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k3s_quick_start_incluster.md)
- [K0s : loxilb 인-클러스터](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k0s_quick_start_incluster.md)
- [MicroK8s : loxilb 인-클러스터](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/microk8s_quick_start_incluster.md)
- [EKS : loxilb 인-클러스터](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/eks-incluster.md)

#### 서비스 프록시로서의 loxilb(kube-proxy 대체)
- [K3s : flannel 서비스 프록시 & loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/service-proxy-flannel.md)
- [K3s : calico 서비스 프록시 & loxilb](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/service-proxy-calico.md)

#### Kubernetes 인그레스로서의 loxilb
- [K3s: loxilb-ingress 실행 방법](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/loxilb-ingress.md)

#### 독립형 모드에서 loxilb 실행
- [독립형 모드에서 loxilb 실행](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/standalone.md)

## 고급 가이드    
- [How-To : loxilb와 함께하는 서비스 그룹 존 설정](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/service-zones.md)
- [How-To : K8s 외부의 엔드포인트에 접근하기](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/ext-ep.md)
- [How-To : loxilb를 사용한 멀티 서버 K3s HA 배포](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/k3s-multi-master.md)
- [How-To : AWS에서 멀티-AZ HA 지원과 함께 loxilb 배포](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/aws-multi-az.md)
- [How-To : ingress-nginx와 함께 loxilb 배포](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/loxilb-nginx-ingress.md)

## 배경 지식
- [eBPF란 무엇인가](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/ebpf.md)
- [k8s 서비스 - 로드 밸런서란 무엇인가](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/lb.md)
- [간단한 아키텍처](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/arch.md)
- [코드 조직](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/code.md)
- [loxilb의 eBPF 내부](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/loxilbebpf.md)
- [loxilb NAT 모드란 무엇인가](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/nat.md)
- [loxilb 로드 밸런서 알고리즘](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/lb-algo.md)
- [수동 빌드/실행 단계](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/run.md)
- [loxilb 디버깅](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/debugging.md)
- [loxicmd 커맨드 사용법](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/cmd.md)
- [loxicmd 개발자 가이드](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/cmd-dev.md)
- [loxilb API 개발자 가이드](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/api-dev.md)
- [API 참조 - loxilb 웹 API](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/api.md)
- [성능 보고서](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/perf.md)
- [개발 로드맵](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/roadmap.md)
- [기여하기](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/contribute.md)
- [시스템 요구 사항](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/requirements.md)
- [자주 묻는 질문(FAQ)](https://github.com/loxilb-io/loxilbdocs/blob/main/docs/faq.md)
- [블로그](https://www.loxilb.io/blog)
- [데모 비디오](https://www.youtube.com/@loxilb697)

## 커뮤니티 

### Slack 
loxilb 개발자 및 다른 loxilb 사용자와 채팅을 하려면 loxilb [Slack](https://www.loxilb.io/members) 채널에 가입하세요. 이곳은 loxilb에 대해 배우고, 질문을 하고, 협력작업을 하기에 좋은 장소입니다.

### 일반 토론
GitHub [토론](https://github.com/loxilb-io/loxilb/discussions)에 자유롭게 질문을 게시하세요. 문제나 버그가 발견되면 GitHub에서 [이슈](https://github.com/loxilb-io/loxilb/issues)를 제기해 주세요. loxilb 커뮤니티의 멤버들이 도와드릴 것입니다.

## CICD 워크플로우 상태

| 기능(Ubuntu20.04) | 기능(Ubuntu22.04)| 기능(RedHat9)|
|:----------|:-------------|:-------------|
| ![Docker-Multi-Arch](https://github.com/loxilb-io/loxilb/actions/workflows/docker-multiarch.yml/badge.svg)  |  [![Docker-Multi-Arch](https://github.com/loxilb-io/loxilb/actions/workflows/docker-multiarch.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/docker-multiarch.yml) |  [![SCTP-LB-Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity-rh9.yml) |
| ![simple workflow](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity.yml/badge.svg)  | [![Sanity-CI-Ubuntu-22](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity-ubuntu-22.yml) | [![Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/basic-sanity-rh9.yml) |
| [![tcp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity.yml) | [![tcp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity-ubuntu-22.yml)   | [![TCP-LB-Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/tcp-sanity-rh9.yml) | 
| [![udp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity.yml) | [![udp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity-ubuntu-22.yml) | [![UDP-LB-Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/udp-sanity-rh9.yml) |
| [![sctp-lb-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/sctp-sanity.yml)  | ![ipsec-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity-ubuntu-22.yml/badge.svg)  | [![IPsec-Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity-rh9.yml) |
| ![extlb workflow](https://github.com/loxilb-io/loxilb/actions/workflows/advanced-lb-sanity.yml/badge.svg) | ![nat66-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity-ubuntu-22.yml/badge.svg)  | [![NAT66-LB-Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity-rh9.yml) | 
| ![ipsec-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/ipsec-sanity.yml/badge.svg)   | [![Scale-Sanity-CI-Ubuntu-22](https://github.com/loxilb-io/loxilb/actions/workflows/scale-sanity-ubuntu-22.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/scale-sanity-ubuntu-22.yml) | [![Adv-LB-Sanity-CI-RH9](https://github.com/loxilb-io/loxilb/actions/workflows/advanced-lb-sanity-rh9.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/advanced-lb-sanity-rh9.yml) |
| ![scale-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/scale-sanity.yml/badge.svg)  | [![perf-CI](https://github.com/loxilb-io/loxilb/actions/workflows/perf.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/perf.yml) | | 
| [![liveness-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/liveness-sanity.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/liveness-sanity.yml)  | | |
| ![nat66-sanity-CI](https://github.com/loxilb-io/loxilb/actions/workflows/nat66-sanity.yml/badge.svg)   | | |
| [![perf-CI](https://github.com/loxilb-io/loxilb/actions/workflows/perf.yml/badge.svg)](https://github.com/loxilb-io/loxilb/actions/workflows/perf.yml)  | | |

| K3s 테스트 | K8s 클러스터 테스트 | EKS 테스트 |
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


## 📚 자세한 정보는 loxilb [웹사이트](https://www.loxilb.io)를 확인하십시오.   

[docs-shield]: https://img.shields.io/badge/info-docs-blue
[docs-url]: https://loxilb-io.github.io/loxilbdocs/
[slack=shield]: https://img.shields.io/badge/Community-Join%20Slack-blue
[slack-url]: https://www.loxilb.io/members
