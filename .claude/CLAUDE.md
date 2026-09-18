# platform-lab

로컬 쿠버네티스 작업장. 4노드 kind 클러스터 + 그때그때 만들거나 시험하는 것들.

**목적을 미리 규정하지 않는다.** 필요한 게 생기면 여기 넣는다.

## 지금 하는 것

`operators/idle-reaper` — 유휴 시간 리소스 회수 오퍼레이터 (Go, kubebuilder).

## 로컬 환경

| 항목 | 값 |
|---|---|
| 컨테이너 런타임 | OrbStack |
| 클러스터 | kind 4노드 (control-plane 1 + worker 3) |
| 노드 존 라벨 | worker에 `zone-a/b/c` |
| 노드 역할 | `lab-worker`=infra(taint `dedicated=infra:NoSchedule`), `lab-worker2/3`=app |
| 호스트 포트 | **18080 / 18443** (8080·8443은 로컬에서 이미 사용 중) |
| 호스트 아키텍처 | **arm64** — 이미지 빌드 시 주의 |

## 함정 (겪은 것만 기록)

- **Go가 amd64 빌드로 깔려 있으면** 이미지가 aarch64 노드에서 안 돈다.
  `go env GOARCH` 확인할 것.
- **macOS에서 etcd가 느리다.** 컨테이너 파일시스템 탓에 fsync가 200ms대,
  스파이크는 700ms를 넘는다. 그러면 lease 갱신이 타임아웃 나고
  kube-controller-manager / kube-scheduler 가 `leaderelection lost` 로
  CrashLoopBackOff 에 빠진다. 증상은 **"Deployment 를 만들어도 파드가 안 뜬다"** —
  ReplicaSet 컨트롤러가 controller-manager 안에 있기 때문이다.
  해결은 `clusters/kind/kind-config.yaml` 의 `leader-elect: false` 하나다.
  컨트롤플레인이 하나뿐이라 lease 는 순수 오버헤드이자 실패 지점일 뿐이다.
- **etcd 를 tmpfs(`/tmp/etcd`)로 옮기는 건 하지 말 것.** 위 증상의 두 번째
  워크어라운드로 넣었다가 뺐다. 실측하면 디스크 etcd 도 slow-fsync 경고 0회,
  컴포넌트 재시작 0회 — tmpfs 는 얻는 게 없었다. 대신 **컨테이너가 재시작될
  때마다(OrbStack 재시작 포함) 클러스터가 통째로 날아간다.** 그리고 이건
  "레포에서 재적용하면 된다"로 감수할 수 있는 손실이 아니다. 다음 셋은 kind 가
  **클러스터 생성 시에만** etcd 에 쓰는 것들이라 `make lab-install` 로 못 되돌린다:
    1. ClusterRoleBinding `kubeadm:cluster-admins` → 모든 kubectl 이 `Forbidden`.
       인증 문제처럼 보이지만 아니다. 복구는 컨트롤플레인 안에서
       `kubectl --kubeconfig /etc/kubernetes/super-admin.conf` 로 바인딩 재생성
    2. kindnet / kube-proxy / coredns → ClusterIP·DNS·노드간 통신 사망.
       CNI 설정 파일은 노드 디스크에 남아서 **파드는 IP 를 받고 Running 으로
       보인다.** 증상은 컨트롤러가 `10.96.0.1:443` 타임아웃 나는 것으로 나타난다
    3. control-plane 의 `node-role.kubernetes.io/control-plane` 라벨과 NoSchedule
       taint → 워크로드가 컨트롤플레인으로 몰린다. kubeadm 의 mark-control-plane
       단계가 Node 오브젝트에 직접 쓰는 것이라 kubelet 재등록으로는 안 붙는다
  이 상태가 되면 손으로 깁지 말고 `make down && make up` 후 재배포한다.
- **kind config 의 `labels:` 로 `node-role.kubernetes.io/*` 를 붙이면 노드가 안 뜬다.**
  이 라벨은 kubelet 이 등록 시 자기에게 붙이는데, NodeRestriction admission 이
  그 접두사를 거부한다. 등록이 실패하면 join 이
  `could not find a JWS signature in the cluster-info ConfigMap` 이라는
  엉뚱한 메시지로 죽어서 원인을 찾기 어렵다. 커스텀 도메인
  (`platform-lab.dev/role`) 을 쓰거나 생성 후 kubectl 로 붙일 것.
  taint 는 `JoinConfiguration.nodeRegistration.taints` 로 문제없이 걸린다.
- **kind 의 kubeadm 설정은 `v1beta3`** 이다. `extraArgs` 를 v1beta4 의 리스트
  형식으로 쓰면 `cannot unmarshal array into Go struct field` 로 실패한다. 맵으로 쓸 것.

## Helm 차트 관련 (겪은 것)

- **`make manifests` 는 `dist/chart` 를 갱신하지 않는다.** RBAC 마커를 고쳐도
  차트는 옛 권한을 그대로 담고 있고, 증상은 클러스터에서만 나타난다 —
  컨트롤러는 뜨는데 권한 없는 리소스를 watch 하지 못한다.
  `make lab-chart` 로 재생성한다 (`lab-deploy` 가 자동 호출).
- **`make install`(kustomize)로 CRD 를 먼저 깔면 Helm 이 그걸 인수하지 못한다.**
  `invalid ownership metadata` 로 upgrade 가 실패한다. CRD 를 지우면 그 안의
  오브젝트도 사라지므로, 라벨·어노테이션을 붙여 입양시키는 편이 낫다.
  ```
  kubectl label crd <name> app.kubernetes.io/managed-by=Helm --overwrite
  kubectl annotate crd <name> meta.helm.sh/release-name=idle-reaper \
    meta.helm.sh/release-namespace=idle-reaper-system --overwrite
  ```
- **차트 재생성은 Chart.yaml 을 템플릿 문구로 되돌린다.** `lab-chart` 가 백업·복원한다.

## 보안 — public 레포다

- **회사 정보 일절 금지**: 내부 도메인, 계정 ID, 아키텍처 세부, 비용 수치
- 시크릿 커밋 차단: `git config core.hooksPath .githooks` (gitleaks pre-commit)
- 커밋 전 `make scan`

## 개발 로그

문제·의사결정은 [`docs/devlog.md`](../docs/devlog.md) 에 기록한다.
트리거·형식·원칙은 글로벌 `~/.claude/CLAUDE.md` §8 (개발 로그) 참조 — 여기 복제하지 않는다.

이 레포는 public 이고 포트폴리오로 쓴다. devlog 도 **회사에 종속되지 않는 일반 기술 문제**로 서술한다.

## 문서 언어

- `README.md` — 영문
- 이 파일 및 내부 메모 — 한국어
- `docs/devlog.md` — 한국어
