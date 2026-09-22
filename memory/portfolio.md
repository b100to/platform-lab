# 채용 제출용 포트폴리오 (md + 그림)

- 범위: 네 작업(워크로드 분산, Istio → Traefik, Authentik SSO, GitOps 재설계)을 한 문서로 묶은 제출용 요약. 상세는 각 주제 문서로 링크한다.
- 구조: 첫 화면의 표만 읽어도 전체가 전달되게 하고, 케이스는 강한 것부터 놓았다(시간순 아님). 각 케이스는 문제·판단·결과·한계를 한 줄씩 항목으로 쓴다.
- 디자인: 이력서에 붙는 문서라서 흑백 선 그림 + 포인트 색 하나(짙은 남색), 각진 모서리, 채움 최소화. 처음 시안(파스텔 패널, 주황·청록, 큰 숫자 타일)은 사용자가 "AI 티가 난다"고 반려했다. 글꼴은 Helvetica Neue + Apple SD Gothic Neo, 굵기는 600까지만.
- 그림: `docs/assets/portfolio/build.py` 가 SVG 5장을 생성하고 `rsvg-convert` 로 PNG를 만든다. 라벨마다 상자 폭 대비 글자 폭을 검사해 넘치면 빌드가 실패한다. 그림을 고칠 땐 SVG·PNG를 손대지 말고 이 스크립트를 고친다.
- 수치는 상세 문서의 정정본을 따른다: backend 선언 39개(실험용 디렉터리 제외) → 생성 규칙 1개(S3 backend stack 69개). Istio는 "명시적 선언" 기준이며 메시 기능 미사용을 단정하지 않는다. ALB 단독도 rewrite·CORS는 가능하다.
- 개인정보는 2026-09-22 사용자 답변으로 정본에 반영했다. GitOps 기간은 로컬 이력서 `/Users/jonghwabaek/works/workspace/resume/index.html`의 IaC·GitOps 항목(2024.03 – 2026.03)을 따른다.
- 제출용 PDF도 반드시 아래 정본에서 생성한다. 별도 요약 초안을 만들거나 이전 초안의 그림·사례 순서를 재사용하지 않는다. 이력서는 경력·성과 요약, 포트폴리오는 설계 판단·검증 근거로 역할을 나누고 기간·수치를 일치시킨다.
- 공개 코드 근거는 제출본 안에서 역할별로 직접 링크한다: 이전 인프라 [infra-config-portfolio](https://github.com/b100to/infra-config-portfolio) + 이전 배포 [manifest-k8s-cluster-portfolio](https://github.com/b100to/manifest-k8s-cluster-portfolio) → 현재 통합 모노레포 [devops-configs-portfolio](https://github.com/b100to/devops-configs-portfolio). 상세 설계 링크만으로 대신하지 않고, 독자가 개편 전후 코드를 바로 비교할 수 있게 세 저장소를 함께 보여준다.
- 2026-09-22 정본 확정: 같은 목적의 다른 초안 `docs/devops-portfolio.md`를 삭제하고 아래 정본 하나로 통합했다. PDF 생성기는 `tools/build_devops_portfolio_pdf.py`이며 정본과 `assets/portfolio/` 그림만 사용한다. 상세 문서는 PDF 부록으로 복제하지 않고 링크한다. 옛 `docs/assets/devops-portfolio/` 그림은 현재 제출본에 쓰지 않는다.
- PDF 폰트: Apple SD Gothic Neo는 CFF 기반 TTC여서 ReportLab TrueType 임베딩과 호환되지 않는다. 본문 한글은 Noto Sans KR 400/600, 영문 보조 라벨은 Helvetica Neue를 사용하고 원본 그림의 서체는 유지한다.
- 2026-09-22 전달본: `output/pdf/devops-portfolio.pdf`, 6쪽(전체 요약 1 / 워크로드 분산 2 / Traefik·SSO·GitOps 각 1). 원문 텍스트 전체 포함·그림 5개·내부 링크 4개·외부 코드·설계 링크와 전체 페이지 시각 검증을 마쳤다. 바탕화면의 동명 옛 PDF를 교체했다.
- 정본: [플랫폼 엔지니어링 포트폴리오](../docs/portfolio.md)
