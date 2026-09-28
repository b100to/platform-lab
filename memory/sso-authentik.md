# Authentik SSO 설계 문서

- 범위: 클러스터 운영 도구와 AWS 접근을 Authentik 하나로 묶은 설계 — IdP 선택 기준, Blueprint 기반 설정 관리, 앱별 연동 방식, AWS 임시 자격 증명, 겪은 함정과 한계.
- 계기: 도구마다 계정이 따로 있어 온·오프보딩이 수작업이었고, AWS는 사람 수만큼 장기 자격 증명이 있었다.
- 기준: 현재 비공개인 원본 구성의 검토 기록이다. 원본 SHA나 비공개 저장소 링크를 공개 근거로 노출하지 않는다.
- 출처: 저장소의 `docs/reference/sso.md`, `aws-cli-oidc.md`, SSO incident 문서 + 작성자의 블로그 글. 블로그는 고용주를 명시하므로 이 public 문서에서 링크하지 않았다.
- 의도적으로 뺀 것: 실제 운영 환경의 보안 약점으로 읽힐 수 있는 세부(정책 미완성 부분, 프록시 신뢰 범위 등). 운영상 교훈(Blueprint 무음 실패)은 포함했다.
- 정본: [계정 하나로 클러스터 도구와 AWS까지](../docs/sso-authentik.md)

- 공개 참조 전환: [platform-engineering-examples](https://github.com/b100to/platform-engineering-examples)의 최소 예제로 설계 패턴을 설명한다. 원본 구성 통계와 운영 성과는 사례 기록이며 예제 크기·실행 결과로 증명하지 않는다. 상세 문서의 상단 범위 안내를 정본으로 따른다.
- 공개 CLI의 범위: 발급된 ID token → STS → credential_process 응답만 구현한다. 원본의 PKCE 로그인·refresh token 갱신·캐시는 포함하지 않는다.
