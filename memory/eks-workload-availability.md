# EKS 워크로드 분산 설계 문서

- 범위: 노드·AZ 이탈 시 서비스가 유지되도록 한 네 층 설계(TopologySpreadConstraints, Descheduler, PDB, Karpenter NodePool)만 설명한다.
- 계기: pod이 노드 한쪽에 몰려 있어 노드 장애나 AZ 문제 때 가용성이 확보되지 않았고, 간헐적 장애로 이어졌다.
- 기준: 현재 비공개인 원본 구성의 검토 기록이다. 원본 SHA나 비공개 저장소 링크를 공개 근거로 노출하지 않는다.
- 재현: `scripts/topology-spread-repro.sh`. `nodeTaintsPolicy` 의 `Ignore`/`Honor` 차이 세 가지를 보여준다. Descheduler evict는 재현하지 않았다.
- 분리 이유: 저장소 구조 문서(`docs/devops-configs-architecture.md`)는 장애 사례와 섞지 않는다는 범위를 유지한다.
- 정본: [노드 한 대가 빠져도 서비스가 남도록](../docs/eks-workload-availability.md)

- 공개 참조 전환: [platform-engineering-examples](https://github.com/b100to/platform-engineering-examples)의 최소 예제로 설계 패턴을 설명한다. 원본 구성 통계와 운영 성과는 사례 기록이며 예제 크기·실행 결과로 증명하지 않는다. 상세 문서의 상단 범위 안내를 정본으로 따른다.
