# platform-lab monorepo decision

## Context

사용자는 이직용 프로젝트를 `platform-lab` 하나의 모노레포에서 관리하기로 결정했다. 별도 Go Todo 저장소를 만들지 않는다. Todo는 먼저 React(Tailwind CSS + shadcn/ui) UI로 시작하고, Go 백엔드는 이후 학습 단계에서 추가한다.

## Rationale

`idle-reaper`의 플랫폼 랩과 작은 앱들을 한 곳에서 관리하면 포트폴리오의 기술 범위와 성장 과정을 함께 보여줄 수 있다. UI를 먼저 완성해 빠르게 보이는 결과를 만들고, Go API는 별도 단계로 추가해 학습 범위를 통제한다. 기존 `apps/testbed`와 독립 앱의 구분은 디렉터리명 변경보다 각 앱의 README로 명확히 한다.

## Canonical rule

실행 규칙과 정확한 경로는 [`.Codex/AGENTS.md`](../.Codex/AGENTS.md)를 정본으로 따른다.
