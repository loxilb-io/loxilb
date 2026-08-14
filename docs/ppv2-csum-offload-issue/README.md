# PROXY protocol v2 + 체크섬 오프로드 잔여 결함 (Issue #675 후속)

> **목적**: 이 문서는 다른 AI 에이전트/개발자가 후속 수정을 바로 착수할 수 있도록,
> #675 수정 과정에서 **의도적으로 남긴 잔여 결함 1건**의 위치·메커니즘·도달 조건·
> 심각도 판정·수정안·검증 레시피를 자기완결적으로 정리한 것이다.
> 모든 파일/라인 참조는 verbatim이며, **확정 사실**과 **가설**을 명확히 구분한다.

- **작성일**: 2026-08-14
- **분석 대상 커밋**: `loxilb-ebpf` `d904e01` (브랜치 `fix/ppv2-csum-offload`)
- **관련 이슈**: [Issue #675](https://github.com/loxilb-io/loxilb/issues/675) (Support for proxy protocol v2)
- **선행 수정**: loxilb-ebpf [#87](https://github.com/loxilb-io/loxilb-ebpf/pull/87) (GSO), [#91](https://github.com/loxilb-io/loxilb-ebpf/pull/91) (`doff == 20`), [#92](https://github.com/loxilb-io/loxilb-ebpf/pull/92) (삽입 실패 시 drop), `fix/ppv2-csum-offload` (체크섬 오프로드)
- **영향 버전**: v0.9.8.8 이하 전부 (이 결함 자체는 ppv2 도입 이후 계속 존재)

### 관련 문서
- [`../tls13-proxyproto-v2-issue/`](../tls13-proxyproto-v2-issue/) — GSO 슈퍼패킷 손상(#1044/#1089) 분석. 같은 헬퍼(`dp_ins_ppv2`)의 선행 결함.
- [`../../cicd/tlsproxyprotov2/`](../../cicd/tlsproxyprotov2/) — 재현/회귀 하니스. `CONTROL-1/2`, `REPRO`(GSO), `REPRO-2`(`doff==20`), `REPRO-3`(체크섬 오프로드).

---

## 0. TL;DR

1. **잔여 결함**: ppv2 헤더 삽입 트리거가 된 TCP 세그먼트가 **페이로드를 가진 경우**,
   `dp_ins_ppv2()`의 `bpf_skb_adjust_room()` 경로를 타고 **잘못된 L4 체크섬**을 내보낸다.
   단, **인그레스 skb가 `CHECKSUM_PARTIAL`일 때만**.
2. **치명적이지 않다**. 정상 경로(순수 핸드셰이크 ACK)는 이미 수정됐고, 잔여 경로는
   *드문 트리거* × *특정 환경*의 교집합에서만 발생하며, 증상은 **해당 연결 1개의 정지**다.
   시스템 전체 장애나 데이터 손상 성격이 아니다. → §4
3. **그러나 후속으로 반드시 처리해야 한다**. 증상이 "간헐적 · 환경 의존적 · 재현 불가"라서,
   제보가 올라오면 #675와 똑같이 처음부터 추적하게 된다. → §4.3
4. **수정 방향**: 삽입 후 **L4 전 구간을 프로그램에서 합산해 완전한 체크섬을 처음부터 계산**한다.
   인그레스 체크섬 상태와 무관하게 정확해진다. → §6
5. **UDP는 무관**. `dp_ct_udp_sm()`은 `xf->pm.ppv2`를 전혀 설정하지 않으므로
   `dp_ins_ppv2()`의 UDP 분기는 **현재 죽은 코드**다. → §3.3 (확정 사실)

---

## 1. 배경 — #675에서 밝혀진 결함 3건

`--ppv2en`(fullnat + PROXY protocol v2)에는 서로 **독립적인** 결함이 3건 있었다.

| # | 결함 | 트리거 | 상태 |
|---|------|--------|------|
| 1 | GSO 슈퍼패킷에 인라인 삽입 시 바이트 스트림 손상 | 첫 데이터 패킷이 `gso_size>0` | **수정됨** (loxilb-ebpf#87) |
| 2 | `doff == 20`(TCP 옵션 없음)에서 PPv2 헤더를 아예 쓰지 않음 | Windows 10/11 (timestamps 기본 off) | **수정됨** (loxilb-ebpf#91) |
| 3 | L4 체크섬을 손으로 유지 → 오프로드 경로에서 무효 | 인그레스가 `CHECKSUM_PARTIAL` | **부분 수정** (`fix/ppv2-csum-offload`) |

결함 3이 "`--ppv2en`은 오프로드를 꺼야만 동작한다"는 우회법의 정체였다. 이 문서는 그 **부분 수정에서 남은 절반**을 다룬다.

### 1.1 결함 3의 두 원인 (둘 다 확정 사실, 실측)

- **(a) `dp_fixup_ppv2()`의 이중 계산** — `seq`/`ack_seq`는 L4 체크섬 커버리지 **안**에 있어
  하류 확정 패스(`skb_checksum_help()` 또는 NIC)가 이미 반영한다. 여기에 `tcp->check`까지
  손으로 같은 델타를 접어 넣어 **28바이트가 두 번** 계산됐다.
  → `bpf_l4_csum_replace()`로 교체. **완전 해결.**
- **(b) `dp_ins_ppv2()`의 `CHECKSUM_NONE` 리셋** — `bpf_skb_adjust_room()`은
  **기본적으로 오프로드 체크섬 표시를 `CHECKSUM_NONE`으로 리셋**한다
  (UAPI 문서: *"By default, the helper will reset any offloaded checksum indicator of the skb
  to CHECKSUM_NONE"*, `BPF_F_ADJ_ROOM_NO_CSUM_RESET` 설명). 그 뒤 TCP 헤더를 room 위로
  밀어 내린다. 결과적으로 `tcp->check`에 들어 있던 **pseudo 헤더 부분합이 "완성된 체크섬"인 양** 나간다.
  → 페이로드 없는 트리거 세그먼트에 한해 `bpf_skb_change_tail()` 경로(`dp_ins_ppv2_tail()`) 신설. **부분 해결.**

---

## 2. 잔여 결함의 정확한 위치

### 2.1 분기 게이트 — `kernel/llb_kern_cdefs.h:1282-1293` (`d904e01`)

```c
  if (is_tcp) {
    dend = DP_TC_PTR(DP_PDATA_END(md));
    tcp = DP_ADD_PTR(DP_PDATA(md), xf->pm.l4_off);
    if (tcp + 1 > dend) {
      LLBS_PPLN_DROPC(xf, LLB_PIPE_RC_PLERR);
      return -1;
    }
    doff = tcp->doff << 2;
    if (doff >= sizeof(*tcp) && xf->pm.l3_plen == doff) {
      return dp_ins_ppv2_tail(md, xf, len, doff);      /* 안전 경로 */
    }
  }
```

`xf->pm.l3_plen`은 IP 페이로드 길이(= TCP 헤더 + TCP 페이로드)다.
`xf->pm.l3_plen == doff` ⟺ **TCP 페이로드 0바이트**.

- **참(페이로드 없음)** → `dp_ins_ppv2_tail()` (`cdefs.h:1183`). 꼬리에서 확장하므로 TCP 헤더가
  움직이지 않고 `csum_start`가 유효하게 남아 스택이 체크섬을 정상 확정한다. **정상.**
- **거짓(페이로드 있음)** → 아래로 흘러 `dp_buf_add_room3()`(= `bpf_skb_adjust_room`,
  `cdefs.h:1300`) 경로. **잔여 결함 구간.**

### 2.2 왜 `adjust_room` 경로는 고칠 수 없었나

두 가지가 겹친다.

1. **`CHECKSUM_NONE` 리셋** — 헬퍼 호출 시점에 skb의 오프로드 표시가 사라진다.
   이후 아무도 체크섬을 확정해 주지 않으므로, 패킷 안의 필드는 **완전한 체크섬**이어야 한다.
   그런데 인그레스가 `CHECKSUM_PARTIAL`이었다면 그 필드엔 pseudo 부분합만 들어 있다.
   증분 갱신을 아무리 정확히 해도 "부분합 + 델타"일 뿐 완전한 값이 되지 않는다.
2. **`csum_start` 무효화** — `BPF_F_ADJ_ROOM_NO_CSUM_RESET`으로 `CHECKSUM_PARTIAL`을
   유지시켜도 소용없다. `BPF_ADJ_ROOM_NET`은 room을 **L4 헤더 앞**에 열고, loxilb는 그 위로
   TCP 헤더를 28바이트 앞으로 memmove한다. `csum_start`(head 기준 절대 위치)는 그대로이므로
   **이동한 TCP 헤더보다 28바이트 뒤**, 즉 삽입된 PPv2 헤더 한가운데를 가리키게 된다.
   그 상태로 확정되면 체크섬이 PPv2 헤더 바이트를 덮어써 헤더 자체가 깨진다.
   → **`NO_CSUM_RESET`은 해법이 아니다** (가설이 아니라, 구조상 확정).

즉 남은 유일한 정답은 **프로그램이 L4 전 구간을 직접 합산**하는 것이다(§6).

---

## 3. 도달 가능성 분석

### 3.1 `xf->pm.ppv2`를 세우는 지점은 두 곳뿐 (확정 사실)

```
$ grep -rn "pm.ppv2 = 1" kernel/
kernel/llb_kern_ct.c:467      # case CT_TCP_SA  (핸드셰이크 ACK)
kernel/llb_kern_ct.c:489      # case CT_TCP_PEST (백스톱)
```

둘 다 `dp_ct_tcp_sm()` 안에 있다. `dp_ins_ppv2()`는 `llb_kern_devif.c:300`에서
`xf->pm.ppv2`일 때만 호출된다.

### 3.2 페이로드 있는 세그먼트가 트리거가 되는 3가지 경로

| # | 시나리오 | 경유 지점 | 현실 빈도 |
|---|----------|-----------|-----------|
| A | 클라이언트의 **순수 핸드셰이크 ACK가 유실/재정렬**되어, loxilb가 `SA` 상태에서 처음 보는 ACK가 ClientHello인 경우 | `ct.c:467` | 낮지만 **확률적으로 반드시 발생** (인터넷 구간 패킷 손실) |
| B | 최종 ACK에 데이터가 **피기백**된 경우 | `ct.c:467` | 낮음. 주류 스택(Linux/Windows/macOS/curl/브라우저)은 순수 ACK를 별도 전송 |
| C | 최종 ACK가 GRO 병합(`is_gso`)되어 `defer_ppv2`로 드롭 → **재전송(데이터 패킷)** 이 백스톱을 탐 | `ct.c:471` → `ct.c:489` | 낮음 (#87 커밋이 "rare"로 분류) |

경로 A의 근거 — `case CT_TCP_SA` 분기는 ACK 플래그와 `ack == rtd->seq + 1`만 검사하고
**페이로드 유무를 보지 않는다**. 따라서 ACK 비트를 단 ClientHello가 그대로 트리거가 된다.

```c
  case CT_TCP_SA:
    ...
    if ((tcp_flags & LLB_TCP_ACK) != LLB_TCP_ACK) { nstate = CT_TCP_ERR; goto end; }
    if (ack != rtd->seq + 1) { nstate = CT_TCP_ERR; goto end; }
    td->seq = seq;
    if (xf->nm.ppv2) {
      nstate = CT_TCP_PEST;
      if (td->ppv2 == 0) {
        if (!is_gso) { xf->pm.ppv2 = 1; td->ppv2 = 1; rtd->ppv2 = 1; }   /* ct.c:467 */
        else { defer_ppv2 = 1; }
      }
    }
```

### 3.3 UDP는 도달 불가 (확정 사실)

`dp_ct_udp_sm()` 전체에 `ppv2` 문자열이 없다. `xf->pm.ppv2`는 TCP CT SM에서만 설정되므로
`dp_ins_ppv2()`의 UDP 분기는 **현재 죽은 코드**다.
(`fix/ppv2-csum-offload`에서 UDP 분기도 함께 정리했지만, 이는 장래 UDP ppv2가 연결될 때를
대비한 선제 수정이며 현재 동작에는 영향이 없다.)

### 3.4 환경 조건 — 인그레스가 `CHECKSUM_PARTIAL`일 때만

| loxilb로 들어오는 패킷의 출처 | `ip_summed` | 잔여 결함 발현 |
|---|---|---|
| 물리 NIC | `CHECKSUM_UNNECESSARY`/`COMPLETE` (바이트에 완전한 체크섬) | **없음** (증분 갱신이 정확) |
| veth / tap / tun 피어, 로컬 생성 | **`CHECKSUM_PARTIAL`** (필드엔 pseudo 부분합만) | **발현** |

해당되는 실제 배포 형태:
- 컨테이너에서 도는 loxilb + 같은 호스트의 백엔드 컨테이너
- k8s **in-cluster 모드**에서 백엔드 파드가 **같은 노드**에 있는 경우
- WireGuard/Tailscale 등 tun 경유 경로

---

## 4. 심각도 판정

### 4.1 터졌을 때의 증상

체크섬 불량 세그먼트를 백엔드(또는 클라이언트)의 TCP 스택이 **조용히 폐기**한다.
그리고 loxilb가 주입한 28바이트는 **어느 쪽 재전송 큐에도 없으므로 영원히 재전송되지 않는다**.
→ 해당 연결은 스트림 선두에 28바이트 구멍이 난 채 **영구 정체**하다 타임아웃된다.

관측되는 형태 (#675 스레드의 제보와 동일):
```
backend: ACK 1, SACK {29:463}   ← 누적 ACK가 전진하지 않음
client : 같은 데이터 반복 재전송
```

### 4.2 왜 치명적이지 않은가

- **연결 단위 실패**다. 클라이언트가 재시도하면 새 연결은 정상 경로(순수 핸드셰이크 ACK →
  `dp_ins_ppv2_tail()`)를 타서 성공한다.
- 정상 경로가 압도적 다수다. #87 이후 삽입 지점이 핸드셰이크 ACK로 옮겨졌기 때문에,
  §3.2의 A/B/C 예외에 걸리지 않는 한 항상 안전 경로다.
- **환경 조건과 트리거 조건의 교집합**이라 노출 면적이 더 좁아진다 (§3.4).
- 데이터 손상(잘못된 바이트가 백엔드에 전달됨)이 아니라 **전달 실패**다. 체크섬이 틀리면
  스택이 버리므로 애플리케이션이 오염된 데이터를 보는 일은 없다.

### 4.3 그럼에도 후속 처리가 필요한 이유

증상이 **간헐적 · 환경 의존적 · 재현 불가**라는 점이 문제다. 이건 #675를 1년 넘게
미해결로 만든 바로 그 성질이다. "가끔 한 연결이 멈춘다"는 제보가 올라오면 원인 후보가
너무 넓어 다시 원점에서 추적하게 된다. **알려진 채로 남겨두는 비용이 수정 비용보다 크다.**

**판정: P2 (다음 사이클에 처리). 현재 PR을 막을 사유는 아니다.**

---

## 5. 재현 방법

### 5.1 핵심 기법 — veth 환경에서 체크섬을 실제로 검증하게 만들기

veth-to-veth는 기본적으로 TCP 체크섬을 **검증하지 않는다**. `CHECKSUM_PARTIAL` skb가
pseudo 부분합만 담은 채 그대로 건너가고 피어가 그것을 신뢰한다. 그래서 이 결함군은
CI에서 구조적으로 보이지 않았다.

loxilb **자신의 veth**에서 TX 체크섬 오프로드를 끄면, 커널이 나가는 길에
`skb_checksum_help()`로 **소프트웨어 확정**을 수행한다 (tun 디바이스가 하는 일과 동일).
그때부터 잘못된 체크섬은 실제로 패킷을 잃게 만든다.

```bash
hexec="sudo ip netns exec"
$hexec llb1 ethtool -K ellb1l3h1  tx off     # 역방향(→클라이언트) 확정 강제
$hexec llb1 ethtool -K ellb1l3ep1 tx off     # 정방향(→백엔드) 확정 강제

# 수신측에서 체크섬 검증
sudo ip netns exec l3h1 tcpdump -i el3h1llb1 -nn -vv 'tcp src port 2020' | grep -c incorrect
```

이 기법은 `cicd/tlsproxyprotov2/validation.sh`의 **`REPRO-3`** 케이스로 정착돼 있다.

### 5.2 잔여 결함(페이로드 있는 트리거) 강제 방법

`REPRO-3`은 정상 경로만 검증한다. 잔여 결함을 재현하려면 §3.2의 트리거를 인위적으로 만들어야 한다.

**방법 1 (권장) — 클라이언트의 순수 핸드셰이크 ACK를 버린다** (경로 A 재현)

l3h1 netns에서 VIP로 가는 **길이 0인 순수 ACK**만 한 번 드롭시키면, loxilb가 `SA` 상태에서
처음 보는 ACK가 ClientHello가 된다.

```bash
# 예시: nftables/iptables로 payload 없는 ACK 1개만 드롭 (한 번만 매치되도록 limit/한정 필요)
# 주의: 모든 순수 ACK를 드롭하면 연결 자체가 성립하지 않는다. 3-way handshake의
#       세 번째 ACK만 선택적으로 버려야 한다.
```

**방법 2 — scapy로 핸드셰이크를 직접 구성** (경로 B 재현, 가장 결정적)

SYN → SYN-ACK 수신 → **ACK + ClientHello를 한 세그먼트로** 전송. 트리거 세그먼트가
확정적으로 페이로드를 갖게 된다.

**방법 3 (가장 간단, 코드 수정 필요) — 게이트를 무력화**

`cdefs.h:1290`의 조건을 `if (0)`로 바꿔 빌드하면 모든 삽입이 `adjust_room` 경로를 탄다.
수정 전 상태와 동일해지므로, 아래 §7의 수치가 그대로 재현된다.

### 5.3 기대 결과 (수정 전 실측값 — 확정 사실)

`fix/ppv2-csum-offload` 적용 **전**, §5.1 조건에서 측정한 값:

| 방향 | 체크섬 오류 | 결과 |
|---|---|---|
| 정방향 (삽입 + seq fixup) | **19/21 incorrect** | 연결 타임아웃 |
| 역방향 (ack fixup) | **17/18 incorrect** | 연결 타임아웃 |

오차는 전부 **정확히 `0x1c`(=28, PPv2 IPv4 헤더 길이)**. ppv2 마킹 이전인 SYN/SYN-ACK만 `(correct)`.

대조군 — 같은 loxilb·같은 경로에서 `--ppv2en`만 뺀 경우:

| | 정방향 | 역방향 |
|---|---|---|
| `fullnat` (ppv2 없음) | **0/10** | **0/7** |
| `fullnat --ppv2en` | 19/21 | 17/18 |

---

## 6. 수정안

### 6.1 방향 — 완전한 체크섬을 처음부터 계산

`adjust_room` 이후 skb는 `CHECKSUM_NONE`이다. 즉 **아무도 확정해 주지 않으므로 우리가
완전한 값을 써 넣으면 된다**. 인그레스가 `PARTIAL`이었든 `COMPLETE`였든 무관하게 정확해진다.

```
check = ~fold( pseudo_hdr_sum
              + sum(TCP 헤더, check 필드를 0으로)
              + sum(PPv2 헤더 28/52B)
              + sum(TCP 페이로드) )
```

- `pseudo_hdr_sum`: 패킷의 **현재(post-NAT)** IP 헤더에서 src/dst, proto, 새 L4 길이로 구성.
  `dp_ins_ppv2()`는 `dp_unparse_packet_always()` 뒤에 실행되므로 IP 헤더는 이미 최종 주소다.
- `sum(PPv2 헤더)`: 이미 `dp_populate_ppv2()`가 `bcsum`으로 돌려준다 (`cdefs.h:1035`).
- `sum(TCP 헤더)`: `doff` 분기 안에서 상수 크기로 계산 가능 (이미 20/24/28/32/36/40 분기가 있음).
- `sum(TCP 페이로드)`: **여기가 유일한 난제.**

### 6.2 페이로드 합산의 제약

- `bpf_csum_diff()`는 호출당 크기 상한이 있다 (커널 `bpf_scratchpad.diff` 크기 = `MAX_BPF_STACK` = **512바이트**).
  → MTU급 패킷은 3~4회 청크 루프가 필요하다. **(구현 착수 시 커널 소스로 재확인할 것 — 가설)**
- 검증기 친화적으로 쓰려면 각 청크마다 상수 크기 + 경계 검사가 필요하다.
  `#pragma unroll`로 고정 횟수 루프를 펼치는 형태가 현실적이다.
- clang-10 / 커널 5.x 타깃을 유지해야 한다 (`ghcr.io/loxilb-io/ossca-build`).

**예상 규모**: 40~60줄, 검증기와 씨름할 여지 있음.

### 6.3 대안과 그 한계 (검토 완료, 모두 기각)

| 대안 | 기각 사유 |
|---|---|
| `BPF_F_ADJ_ROOM_NO_CSUM_RESET`으로 `PARTIAL` 유지 | §2.2-(2). TCP 헤더 이동으로 `csum_start`가 PPv2 헤더 한가운데를 가리키게 됨 |
| 페이로드 있는 트리거를 `defer_ppv2`로 드롭 | 재전송도 같은 크기의 데이터 패킷 → **라이브락** |
| 삽입을 이후의 순수 ACK로 미룸 | PPv2 헤더는 스트림 **선두**여야 함. 중간 삽입은 프로토콜 위반 |
| `bpf_skb_change_tail()`로 페이로드까지 밀어내기 | 페이로드 이동에 결국 바운드 루프가 필요 → §6.1과 동일 비용 |

### 6.4 착수 시 유의점

- 수정 후 **정상 경로(`dp_ins_ppv2_tail`)를 건드리지 말 것**. 이미 검증됐다.
- 완전 계산 경로가 완성되면 `dp_ins_ppv2_tail()`과 통합할지 검토할 수 있으나,
  tail 경로는 스택에 확정을 위임하므로 더 싸다. **분리 유지를 권장.**
- `dp_ins_ppv2()`의 반환값은 `llb_kern_devif.c:300`에서 검사되어 실패 시 drop된다 (#92).
  새 경로에서도 실패 시 `-1`을 반환하면 손상 패킷이 나가지 않는다.

---

## 7. 검증 레시피

### 7.1 회귀 하니스

```bash
cd cicd/tlsproxyprotov2/
BASE_IMAGE=ghcr.io/loxilb-io/loxilb:v0.9.8.8 ./build_deploy.sh   # 워킹트리 데이터플레인 빌드+배포
./validation.sh                                                  # EXPECT=fixed (기본)
```

5개 케이스 전부 `OK`여야 한다: `CONTROL-1`, `CONTROL-2`, `REPRO`(GSO),
`REPRO-2`(`doff==20`), `REPRO-3`(체크섬 오프로드).

게이트가 실제로 판별하는지 확인하려면 체크섬 수정이 없는 데이터플레인으로 돌린다
(`REPRO-3`만 `FAIL`이어야 한다):

```bash
EXPECT=bug ./validation.sh
```

### 7.2 잔여 결함 수정의 합격 기준

§5.1 조건에서 **§5.2의 트리거를 강제한 상태로**:

| 측정 | 목표 |
|---|---|
| 정방향(삽입 패킷 포함) `incorrect` 카운트 | **0** |
| 역방향 `incorrect` 카운트 | **0** |
| 연결 성립 | 성공 |
| 완전-체크섬 인그레스(송신측 오프로드 off = 물리 NIC 상당) | **0**, 성공 |

마지막 항목이 중요하다 — 물리 NIC 경로를 **회귀시키지 않았는지** 확인하는 대조군이다.

### 7.3 수정 후 실측 참고값 (`fix/ppv2-csum-offload`, 정상 경로 기준 — 확정 사실)

| 조건 | 결과 |
|---|---|
| 정방향, 소프트웨어 확정 강제 | 0/30 incorrect, 성공 |
| 역방향, 소프트웨어 확정 강제 | 0/8 incorrect, 성공 |
| 양방향 동시 (tun 경로 상당) | 0/8, 성공 |
| 완전-체크섬 인그레스 (물리 NIC 상당) | 0/10, 성공 |

---

## 8. 확정 사실 / 가설 구분

**확정 사실 (실측 또는 소스 verbatim으로 확인)**
- `xf->pm.ppv2` 설정 지점은 `llb_kern_ct.c:467`, `:489` 두 곳뿐이다.
- `dp_ct_udp_sm()`은 ppv2를 설정하지 않는다 → UDP 분기는 죽은 코드다.
- `case CT_TCP_SA` 분기는 페이로드 유무를 검사하지 않는다.
- `bpf_skb_adjust_room()`은 기본적으로 오프로드 체크섬 표시를 `CHECKSUM_NONE`으로 리셋한다 (UAPI 문서).
- 수정 전 측정값(19/21, 17/18, 오차 `0x1c`)과 대조군(0/10, 0/7), 수정 후 측정값(§7.3).
- `bpf_l4_csum_replace()`는 skb의 체크섬 상태를 인지한다 — 이를 쓰도록 바꾸자 `PARTIAL`
  경로에서 `seq`/`ack` fixup 패킷이 전부 `(correct)`로 바뀌었다(실측). 문서에는 명시돼 있지 않다.

**가설 (착수 전 확인 필요)**
- `bpf_csum_diff()`의 호출당 상한이 512바이트(`MAX_BPF_STACK`)라는 점. 커널 소스로 재확인할 것.
- 경로 A(순수 ACK 유실)의 실제 발생 빈도. 정성적 추정일 뿐 측정치가 없다.
- 페이로드 합산 루프가 clang-10 + 커널 5.x 검증기를 통과하는지. **미검증.**

---

## 9. 참조

**코드**
- `loxilb-ebpf` `d904e01` — `kernel/llb_kern_cdefs.h`
  - `dp_fixup_ppv2()` : 1099
  - `dp_ins_ppv2_tail()` : 1183 (안전 경로)
  - `dp_ins_ppv2()` : 1242
  - 분기 게이트 : 1290
  - `dp_buf_add_room3()` 호출 : 1300 (잔여 결함 구간)
  - `dp_populate_ppv2()` : 1035 (`bcsum` 반환)
- `loxilb-ebpf` `kernel/llb_kern_ct.c` : 467(SA), 471(defer), 489(PEST 백스톱), 497(defer)
- `loxilb-ebpf` `kernel/llb_kern_devif.c` : 300 (반환값 검사)

**브랜치 / PR**
- loxilb-ebpf `fix/ppv2-csum-offload` (`d904e01`) — 체크섬 수정
- loxilb `fix/ppv2-no-tcp-options` — cicd 회귀(`REPRO-2`, `REPRO-3`) + 서브모듈 bump

**하니스**
- `cicd/tlsproxyprotov2/validation.sh` — `REPRO-3`가 §5.1 기법을 구현
- `cicd/tlsproxyprotov2/README.md` — 결함 3건의 요약과 측정 결과
