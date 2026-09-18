# 개발 로그

이 랩에서 겪은 문제와 내린 결정을 남긴다. 성공한 결과만이 아니라
**실패한 접근과 그 이유**도 지우지 않는다.

작성 트리거·형식은 `.claude/CLAUDE.md` 의 「개발 로그」 절 참조.
최신 항목이 위로 온다.

---

## etcd 를 tmpfs 에 올린 대가 — 컨테이너 재시작 한 번에 클러스터가 사라졌다  (2026-09-01)

### Problem

로컬 4노드 kind 랩이 통째로 못 쓰게 됐다. `kubectl` 이 전부 이렇게 답했다.

```
Error from server (Forbidden): nodes is forbidden:
User "kubernetes-admin" cannot list resource "nodes" in API group "" at the cluster scope
```

kubeconfig 도 인증서도 건드린 적이 없었다.

### Expected

컨테이너 런타임을 재시작해도 클러스터는 그대로 돌아온다. 설령 상태가 날아가도
레포에서 재적용하면 복구된다 — 클러스터 설정 주석에 그렇게 적어놨었다.

### Cause

kind config 가 etcd dataDir 을 `/tmp/etcd` 로 보내고 있었고, kind 노드에서 `/tmp`
는 tmpfs 다. 컨테이너 런타임이 재시작하면서 노드 컨테이너가 같이 재시작됐고,
그 순간 etcd 데이터가 전부 사라졌다.

확인은 데이터 디렉터리 타임스탬프로 했다. 클러스터는 7일 전에 만들었는데
member 디렉터리는 그날 아침에 새로 생겨 있었다.

```
$ docker exec <control-plane> ls -la /tmp/etcd/member
drwx------ 2 root root 60 Sep  1 07:46 snap
drwx------ 2 root root 80 Sep  1 07:46 wal
```

Forbidden 은 인증 문제가 아니었다. `auth whoami` 를 보면 신원은 멀쩡하다.

```
Username   kubernetes-admin
Groups     [kubeadm:cluster-admins system:authenticated]
```

apiserver 는 기본 RBAC 롤을 부팅 때 스스로 다시 만든다. 하지만 `kubernetes-admin`
을 cluster-admin 에 이어주는 ClusterRoleBinding `kubeadm:cluster-admins` 는
kubeadm 이 클러스터 생성 시 **etcd 에 오브젝트로** 넣은 것이라 부트스트랩 대상이
아니다. 그래서 그룹은 있는데 그 그룹에 걸린 권한이 없었다.

권한을 되살리고 나니 손실이 한 겹이 아니었다.

| 잃은 것 | 어떻게 드러났나 |
|---|---|
| ClusterRoleBinding `kubeadm:cluster-admins` | 모든 kubectl 이 Forbidden |
| kindnet / kube-proxy / coredns | 컨트롤러가 `10.96.0.1:443` 타임아웃 |
| control-plane 라벨·NoSchedule taint | 워크로드가 전부 컨트롤플레인으로 몰림 |

특히 두 번째가 고약했다. `kubectl get pods` 는 전부 Running 이었다. CNI **설정
파일**은 노드 디스크에 남아 있어서 파드가 IP 를 받고 뜨기 때문이다. 죽은 건
kindnet 데몬이 프로그래밍하는 노드 간 라우팅과, kube-proxy 가 프로그래밍하는
ClusterIP 의 iptables 룰이었다. 그래서 증상이 "파드가 안 뜬다"가 아니라
"파드는 떴는데 서비스 IP 로 아무것도 안 간다"로 나왔다.

세 번째는 kubeadm 의 mark-control-plane 단계가 Node 오브젝트에 직접 쓰는 것이라,
kubelet 이 재등록해도 안 붙는다. kubelet 이 자기에게 붙이는 것만 살아남았다.

```
$ docker exec <control-plane> cat /var/lib/kubelet/kubeadm-flags.env
KUBELET_KUBEADM_ARGS="--node-ip=... --node-labels=ingress-ready=true --provider-id=..."
```

zone·role 라벨과 infra 노드 taint 는 kind config 의 nodeRegistration 으로 걸어둔
것이라 무사했다. 반대로 컨트롤플레인 것만 사라진 이유가 여기 있다.

### Attempts

**1. 바인딩만 복구.** 컨트롤플레인 안의 `super-admin.conf` 는 `system:masters`
그룹이고 이 그룹은 apiserver 가 부팅 때 바인딩까지 만들어준다. 그걸로 들어가서
없어진 바인딩을 다시 만들었다.

```
kubectl --kubeconfig /etc/kubernetes/super-admin.conf \
  create clusterrolebinding kubeadm:cluster-admins \
  --clusterrole=cluster-admin --group=kubeadm:cluster-admins
```

kubectl 은 살아났지만 클러스터는 여전히 고장난 상태였다.

**2. 레포에서 재적용.** 오퍼레이터와 테스트 워크로드를 다시 깔았다. 파드는 떴는데
컨트롤러가 apiserver 를 못 찾았다.

```
"Error retrieving lease lock" err="Get \"https://10.96.0.1:443/...\": context deadline exceeded"
```

여기서 `kubectl get ds -n kube-system` 이 `No resources found` 를 뱉었다.
**애드온은 레포에 없다.** kind 가 클러스터 생성 시에만 넣는다. 이 접근으로는
복구가 불가능하다는 게 이 시점에 확정됐다.

**3. 클러스터 재생성.** `make down && make up` 으로 정상화. 즉시 복구는 됐지만
런타임 재시작마다 반복될 문제라 원인을 그대로 두는 셈이었다.

### Decision

etcd 를 디스크로 되돌렸다. kind config 에서 dataDir 오버라이드를 삭제.

### Why

tmpfs 는 원래 워크어라운드 **두 개 중 하나**였다. 증상은 macOS 컨테이너
파일시스템에서 etcd fsync 가 200ms대(스파이크 700ms+)로 느려 lease 갱신이
타임아웃 나고 kube-controller-manager 가 `leaderelection lost` 로
CrashLoopBackOff 에 빠지는 것이었다. 대응이 (1) etcd 를 tmpfs 로, (2) 단일
컨트롤플레인이니 leader election 끄기, 둘이었다.

그런데 (2)는 그 실패 모드를 **직접** 제거한다. lease 자체를 안 쓰게 만드니까.
그렇다면 (1)은 무엇을 사고 있었나? 측정해봤다.

| | tmpfs | 디스크 |
|---|---|---|
| etcd slow-fsync / "took too long" 경고 | 0 | 0 |
| 컴포넌트 재시작 | 0 | 0 |
| 노드 전부 Ready 까지 | ~30s | ~30s |
| 노드 컨테이너 4개 재시작 후 | 클러스터 전멸 | 전부 보존 |

배포 한 사이클을 포함해 같은 조건으로 비교했다. 디스크 etcd 는 경고를 한 줄도
남기지 않았다. tmpfs 는 **아무것도 사주지 않으면서** 재시작마다 클러스터를
받아가고 있었다.

원래 주석은 이 trade-off 를 "레포에서 전부 재적용하므로 감수한다"고 적었는데,
그 전제가 틀렸다. 레포가 재적용할 수 없는 것(애드온, RBAC 바인딩, kubeadm 이
Node 에 직접 쓰는 라벨·taint)이 같이 죽기 때문이다. 감수 가능한 손실이라고
판단했던 근거 자체가 성립하지 않았다.

디스크로 되돌리는 쪽의 리스크는 fsync 지연이 다시 나타나는 것인데, 그건
`leader-elect: false` 가 이미 막고 있고 재현되지 않았다. 나타나더라도 이제는
crashloop 이 아니라 apiserver 지연 경고로 보일 것이다 — 훨씬 진단하기 쉬운 형태다.

### Result

노드 컨테이너 4개를 동시에 재시작해 런타임 재시작을 흉내냈다. API 는 10초 안에
돌아왔고 전부 살아남았다.

```
RBAC:            yes
control-plane:   node-role.kubernetes.io/control-plane (taint 유지)
addons:          kindnet 4/4, kube-proxy 4/4
helm:            idle-reaper deployed
CRD:             idlewindows, wakerequests
pods:            all Running
etcd 경고:       0
```

컴포넌트 RESTARTS 는 전부 `1` — docker restart 그 자체이고 crashloop 이 아니다.
컨트롤러 로그의 에러도 재시작 순간 apiserver 부재로 난 1줄뿐이었다.
워크로드는 app 노드 두 개에 5/4 로 분산됐다. 컨트롤플레인에는 하나도 없다.

### What I Learned

**워크어라운드를 두 개 겹쳐 넣으면 어느 쪽이 들었는지 모른 채로 둘 다 유지하게
된다.** 증상이 사라지면 검증이 끝난 것처럼 느껴지지만, 실제로는 효과 없는 쪽의
비용을 계속 내게 된다. 여기서는 그 비용이 "재시작마다 클러스터 소멸"이었다.
한 번에 하나씩 넣고 각각을 측정했어야 했다.

**"레포에서 재적용하면 된다"는 재적용 대상 목록을 실제로 세어보기 전에는 주장이
아니라 희망이다.** 클러스터 생성 시에만 쓰이는 오브젝트 — CNI, kube-proxy, DNS,
kubeadm 이 만든 RBAC 바인딩 — 는 어떤 GitOps 레포에도 들어있지 않다.

**Forbidden 이 항상 인증·인가 설정 문제인 건 아니다.** 이번엔 데이터 유실의
첫 번째 증상이었다. `auth whoami` 로 신원이 멀쩡한 걸 먼저 확인하면 "권한을 주는
오브젝트가 사라졌다" 쪽으로 방향이 잡힌다.

**Running 은 건강하다는 뜻이 아니다.** CNI 설정 파일과 CNI 데몬은 수명이 다르다.
파일만 남아도 파드는 IP 를 받고 Running 이 된다. 노드 간 통신과 ClusterIP 는
그와 별개로 죽어 있을 수 있다.

---

## 강제 분산(`DoNotSchedule`)은 장애 중 복구를 막을 수 있다 — `nodeTaintsPolicy` 를 재현으로 확인  (2026-09-18)

### Problem

노드 장애에 대비해 같은 앱의 replica 를 노드별로 강제 분산(hostname,
`whenUnsatisfiable: DoNotSchedule`)하는 설계를 문서로 정리하던 중이었다.
`nodeTaintsPolicy: Honor` 가 필요한 이유는 "replica 3 부터 Pending 이 난다"는
한 가지로만 알고 있었다. 그런데 같은 논리를 따라가면 **노드가 죽어 있는 동안**에도
같은 일이 벌어져야 했다. 문서에 쓰기 전에 실제로 그런지 확인이 필요했다.

### Expected

NotReady 노드에는 `node.kubernetes.io/not-ready:NoSchedule` taint 가 붙는다.
기본값 `Ignore` 는 taint 노드도 분산 도메인으로 센다. 그렇다면 죽은 노드가
"0개짜리 도메인"으로 남아, 살아 있는 노드에 대체 pod 을 올리는 것이
`maxSkew` 위반이 되어야 한다.

### Cause

스케줄러는 분산 도메인을 셀 때 "이 pod 이 갈 수 있는 노드인가"를
`nodeTaintsPolicy` 로만 판단한다. 이 lab 은 taint 걸린 infra 노드 1대 + app 노드
2대라, 전용 NodePool 에 taint 를 거는 운영 클러스터와 같은 모양이다.
`scripts/topology-spread-repro.sh` 로 `nodeTaintsPolicy` 만 다른 Deployment 두 개를
나란히 돌렸다.

| 실험 | `Ignore` (기본값) | `Honor` |
|---|---|---|
| A. replica 3, app 노드 2대 | 1개 Pending | 2:1 |
| B-1. replica 2, 한 노드를 unschedulable 로 만들고 그 pod 삭제 | 대체 pod Pending | 생존 노드에 Running |
| B-2. 노드 복귀 | 1:1 | **2:0 그대로** |

A 의 Pending 메시지:
`2 node(s) didn't match pod topology spread constraints, 2 node(s) had untolerated taint(s)`

### Attempts

- 실제 노드 정지(`make kill-node`) 대신 tolerate 하지 않는 `NoSchedule` taint 로
  대체했다. 스케줄러가 보는 조건은 같고, 다른 lab 워크로드를 건드리지 않으며,
  실패해도 `trap` 으로 taint 를 지우면 끝난다. NotReady 의 `NoExecute` 쪽
  (300초 뒤 eviction)은 이 실험의 질문이 아니라서 pod 을 직접 삭제했다.
- 처음엔 명령을 셸 변수(`K="kubectl --context ..."`)에 담았다가 zsh 가 공백을
  쪼개지 않아 전부 `command not found` 가 났다. 함수로 바꾸니 `k` alias 와 충돌했다.
  bash 스크립트 파일로 옮겨서 해결했다.
- 파일 개수를 `ls` 로 세었다가 `ls` 가 eza alias 라 출력이 파일명으로 파싱되지
  않았다. 카운트가 전부 0 으로 나왔고, `find` 로 다시 셌다.

### Decision

문서에는 `Honor` 의 효과를 둘로 나눠 적었다: 유령 도메인 제거, 장애 중 복구 허용.
그리고 B-2 를 Descheduler 가 필요한 근거로 연결했다.

### Why

B-1 과 B-2 는 한 쌍이다. `Honor` 는 장애 중 용량을 지키는 대신 복귀 뒤 쏠림을
남긴다. TSC 는 스케줄 시점에만 작동하므로 아무도 그 pod 을 옮기지 않는다.
"TSC 를 강제로 걸었으니 Descheduler 는 없어도 된다"도, "Descheduler 가 있으니
`Honor` 는 없어도 된다"도 틀렸다는 것을 같은 실험이 보여준다.

### Result

세 실험 모두 예상대로 재현됐다. 스크립트는 종료 시 taint 와 namespace 를 지우며,
실행 뒤 `lab-worker3` 의 taint 가 비어 있는 것을 확인했다.
Descheduler 의 evict 동작은 재현하지 않았다.

### What I Learned

**가용성을 위한 강제 제약은 장애 상황에서 거꾸로 작동하는지 따로 확인해야 한다.**
`DoNotSchedule` 은 평시의 쏠림을 막지만, `Ignore` 와 만나면 장애 중에는
"분산할 수 없으니 띄우지 않는다"가 된다. 평시 테스트로는 보이지 않는다.

**replica 2 로 하는 테스트는 분산 설정을 검증하지 못한다.** 1:1 에서 멈추므로
유령 도메인이 있어도 skew 가 1 을 넘지 않는다. 3 부터 드러난다.

**출력을 파싱하기 전에 그 명령이 alias 인지 본다.** 0 이라는 숫자는 에러처럼
보이지 않는다.

---
