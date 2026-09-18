# devops-configs-portfolio 아키텍처 문서

- 범위: 익명화된 포트폴리오 저장소의 전체 GitOps 아키텍처와 폴더·공통화 설계만 설명한다.
- 연혁: 이전 저장소는 두 갈래다. 인프라 쪽은 입사 시점의 `infra-config-portfolio`(Terraform·ECS·Terraform Cloud), 배포 쪽은 `manifest-k8s-cluster-portfolio`(EKS 서비스별 Helm·raw manifest). 둘이 현재 `devops-configs-portfolio` GitOps 모노레포로 수렴했다.
- 기준: `b100to/devops-configs-portfolio` commit `7e3427a3a422f103cc6e4bc12adf79147ddacbec`, `b100to/infra-config-portfolio` commit `83a3c408fb7be3917c0f2efba3d137f54c0fa205`의 정적 코드 스냅샷이다.
- 이유: 개인 소개, 운영 성과, 비용 수치, 장애 사례와 섞지 않고 저장소 설계 자체를 검토 가능하게 남긴다.
- 두 저장소는 나란히 운영되었고, 입사 후 약 2년간 쓴 뒤 하나로 개편했다. 합친 이유는 셋이다: 하나의 개념이 두 저장소에 걸치는 불편, LLM 에이전트가 한 컨텍스트에서 양쪽을 읽어야 하는 점, 분리를 정당화할 만큼 크지 않은 코드 규모. 상세는 정본 1-3절을 따른다.
- Terraform Cloud를 떠난 이유는 셋이다: 유료화에 따른 추가 비용, `dev`·`stage`·`prod` 환경 브랜치를 맞춰야 하는 번거로움, 환경이 `terraform.workspace` 이름 안에 숨는 구조. 상세는 정본 4절 첫 소절을 따른다.
- Terramate 선택의 우선 기준은 팀 가독성·온보딩·단순성이며, DRY와 확장성, 지속적인 업데이트, Terramate Cloud의 stack·drift 가시성이 판단을 뒷받침했다. 상세 비교는 정본 4절을 따른다.
- 정본: [GitOps 저장소 아키텍처와 공통화 설계](../docs/devops-configs-architecture.md)
