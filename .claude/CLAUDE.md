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
- **kind 4노드 동시 조인 시** etcd write latency가 300ms까지 튀고
  kube-scheduler가 CrashLoopBackOff에 빠진다. 일시적이고 저절로 수렴하니
  노드가 NotReady로 몇 분 머물러도 재생성하지 말 것.

## 보안 — public 레포다

- **회사 정보 일절 금지**: 내부 도메인, 계정 ID, 아키텍처 세부, 비용 수치
- 시크릿 커밋 차단: `git config core.hooksPath .githooks` (gitleaks pre-commit)
- 커밋 전 `make scan`

## 문서 언어

- `README.md` — 영문
- 이 파일 및 내부 메모 — 한국어
