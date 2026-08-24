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
  `clusters/kind/kind-config.yaml` 에서 두 가지로 해결했다.
    1. etcd dataDir 을 `/tmp/etcd` 로. kind 노드는 `/tmp` 가 tmpfs 라 RAM 에 쓴다
       → **컨트롤플레인 컨테이너를 재시작하면 클러스터 상태가 날아간다.**
       레포에서 전부 재적용하므로 감수한다
    2. 컨트롤플레인이 하나뿐이라 leader election 을 끈다 (`leader-elect: false`)
  적용 후 etcd 지연 경고 96회 → 1회, 노드 Ready 까지 7분 → 1분.
- **kind 의 kubeadm 설정은 `v1beta3`** 이다. `extraArgs` 를 v1beta4 의 리스트
  형식으로 쓰면 `cannot unmarshal array into Go struct field` 로 실패한다. 맵으로 쓸 것.

## 보안 — public 레포다

- **회사 정보 일절 금지**: 내부 도메인, 계정 ID, 아키텍처 세부, 비용 수치
- 시크릿 커밋 차단: `git config core.hooksPath .githooks` (gitleaks pre-commit)
- 커밋 전 `make scan`

## 문서 언어

- `README.md` — 영문
- 이 파일 및 내부 메모 — 한국어
