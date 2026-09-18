# Istio → Traefik 전환 문서

- 범위: 서비스 메시(Istio)를 제거하고 Traefik + AWS Load Balancer Controller 구조로 옮긴 판단, 설계, 무중단 전환 절차, 잃은 것.
- 계기: 개발자는 디버깅·구조 파악·인수인계가 어려웠고, 엔지니어는 sidecar·Istio 설정·Envoy 까지 알아야 하는 부담이 컸다. 서비스 수가 많지 않고 MSA 경계도 뚜렷하지 않았으며 비용을 줄여야 하는 시기였다.
- 핵심 근거: 공개 스냅샷의 명시적 Istio 설정은 `Gateway` 4 + `VirtualService` 4로 인그레스 중심이다. 정책 부재만으로 auto mTLS까지 미사용이었다고 단정하지 않는다. ALB도 URL rewrite·CORS 응답 헤더를 지원하므로 선택 이유는 CRD 재사용과 책임 분리로 설명한다. 상세 근거는 정본을 따른다.
- Traefik 을 고른 이유: 표준이던 ingress-nginx 의 retirement 공지(2025-11-11, 2026-03 까지 best-effort) 직후였다. 나머지는 CRD 기반 설정. 이 동기는 사용자 증언이며 저장소에는 없다.
- 기준: `b100to/manifest-k8s-cluster-portfolio` commit `ed4f0a3…`, `b100to/devops-configs-portfolio` commit `7e3427a…` 의 정적 스냅샷.
- 확인 못 한 것: 실측 리소스 절감치, 계획(Terraform ALB + NodePort)이 최종 구조(LB Controller + IP target)로 바뀐 동기. 문서에는 기술적 효과만 적었다.
- 정본: [서비스 메시를 걷어내기](../docs/istio-to-traefik.md)
