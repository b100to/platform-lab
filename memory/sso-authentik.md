# Authentik SSO 설계 문서

- 범위: 클러스터 운영 도구와 AWS 접근을 Authentik 하나로 묶은 설계 — IdP 선택 기준, Blueprint 기반 설정 관리, 앱별 연동 방식, AWS 임시 자격 증명, 겪은 함정과 한계.
- 계기: 도구마다 계정이 따로 있어 온·오프보딩이 수작업이었고, AWS는 사람 수만큼 장기 자격 증명이 있었다.
- 기준: `b100to/devops-configs-portfolio` commit `7e3427a3a422f103cc6e4bc12adf79147ddacbec`의 정적 스냅샷. 로컬 재현은 하지 않았다.
- 출처: 저장소의 `docs/reference/sso.md`, `aws-cli-oidc.md`, SSO incident 문서 + 작성자의 블로그 글. 블로그는 고용주를 명시하므로 이 public 문서에서 링크하지 않았다.
- 의도적으로 뺀 것: 실제 운영 환경의 보안 약점으로 읽힐 수 있는 세부(정책 미완성 부분, 프록시 신뢰 범위 등). 운영상 교훈(Blueprint 무음 실패)은 포함했다.
- 정본: [계정 하나로 클러스터 도구와 AWS까지](../docs/sso-authentik.md)
