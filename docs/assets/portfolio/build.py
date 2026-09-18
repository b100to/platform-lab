#!/usr/bin/env python3
"""Generate the portfolio figures: SVG sources + PNG renders (needs rsvg-convert).

    python3 docs/assets/portfolio/build.py

Style: monochrome line drawings with one accent, sized for an A4 page.
Every label is width-checked against its box before writing, so a text
overflow fails here instead of showing up in the rendered image.
"""
import os, subprocess, sys
from xml.sax.saxutils import escape

OUT = os.path.dirname(os.path.abspath(__file__))
W = 920
INK, TEXT, GRAY, HAIR, FILL = "#111827", "#374151", "#6B7280", "#D1D5DB", "#F3F4F6"
ACC = "#1F3A5F"                      # the one accent: the chosen / resulting state
FONT = "'Helvetica Neue','Apple SD Gothic Neo',sans-serif"
problems = []

def tw(s, size, heavy=False):
    w = 0.0
    for ch in s:
        o = ord(ch)
        if o >= 0x2E80 or 0x1100 <= o <= 0x11FF: w += 1.0
        elif ch == " ": w += 0.28
        elif ch.isupper(): w += 0.68
        elif ch in "·:.,|/()[]'!il": w += 0.30
        elif ch in "→": w += 1.0
        else: w += 0.56
    return w * size * (1.04 if heavy else 1.0)

class Fig:
    def __init__(s, name, h, desc):
        s.name, s.h = name, h
        s.el = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{h}" viewBox="0 0 {W} {h}" role="img" aria-label="{escape(desc)}">',
                f'<defs><marker id="ah" markerWidth="8" markerHeight="8" refX="6.5" refY="4" orient="auto"><path d="M1.5 1.2 L6.8 4 L1.5 6.8" fill="none" stroke="{GRAY}" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/></marker></defs>',
                f'<rect width="{W}" height="{h}" fill="#FFFFFF"/>']
    def rect(s, x, y, w, h, fill="#FFFFFF", stroke=INK, sw=1, dash=None, rx=3):
        d = f' stroke-dasharray="{dash}"' if dash else ""
        st = f' stroke="{stroke}" stroke-width="{sw}"' if stroke else ""
        s.el.append(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{rx}" fill="{fill}"{st}{d}/>')
    def text(s, x, y, t, size=14.5, weight=400, fill=TEXT, anchor="start", maxw=None, ls=0):
        if maxw is not None and tw(t, size, weight >= 600) > maxw:
            problems.append(f"{s.name}: '{t}' ~{tw(t, size, weight >= 600):.0f}px > {maxw}px")
        sp = f' letter-spacing="{ls}"' if ls else ""
        s.el.append(f'<text x="{x}" y="{y}" font-family="{FONT}" font-size="{size}" font-weight="{weight}" fill="{fill}" text-anchor="{anchor}"{sp}>{escape(t)}</text>')
    def line(s, x1, y1, x2, y2, color=HAIR, sw=1, dash=None, head=False):
        d = f' stroke-dasharray="{dash}"' if dash else ""
        m = ' marker-end="url(#ah)"' if head else ""
        s.el.append(f'<line x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}" stroke="{color}" stroke-width="{sw}" stroke-linecap="round"{d}{m}/>')
    def arrow(s, x1, y1, x2, y2): s.line(x1, y1, x2, y2, GRAY, 1.2, head=True)
    def box(s, x, y, w, h, title, sub=None, accent=False, dash=None, fill="#FFFFFF"):
        s.rect(x, y, w, h, fill, ACC if accent else INK, 1.5 if accent else 1, dash)
        c = ACC if accent else INK
        if sub:
            s.text(x + w/2, y + h/2 - 3, title, 14.5, 600, c, "middle", w - 12)
            s.text(x + w/2, y + h/2 + 15, sub, 12.5, 400, GRAY, "middle", w - 8)
        else:
            s.text(x + w/2, y + h/2 + 5, title, 14.5, 600, c, "middle", w - 12)
    def pod(s, cx, y, new=False, size=22):
        s.rect(cx - size/2, y, size, size, "#FFFFFF" if new else ACC, ACC, 1.5, rx=2)
    def save(s):
        p = os.path.join(OUT, s.name + ".svg")
        open(p, "w").write("\n".join(s.el) + "\n</svg>\n")
        subprocess.run(["rsvg-convert", "-w", str(W * 3), p, "-o", p[:-4] + ".png"], check=True)

# ------------------------------------------------------------ 1. availability: failure scenario
f = Fig("01-availability", 246, "장애 시나리오 다섯 단계: 평시 1대1, 노드 A 이탈, 대체 pod 기동, 복귀 뒤 0대2, 재분산 1대1")
steps = [("평시 1 : 1", "TSC hostname 강제", "ok", 1, [0]),
         ("노드 A 이탈", "B가 계속 서빙", "down", 0, [0]),
         ("대체 pod 기동", "Honor · 죽은 노드 제외", "down", 0, [0, 1]),
         ("A 복귀 0 : 2", "아무도 옮기지 않음", "ok", 0, [0, 0]),
         ("재분산 1 : 1", "Descheduler + PDB", "ok", -1, [0])]
for i, (title, cap, a_state, a_pods, b_pods) in enumerate(steps):
    x = 24 + i * 178
    f.text(x, 40, str(i + 1), 13, 600, GRAY)
    f.text(x + 16, 40, title, 16, 600, INK, maxw=144)
    f.text(x, 60, cap, 13, 400, GRAY, maxw=160)
    for j, lab in enumerate("AB"):
        nx = x + j * 88
        down = j == 0 and a_state == "down"
        f.rect(nx, 76, 72, 84, FILL if down else "#FFFFFF", GRAY if down else INK, 1, "4 3" if down else None)
        f.text(nx + 36, 95, "노드 " + lab, 13, 400, GRAY, "middle")
        if down:
            f.text(nx + 36, 130, "NotReady", 12.5, 400, GRAY, "middle", 66)
        pods = b_pods if j == 1 else ([1] if a_pods == -1 else [0] * a_pods)
        for k, new in enumerate(pods):
            f.pod(nx + 36, 102 + k * 28, bool(new))
    if i < 4: f.arrow(x + 163, 118, x + 175, 118)
f.line(24, 182, W - 24, 182)
f.pod(35, 197, False, 14); f.text(50, 209, "pod", 13, 400, GRAY)
f.pod(95, 197, True, 14);  f.text(110, 209, "새로 뜬 pod", 13, 400, GRAY)
f.text(230, 209, "노드 공급 · Karpenter", 13, 600, INK)
f.text(372, 209, "AZ별 NodePool, pool당 통상 1대, 최초 만료 시점 분산, pool별 중단 예산 1대", 13, 400, TEXT, maxw=520)
f.text(24, 232, "복귀만으로는 분포가 돌아오지 않는다. 배치(TSC)와 재배치(Descheduler)가 따로 필요한 이유.", 13, 400, GRAY, maxw=860)
f.save()

# ------------------------------------------------------------ 1b. Ignore vs Honor
f = Fig("01b-honor", 224, "nodeTaintsPolicy 비교: Ignore는 taint 노드를 0개 도메인으로 세어 세 번째 pod이 Pending, Honor는 제외해 2대1 배치")
for r, honor in enumerate((False, True)):
    y = 24 + r * 104
    f.text(24, y + 34, "Honor" if honor else "Ignore", 16, 600, ACC if honor else INK)
    if not honor: f.text(80, y + 34, "기본값", 13, 400, GRAY)
    f.text(24, y + 56, "갈 수 없는 노드는 제외" if honor else "taint 노드도 도메인으로 셈", 13, 400, GRAY, maxw=172)
    for j in range(2):
        nx = 210 + j * 150
        f.rect(nx, y, 134, 80)
        f.text(nx + 67, y + 20, f"앱 노드 {j + 1}", 13, 400, GRAY, "middle")
        n = (2 if j == 0 else 1) if honor else 1
        for k in range(n):
            f.pod(nx + 67 + (k - (n - 1) / 2) * 32, y + 38, honor and j == 0 and k == 1)
    tx = 510
    f.rect(tx, y, 134, 80, FILL if honor else "#FFFFFF", HAIR if honor else INK, 1, None if honor else "4 3")
    f.text(tx + 67, y + 20, "taint 노드", 13, 400, GRAY, "middle")
    f.text(tx + 67, y + 54, "계산에서 제외" if honor else "0개 · 계산에 포함", 13, 400 if honor else 600, GRAY if honor else INK, "middle", 126)
    f.arrow(656, y + 40, 680, y + 40)
    f.text(694, y + 36, "2 : 1 배치" if honor else "3번째 pod Pending", 16, 600, ACC if honor else INK, maxw=200)
    f.text(694, y + 58, "2 vs 1 → skew 1 (한도 이내)" if honor else "2 vs 0 → skew 2 (한도 1 초과)", 13, 400, GRAY, maxw=204)
    if not honor: f.line(24, y + 92, W - 24, y + 92)
f.save()

# ------------------------------------------------------------ 2. mesh before / after
f = Fig("02-mesh", 292, "전환 전에는 Istio gateway, pod마다 sidecar, istiod가 있었고 전환 후에는 ALB, Traefik, 앱만 있는 pod")
def half(x0, title, after):
    f.text(x0, 40, title, 16, 600, ACC if after else INK)
    f.box(x0, 100, 64, 52, "ALB")
    f.arrow(x0 + 68, 126, x0 + 84, 126)
    f.box(x0 + 88, 100, 110, 52, "Traefik" if after else "Gateway", "L7 라우팅" if after else "Istio · Envoy", accent=after)
    f.arrow(x0 + 202, 126, x0 + 218, 126)
    for k in range(2):
        py = 62 + k * 70
        f.rect(x0 + 222, py, 202, 58, "#FFFFFF", HAIR)
        f.text(x0 + 230, py + 16, "pod", 12.5, 400, GRAY)
        if after:
            f.rect(x0 + 230, py + 22, 186, 28); f.text(x0 + 323, py + 41, "app", 13.5, 600, INK, "middle")
        else:
            f.rect(x0 + 230, py + 22, 86, 28); f.text(x0 + 273, py + 41, "app", 13.5, 600, INK, "middle")
            f.rect(x0 + 322, py + 22, 94, 28, FILL, GRAY); f.text(x0 + 369, py + 41, "sidecar", 13.5, 400, TEXT, "middle")
    if not after:
        f.box(x0 + 88, 172, 110, 34, "istiod", dash="4 3", fill=FILL)
        f.line(x0 + 198, 189, x0 + 218, 176, GRAY, 1.2, "4 3", head=True)
    f.text(x0, 232, "알아야 할 것", 13, 400, GRAY)
    f.text(x0 + 84, 232, "Kubernetes → CRD 두 종류" if after else "Kubernetes → Istio → Envoy", 14.5, 600 if after else 400, ACC if after else TEXT, maxw=330)
half(24, "전환 전 · 서비스 메시", False)
f.line(460, 24, 460, 238)
half(472, "전환 후 · 인그레스", True)
f.line(24, 250, W - 24, 250)
f.text(24, 276, "공개 스냅샷의 명시적 Istio 선언", 13, 400, GRAY)
f.text(232, 276, "Gateway 4  ·  VirtualService 4  ·  mTLS · 인가 · 트래픽 정책 0", 14.5, 600, INK, maxw=640)
f.save()

# ------------------------------------------------------------ 3. SSO
f = Fig("03-sso", 322, "사용자는 Google 계정으로 Authentik에 로그인하고 Authentik이 앱의 능력에 따라 다섯 방식으로 연동한다. 설정은 Git에서, 시크릿은 Secrets Manager에서 온다")
f.box(24, 104, 112, 52, "사용자", "Google 계정")
f.arrow(140, 130, 160, 130)
f.box(164, 88, 140, 84, "Authentik", "IdP · 그룹 · 정책", accent=True)
f.line(304, 130, 332, 130, GRAY, 1.2)
f.line(332, 46, 332, 214, GRAY, 1.2)
rows = [("OIDC", "Argo CD · Argo Workflows · 위키"), ("OAuth", "Grafana · Airflow"), ("oauth2-proxy", "인증 기능이 없는 도구"),
        ("SAML", "AWS 콘솔 · 임시 세션"), ("OIDC + PKCE", "AWS CLI · credential_process로 자동 갱신")]
for k, (m, tgt) in enumerate(rows):
    y = 46 + k * 42
    f.arrow(332, y, 350, y)
    f.text(360, y + 5, m, 14.5, 600, INK, maxw=130)
    f.text(500, y + 5, tgt, 14.5, 400, TEXT, maxw=396)
f.text(360, 242, "연동 방식은 앱이 무엇을 지원하는가로 정한다", 13, 400, GRAY)
f.line(24, 258, W - 24, 258)
f.text(24, 284, "설정", 13, 600, INK)
f.text(84, 284, "Git (Blueprint YAML)  →  Argo CD  →  ConfigMap  →  Authentik", 14.5, 400, TEXT, maxw=800)
f.text(24, 308, "시크릿", 13, 600, INK)
f.text(84, 308, "Secrets Manager  →  External Secrets  →  K8s Secret  →  Authentik · 앱", 14.5, 400, TEXT, maxw=800)
f.save()

# ------------------------------------------------------------ 4. GitOps
f = Fig("04-gitops", 312, "입사 시점에는 Terraform 저장소와 manifest 저장소, 환경 브랜치 셋이 있었고 현재는 모노레포 하나에서 인프라 흐름과 배포 흐름이 나뉜다")
f.text(24, 40, "입사 시점 · 저장소 둘", 16, 600, INK)
f.box(24, 56, 276, 52, "Terraform repo", "Terraform Cloud · workspace")
f.box(24, 118, 276, 52, "Manifest repo", "서비스별 Helm chart")
f.text(24, 196, "환경 브랜치  dev / stage / prod 를 맞춰야 함", 13, 400, GRAY, maxw=280)
f.text(24, 216, "연결된 변경 하나 = PR 두 개", 13, 400, GRAY, maxw=280)
f.arrow(314, 113, 340, 113)
f.text(356, 40, "현재 · GitOps 모노레포", 16, 600, ACC)
flows = [("인프라", [("modules/", "생성 규칙"), ("stacks/…/<env>", "환경 = leaf 디렉터리"), ("AWS · EKS", "CI가 plan · apply")]),
         ("배포", [("chart + values", "공통 골격 + 차이"), ("Argo CD", "Application"), ("Kubernetes", "워크로드")])]
for r, (lab, cells) in enumerate(flows):
    y = 56 + r * 62
    f.text(356, y + 31, lab, 13, 600, INK)
    for c, (t, sub) in enumerate(cells):
        x = 412 + c * 166
        f.box(x, y, 148, 52, t, sub, accent=(c == 1))
        if c < 2: f.arrow(x + 151, y + 26, x + 163, y + 26)
f.text(356, 196, "main + feature 브랜치. 환경은 브랜치가 아니라 디렉터리와 stack tag.", 13, 400, GRAY, maxw=540)
f.text(356, 216, "한 앱의 예외는 values에, 여러 소비자의 계약은 chart와 module에.", 13, 400, GRAY, maxw=540)
f.line(24, 238, W - 24, 238)
for k, (num, lab) in enumerate((("39 → 1", "손으로 쓴 backend 선언"), ("59 → 0", "terraform_remote_state 참조"), ("7 → 1", "AWS provider 버전 제약의 종류"))):
    x = 24 + k * 300
    f.text(x, 272, num, 21, 600, ACC)
    f.text(x, 294, lab, 13, 400, GRAY, maxw=280)
f.save()

if problems:
    print("TEXT OVERFLOW:"); [print("  " + p) for p in problems]; sys.exit(1)
print("ok:", ", ".join(sorted(x for x in os.listdir(OUT) if x.endswith(".png"))))
