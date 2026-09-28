# 채용 제출용 포트폴리오 (md + 그림)

- 범위: 네 작업(워크로드 분산, Istio → Traefik, Authentik SSO, GitOps 재설계)을 한 문서로 묶은 제출용 요약. 상세는 각 주제 문서로 링크한다.
- 구조: 첫 화면의 표만 읽어도 전체가 전달되게 하고, 케이스는 강한 것부터 놓았다(시간순 아님). 각 케이스는 문제·판단·결과·한계를 한 줄씩 항목으로 쓴다.
- 디자인: 이력서에 붙는 문서라서 흑백 선 그림 + 포인트 색 하나(짙은 남색), 각진 모서리, 채움 최소화. 처음 시안(파스텔 패널, 주황·청록, 큰 숫자 타일)은 사용자가 "AI 티가 난다"고 반려했다. 글꼴은 Helvetica Neue + Apple SD Gothic Neo, 굵기는 600까지만.
- 그림: `docs/assets/portfolio/build.py` 가 SVG 5장을 생성하고 `rsvg-convert` 로 PNG를 만든다. 라벨마다 상자 폭 대비 글자 폭을 검사해 넘치면 빌드가 실패한다. 그림을 고칠 땐 SVG·PNG를 손대지 말고 이 스크립트를 고친다.
- 수치는 상세 문서의 정정본을 따른다: backend 선언 39개(실험용 디렉터리 제외) → 생성 규칙 1개(S3 backend stack 69개). Istio는 "명시적 선언" 기준이며 메시 기능 미사용을 단정하지 않는다. ALB 단독도 rewrite·CORS는 가능하다.
- 개인정보는 2026-09-22 사용자 답변으로 정본에 반영했다. GitOps 기간은 로컬 이력서 `/Users/jonghwabaek/works/workspace/resume/index.html`의 IaC·GitOps 항목(2024.03 – 2026.03)을 따른다.
- 제출용 PDF도 반드시 아래 정본에서 생성한다. 별도 요약 초안을 만들거나 이전 초안의 그림·사례 순서를 재사용하지 않는다. 이력서는 경력·성과 요약, 포트폴리오는 설계 판단·검증 근거로 역할을 나누고 기간·수치를 일치시킨다.
- 공개 코드는 [platform-engineering-examples](https://github.com/b100to/platform-engineering-examples) 하나에 모은다. 사용자가 운영 스냅샷 3개 공개 대신 독립된 예제 저장소를 명시적으로 선택했다. 기존 infra-config-portfolio·manifest-k8s-cluster-portfolio·devops-configs-portfolio는 비공개 원본 보관용이다. 새 앱을 platform-lab 모노레포에 두는 규칙과 별개인 공개 예제 배포 요청이다.
- 공개 예제는 원본 이력을 복사하지 않고 가상 값으로 단순화한다. 포트폴리오의 운영 성과·원본 수치와 예제의 재현 범위를 구분한다. 상세 문서의 코드 링크도 예제로 연결하며 예제가 원본 전체 통계를 증명한다고 쓰지 않는다. Authentik의 15개는 정책 정의가 아니라 정책 바인딩 수다.
- 전환은 새 예제 검증·공개 → 포트폴리오·상세 문서·PDF 링크 갱신 → 비로그인 링크 확인 → 기존 3개 저장소 Private 순서다. PDF는 바탕화면의 동명 파일도 교체한다.
- 2026-09-22 정본 확정: 같은 목적의 다른 초안 `docs/devops-portfolio.md`를 삭제하고 아래 정본 하나로 통합했다. PDF 생성기는 `tools/build_devops_portfolio_pdf.py`이며 정본과 `assets/portfolio/` 그림만 사용한다. 상세 문서는 PDF 부록으로 복제하지 않고 링크한다. 옛 `docs/assets/devops-portfolio/` 그림은 현재 제출본에 쓰지 않는다.
- PDF 폰트: Apple SD Gothic Neo는 CFF 기반 TTC여서 ReportLab TrueType 임베딩과 호환되지 않는다. 본문 한글은 Noto Sans KR 400/600, 영문 보조 라벨은 Helvetica Neue를 사용하고 원본 그림의 서체는 유지한다.
- 2026-09-22 전달본: `output/pdf/devops-portfolio.pdf`, 6쪽(전체 요약 1 / 워크로드 분산 2 / Traefik·SSO·GitOps 각 1). 원문 텍스트 전체 포함·그림 5개·내부 링크 4개·외부 코드·설계 링크와 전체 페이지 시각 검증을 마쳤다. 바탕화면의 동명 옛 PDF를 교체했다.
- 정본: [플랫폼 엔지니어링 포트폴리오](../docs/portfolio.md)
