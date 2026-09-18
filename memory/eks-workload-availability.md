# EKS 워크로드 분산 설계 문서

- 범위: 노드·AZ 이탈 시 서비스가 유지되도록 한 네 층 설계(TopologySpreadConstraints, Descheduler, PDB, Karpenter NodePool)만 설명한다.
- 계기: pod이 노드 한쪽에 몰려 있어 노드 장애나 AZ 문제 때 가용성이 확보되지 않았고, 간헐적 장애로 이어졌다.
- 기준: `b100to/devops-configs-portfolio` commit `7e3427a3a422f103cc6e4bc12adf79147ddacbec`의 정적 스냅샷 + 이 저장소 kind 클러스터에서의 재현.
- 재현: `scripts/topology-spread-repro.sh`. `nodeTaintsPolicy` 의 `Ignore`/`Honor` 차이 세 가지를 보여준다. Descheduler evict는 재현하지 않았다.
- 분리 이유: 저장소 구조 문서(`docs/devops-configs-architecture.md`)는 장애 사례와 섞지 않는다는 범위를 유지한다.
- 정본: [노드 한 대가 빠져도 서비스가 남도록](../docs/eks-workload-availability.md)
