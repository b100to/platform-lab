# platform-lab

## 포트폴리오 모노레포

- 이 저장소는 이직용 프로젝트를 함께 관리하는 모노레포다. 새 프로젝트를 별도 저장소로 분리하지 않는다.
- `idle-reaper`는 이 저장소의 주력 Kubernetes 플랫폼 프로젝트로 유지한다. 기존 `operators/`, `clusters/`, `platform/`, `tools/`의 역할 기반 구조를 불필요하게 이동하지 않는다.
- 독립 실행 가능한 포트폴리오 앱은 `apps/<project-name>/`에 둔다. Todo 앱은 `apps/todo/`에 둔다.
- `apps/testbed/`는 `idle-reaper` 동작 검증용 Kubernetes 워크로드이고, 각 독립 앱은 하위 디렉터리의 `README.md`에 목적과 실행 방법을 명확히 적어 역할을 구분한다.
- 앱별 언어·의존성·실행 환경은 해당 앱 디렉터리 안에 격리한다. 공유가 검증되기 전에는 루트 공통 모듈이나 과도한 추상화를 만들지 않는다.
- Todo는 먼저 `apps/todo/web/`에 React, Tailwind CSS, shadcn/ui로 UI를 만든다. Go 백엔드는 학습을 시작하는 시점에 `apps/todo/api/`로 추가한다.

## 문서의 정본

- `docs/devlog.md`에는 시간순 발견과 작업 기록을 남긴다.
- `operators/idle-reaper/DESIGN.md`는 idle-reaper 설계 결정과 트레이드오프의 정본이다. 같은 내용을 `docs/decisions.md`에 복제하지 않는다.
- 재사용할 가치가 있는 장애 분석만 `docs/troubleshooting.md`에 남긴다. 일상 작업을 인위적으로 문서화하지 않는다.
