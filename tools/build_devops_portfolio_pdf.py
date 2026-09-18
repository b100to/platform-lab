#!/usr/bin/env python3
"""Build a standalone Korean portfolio from the five canonical Markdown files.

Dependencies: reportlab, markdown-it-py, fonttools, Pillow, pypdf.
Usage: python tools/build_devops_portfolio_pdf.py [--output output/pdf/devops-portfolio.pdf]
Set --font-path or PORTFOLIO_KOREAN_FONT to a Noto Sans KR variable TTF.
The six Mermaid figures are drawn as vector diagrams; source labels are read afresh.
"""
from __future__ import annotations

import argparse
import hashlib
import html
import math
import os
from pathlib import Path
import re

from fontTools.ttLib import TTFont as FontToolsFont
from fontTools.varLib.instancer import instantiateVariableFont
from markdown_it import MarkdownIt
from PIL import Image as PILImage
from reportlab.lib import colors
from reportlab.lib.enums import TA_CENTER
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import (
    BaseDocTemplate, CondPageBreak, Flowable, Frame, Image, KeepTogether, PageBreak,
    PageTemplate, Paragraph, Spacer, Table, TableStyle,
)

ROOT = Path(__file__).resolve().parents[1]
DOCS = ROOT / "docs"
TMP = ROOT / "tmp/pdfs"
FILES = ["devops-configs-architecture.md", "eks-workload-availability.md",
         "sso-authentik.md", "istio-to-traefik.md"]
LABELS = ["GitOps 재설계", "EKS 워크로드 가용성", "Authentik SSO", "Istio → Traefik"]
INK = colors.HexColor("#182B42")
BLUE = colors.HexColor("#245EA5")
TEAL = colors.HexColor("#14796E")
MUTED = colors.HexColor("#53657A")
PALE = colors.HexColor("#F1F5F9")
LINE = colors.HexColor("#D9E2EB")
W, H = A4
MARGIN = 19 * mm
CONTENT = W - MARGIN * 2
MD = MarkdownIt("commonmark").enable("table")


def clean(text):
    return text.replace("\u2011", "-").replace("\u2013", "-").replace("\u2014", "-")


def font_setup(path):
    dest = TMP / "fonts"
    dest.mkdir(parents=True, exist_ok=True)
    for weight, name in [(400, "KR"), (700, "KR-Bold")]:
        style = "Regular" if weight == 400 else "Bold"
        target = dest / f"PortfolioNotoSansKR-{style}.ttf"
        if not target.exists() or target.stat().st_mtime < path.stat().st_mtime:
            font = FontToolsFont(path)
            if "fvar" in font:
                font = instantiateVariableFont(font, {"wght": weight}, inplace=True)
            names = {1: "Portfolio Noto Sans KR", 2: style,
                     3: f"PortfolioNotoSansKR-{style}",
                     4: f"Portfolio Noto Sans KR {style}",
                     6: f"PortfolioNotoSansKR-{style}",
                     16: "Portfolio Noto Sans KR", 17: style}
            for record in list(font["name"].names):
                if record.nameID in names:
                    font["name"].setName(names[record.nameID], record.nameID,
                                         record.platformID, record.platEncID, record.langID)
            font.save(target)
        pdfmetrics.registerFont(TTFont(name, str(target)))
    pdfmetrics.registerFontFamily("KR", normal="KR", bold="KR-Bold", italic="KR", boldItalic="KR-Bold")
    mono = Path.home() / "Library/Fonts/JetBrainsMonoNerdFontMono-Regular.ttf"
    if not mono.exists():
        mono = Path("/System/Library/Fonts/Supplemental/Andale Mono.ttf")
    pdfmetrics.registerFont(TTFont("Code", str(mono if mono.exists() else dest / "PortfolioNotoSansKR-Regular.ttf")))


ST = {}


def setup_styles():
    specs = {
        "body": dict(fontSize=9.4, leading=15.3, spaceAfter=7),
        "summary": dict(fontSize=10, leading=15.2, spaceAfter=8),
        "title": dict(fontName="KR-Bold", fontSize=21, leading=29, spaceAfter=15),
        "h2": dict(fontName="KR-Bold", fontSize=14, leading=20, spaceBefore=17, spaceAfter=8, keepWithNext=True),
        "h3": dict(fontName="KR-Bold", fontSize=11, leading=16, spaceBefore=11, spaceAfter=6, keepWithNext=True),
        "small": dict(fontSize=8.3, leading=12.8, textColor=MUTED, spaceAfter=6),
        "table": dict(fontSize=8.7, leading=13),
        "summarytable": dict(fontSize=9.5, leading=14),
        "code": dict(fontName="Code", fontSize=8, leading=11, spaceAfter=0, wordWrap="CJK"),
    }
    for name, opts in specs.items():
        ST[name] = ParagraphStyle(name, **{"fontName": "KR", "textColor": INK, "wordWrap": "CJK", **opts})


def anchor(file, title=""):
    return "d_" + hashlib.sha1((file + "#" + title).encode()).hexdigest()[:14]


def slug(text):
    text = re.sub(r"[`*_]", "", text).lower()
    text = re.sub(r"[^\w\s-]", "", text)
    return re.sub(r"\s", "-", text)


ANCHORS = {}


def load_anchors():
    for file in FILES:
        tokens = MD.parse((DOCS / file).read_text())
        for i, token in enumerate(tokens):
            if token.type == "heading_open" and token.tag == "h2":
                title = tokens[i + 1].content
                ANCHORS[(file, slug(title))] = anchor(file, title)


def target_link(url, source):
    if url.startswith(("https://", "http://", "mailto:")):
        return url
    path, _, frag = url.partition("#")
    file = Path(path).name if path else source
    if file in FILES:
        return "#" + ANCHORS.get((file, frag), anchor(file))
    return None


def inline(children, source):
    result = []
    links = []
    for t in children or []:
        if t.type == "text": result.append(html.escape(clean(t.content)))
        elif t.type == "code_inline": result.append('<font color="#285A72">' + html.escape(clean(t.content)) + "</font>")
        elif t.type == "strong_open": result.append("<b>")
        elif t.type == "strong_close": result.append("</b>")
        elif t.type == "em_open": result.append("<i>")
        elif t.type == "em_close": result.append("</i>")
        elif t.type in ("softbreak", "hardbreak"): result.append("<br/>" if t.type == "hardbreak" else " ")
        elif t.type == "html_inline":
            if re.fullmatch(r"<br\s*/?>", t.content): result.append("<br/>")
        elif t.type == "link_open":
            url = target_link(t.attrGet("href"), source)
            links.append(url)
            if url: result.append('<a href="' + html.escape(url, quote=True) + '" color="#245EA5">')
        elif t.type == "link_close":
            if links.pop(): result.append("</a>")
    return "".join(result)


def p(text, style="body"):
    return Paragraph(text, ST[style])


class SectionMark(Flowable):
    def __init__(self, key, title, level=0):
        super().__init__()
        self.key, self.title, self.level = key, title, level
        self.width, self.height = 0, 0
        self.keepWithNext = True

    def draw(self):
        self.canv.bookmarkPage(self.key)
        self.canv.addOutlineEntry(self.title, self.key, self.level, closed=True)
        if self.level == 0:
            self.canv._portfolio_chapter = self.title


class Diagram(Flowable):
    """Small, deterministic vector layouts for the six source Mermaid figures."""
    def __init__(self, source):
        super().__init__()
        self.source = source
        self.width = CONTENT
        self.height = 245
        self.labels = {k: v.replace("<br/>", "\n") for k, v in re.findall(r"\b(\w+)\[([^\]]+)\]", source)}
        self.nodes = {}

    def box(self, name, x, y, width=87, height=45):
        c = self.canv
        self.nodes[name] = (x, y, width, height)
        c.setFillColor(PALE)
        c.setStrokeColor(LINE)
        c.roundRect(x, y, width, height, 5, fill=1, stroke=1)
        style = ParagraphStyle("node", fontName="KR", fontSize=8.2, leading=11, alignment=TA_CENTER, wordWrap="CJK", textColor=INK)
        para = Paragraph(html.escape(clean(self.labels.get(name, name))).replace("\n", "<br/>"), style)
        _, ht = para.wrap(width - 10, height)
        if ht > height - 4:
            raise ValueError(f"Diagram label exceeds node: {name}")
        para.drawOn(c, x + 5, y + (height - ht) / 2)

    def edge(self, a, b, label="", dashed=False, route=None, ends=("r", "l"), double=False):
        c = self.canv
        def pos(n, side):
            x,y,w,h = self.nodes[n]
            return {"r":(x+w,y+h/2),"l":(x,y+h/2),"t":(x+w/2,y+h),"b":(x+w/2,y)}[side]
        start, end = pos(a,ends[0]), pos(b,ends[1])
        pts = [start] + (route or []) + [end]
        c.setStrokeColor(TEAL if dashed else BLUE)
        c.setFillColor(TEAL if dashed else BLUE)
        c.setLineWidth(0.8)
        c.setDash(3,2) if dashed else c.setDash()
        path = c.beginPath(); path.moveTo(*start)
        for xy in pts[1:]: path.lineTo(*xy)
        c.drawPath(path)
        c.setDash()
        def arrow(prev, dest):
            angle = math.atan2(dest[1]-prev[1],dest[0]-prev[0])
            q = c.beginPath(); q.moveTo(*dest)
            q.lineTo(dest[0]-5*math.cos(angle-.45),dest[1]-5*math.sin(angle-.45))
            q.lineTo(dest[0]-5*math.cos(angle+.45),dest[1]-5*math.sin(angle+.45)); q.close()
            c.drawPath(q,fill=1,stroke=0)
        arrow(pts[-2],end)
        if double: arrow(pts[1],start)
        if label:
            mid = pts[len(pts)//2] if route else ((start[0]+end[0])/2,(start[1]+end[1])/2)
            c.setFont("KR",7.2)
            c.setFillColor(MUTED)
            c.drawCentredString(mid[0],mid[1]+6,clean(label))

    def draw(self):
        s = self.source
        if "generated Terraform" in s:
            self.labels["GEN"] = "generated\nTerraform"
            self.labels["APP"] = "Argo CD\nApplication"
            for n,x,y,w in [("GIT",0,113,80),("CI",99,194,61),("TM",180,194,87),("GEN",286,194,87),("AWS",393,194,94),("APP",99,45,85),("CHART",216,112,109),("VALUES",216,56,109),("RAW",216,0,109),("K8S",393,56,94)]: self.box(n,x,y,w)
            self.edge("GIT","CI",route=[(88,135),(88,216)])
            self.edge("GIT","APP",route=[(88,135),(88,67)])
            for a,b in [("CI","TM"),("TM","GEN"),("GEN","AWS")]: self.edge(a,b)
            for n,y in [("CHART",134),("VALUES",78),("RAW",22)]:
                self.edge("APP",n,route=[(200,67),(200,y)])
                self.edge(n,"K8S",route=[(353,y),(353,78)])
            self.edge("AWS","K8S","실행 기반",ends=("b","t"))
        elif "ROOT[config.tm.hcl" in s:
            ids=["ROOT","DOMAIN","KIND","TARGET","ENV","STACK"]
            for i,n in enumerate(ids): self.box(n,85,207-i*40,318,33)
            for a,b in zip(ids,ids[1:]): self.edge(a,b,ends=("b","t"))
        elif "Helm render" in s:
            self.height = 180
            self.box("ARGO",0,80,150,60); self.box("CHART",186,130,150,57); self.box("ENV",186,22,150,57); self.box("RENDER",375,81,111,60)
            self.edge("ARGO","CHART",route=[(167,110),(167,158)])
            self.edge("ARGO","ENV",route=[(167,110),(167,50)])
            self.canv.setFont("KR",7.2); self.canv.setFillColor(MUTED)
            self.canv.drawString(155,194,"chart 선택")
            self.canv.drawString(155,9,"valueFiles 선택")
            self.edge("CHART","RENDER",route=[(353,158),(353,111)])
            self.edge("ENV","RENDER",route=[(353,50),(353,111)])
        elif "Descheduler evict" in s:
            for n,x in [("N",0),("D",175),("S",350)]: self.box(n,x,99,137,65)
            self.edge("N","D","노드 이탈")
            self.edge("D","S","노드 복귀")
            self.edge("S","N","Descheduler evict · PDB가 1개 유지",route=[(418.5,55),(68.5,55)],ends=("b","b"))
        elif "subgraph login" in s:
            # Three lanes preserve the distinct login, configuration and secret flows.
            for n,x,y,w in [("U",0,188,64),("T",82,188,71),("APP",173,188,83),("AK",310,188,87),("G",417,188,71),("GIT",0,105,130),("ARGO",149,105,81),("CM",249,105,88),("SM",0,22,133),("ESO",153,22,113),("KS",286,22,95)]: self.box(n,x,y,w,39)
            for a,b in [("U","T"),("T","APP"),("AK","G"),("GIT","ARGO"),("ARGO","CM"),("SM","ESO"),("ESO","KS")]:self.edge(a,b)
            self.edge("APP","AK",double=True)
            self.canv.setFont("KR",7);self.canv.setFillColor(MUTED);self.canv.drawCentredString(284,235,"OIDC · OAuth · SAML")
            self.edge("CM","AK",route=[(354,124)],ends=("r","b"))
            self.edge("KS","AK",route=[(402,41),(402,173),(354,173)],ends=("r","b"))
            self.edge("KS","APP",route=[(333.5,79),(239,79),(239,173),(214.5,173)],ends=("t","b"))
        elif "AWS Load Balancer Controller" in s:
            for n,x,y,w in [("I",0,88,60),("R53",81,88,77),("ALB",183,88,83),("T",309,88,78),("S1",412,123,75),("S2",412,55,75),("EDNS",43,185,115),("LBC",177,185,140),("GIT",333,185,153)]:self.box(n,x,y,w,43)
            for a,b in [("I","R53"),("R53","ALB")]:self.edge(a,b)
            self.edge("ALB","T","catch-all")
            self.edge("T","S1",route=[(400,109),(400,144)])
            self.edge("T","S2",route=[(400,109),(400,76)])
            self.edge("EDNS","R53","hostname annotation",dashed=True,ends=("b","t"))
            self.edge("LBC","ALB","Ingress 1개 관리",dashed=True,ends=("b","t"))
            self.edge("GIT","T","Argo CD",dashed=True,ends=("b","t"))
        else: raise ValueError("Unrecognized Mermaid figure; define its vector layout before publishing")


def table_from_tokens(tokens, i, source, summary):
    rows, row = [], []
    i += 1
    while tokens[i].type != "table_close":
        t=tokens[i]
        if t.type == "tr_open": row=[]
        if t.type == "inline": row.append(p(inline(t.children,source), "summarytable" if summary else "table"))
        if t.type == "tr_close": rows.append(row)
        i+=1
    count=len(rows[0]); ratios={2:[.30,.70],3:[.23,.38,.39],4:[.16,.28,.28,.28],5:[.16,.21,.21,.21,.21]}.get(count,[1/count]*count)
    tab=Table(rows,colWidths=[CONTENT*r for r in ratios],repeatRows=1,hAlign="LEFT")
    tab.setStyle(TableStyle([
        ("BACKGROUND",(0,0),(-1,0),INK),("TEXTCOLOR",(0,0),(-1,0),colors.white),
        ("VALIGN",(0,0),(-1,-1),"TOP"),("ROWBACKGROUNDS",(0,1),(-1,-1),[colors.white,PALE]),
        ("LINEBELOW",(0,0),(-1,0),.7,INK),("LINEBELOW",(0,1),(-1,-1),.35,LINE),
        ("LEFTPADDING",(0,0),(-1,-1),7),("RIGHTPADDING",(0,0),(-1,-1),7),
        ("TOPPADDING",(0,0),(-1,-1),7),("BOTTOMPADDING",(0,0),(-1,-1),7),
    ]))
    for cell in rows[0]:
        cell.__init__(cell.text, ParagraphStyle("headercell",parent=cell.style,textColor=colors.white,fontName="KR-Bold"))
    return [tab,Spacer(1,10)],i


def code_flow(text):
    # XPreformatted cannot split long source lines, so wrap explicitly by glyph width.
    out=[]
    for line in clean(text).expandtabs(2).splitlines():
        chunks=[]; current=""; width=0
        for ch in line:
            face="Code" if ord(ch)<128 or 0x2500<=ord(ch)<=0x257f else "KR"
            cw=pdfmetrics.stringWidth(ch,face,8)
            if width+cw > CONTENT-22:
                chunks.append(current); current=""; width=0
            current+=ch; width+=cw
        chunks.append(current)
        for chunk in chunks:
            markup=""
            for match in re.finditer(r"[\x00-\x7f\u2500-\u257f]+|[^\x00-\x7f\u2500-\u257f]+",chunk):
                text=match.group(); face="Code" if ord(text[0])<128 or 0x2500<=ord(text[0])<=0x257f else "KR"
                markup+=f'<font name="{face}">'+html.escape(text).replace(" ","&#160;")+"</font>"
            para=p(markup or "&#160;","code")
            out.append([para])
    tab=Table(out,colWidths=[CONTENT],hAlign="LEFT")
    tab.setStyle(TableStyle([("BACKGROUND",(0,0),(-1,-1),PALE),("LEFTPADDING",(0,0),(-1,-1),10),("RIGHTPADDING",(0,0),(-1,-1),10),("TOPPADDING",(0,0),(-1,-1),0),("BOTTOMPADDING",(0,0),(-1,-1),0),("TOPPADDING",(0,0),(-1,0),8),("BOTTOMPADDING",(0,-1),(-1,-1),8)]))
    if tab.wrap(CONTENT, H)[1] < 400:
        return [KeepTogether([tab,Spacer(1,9)])]
    return [tab,Spacer(1,9)]


def markdown_flow(text, source, summary=False, skip_h1=False):
    tokens=MD.parse(text); out=[]; i=0; list_level=0
    while i<len(tokens):
        t=tokens[i]
        if t.type=="table_open":
            flow,i=table_from_tokens(tokens,i,source,summary);out.extend(flow)
        elif t.type=="heading_open":
            level=int(t.tag[1]); title=tokens[i+1].content
            if not (level==1 and skip_h1):
                if not summary and level in (2, 3):
                    out.append(CondPageBreak(85 if level == 2 else 60))
                if not summary and level==2:out.append(SectionMark(anchor(source,title),clean(title),1))
                title_markup = inline(tokens[i+1].children,source)
                if level == 1 and len(title) > 30:
                    title_markup = title_markup.replace(": ", ":<br/>", 1)
                out.append(p(title_markup,"title" if level<=1 or summary else "h2" if level==2 else "h3"))
            i+=2
        elif t.type=="paragraph_open":
            tok=tokens[i+1]
            image=next((c for c in tok.children or [] if c.type=="image"),None)
            if image:
                path=DOCS/image.attrGet("src")
                with PILImage.open(path) as im: width,height=im.size
                target_h=min(238 if summary else 300, CONTENT*height/width)
                out.append(Image(str(path),width=target_h*width/height,height=target_h,hAlign="CENTER"));out.append(Spacer(1,10))
            else:
                markup=inline(tok.children,source)
                if list_level: markup="• "+markup
                paragraph = p(markup,"summary" if summary else "body")
                if tok.content.rstrip().endswith("발췌:"):
                    paragraph.keepWithNext = True
                out.append(paragraph)
            i+=2
        elif t.type in ("bullet_list_open","ordered_list_open"):list_level+=1
        elif t.type in ("bullet_list_close","ordered_list_close"):list_level-=1
        elif t.type=="fence":
            if t.info.strip()=="mermaid":out.extend([Diagram(t.content),Spacer(1,8)])
            else:out.extend(code_flow(t.content))
        elif t.type=="hr":out.append(Spacer(1,7))
        i+=1
    return out


class PortfolioDoc(BaseDocTemplate):
    def __init__(self,path):
        super().__init__(str(path),pagesize=A4,leftMargin=MARGIN,rightMargin=MARGIN,topMargin=17*mm,bottomMargin=19*mm,title="DevOps 포트폴리오 | 플랫폼 설계와 운영",author="",allowSplitting=1)
        frame=Frame(MARGIN,19*mm,CONTENT,H-36*mm,leftPadding=0,rightPadding=0,topPadding=0,bottomPadding=0)
        self.addPageTemplates(PageTemplate(id="default",frames=[frame],onPageEnd=self.footer))

    def footer(self,canvas,doc):
        canvas.saveState();canvas.setStrokeColor(LINE);canvas.line(MARGIN,15*mm,W-MARGIN,15*mm)
        canvas.setFont("KR",8);canvas.setFillColor(MUTED)
        chapter=getattr(canvas,"_portfolio_chapter","DevOps 포트폴리오")
        canvas.drawString(MARGIN,10*mm,chapter[:65]);canvas.drawRightString(W-MARGIN,10*mm,str(doc.page));canvas.restoreState()


def cover(summary):
    intro=re.split(r"^## ",summary,maxsplit=1,flags=re.M)[0]
    intro=re.sub(r"^# .*\n", "", intro)
    out=[Spacer(1,18),p("DEVOPS / PLATFORM ENGINEERING","small"),Spacer(1,13),
         p("팀이 이해하고<br/>운영할 수 있는 플랫폼","title"),Spacer(1,5)]
    out+=markdown_flow(intro,"devops-portfolio.md",True)
    out.extend([Spacer(1,16),p("읽는 방법","h2"),p("<b>빠른 검토: 2-5쪽</b>에서 문제·구조·선택 이유를 확인합니다.<br/><b>상세 검토: 부록</b>에서 설정 예시, 검증 범위와 운영 과제를 확인합니다.","summary"),Spacer(1,6)])
    rows=[[p("사례","summarytable"),p("요약","summarytable"),p("상세 설계","summarytable")]]
    for i,(file,label) in enumerate(zip(FILES,LABELS)):
        rows.append([p(label,"summarytable"),p(f'<a href="#summary-{i}" color="#245EA5">{i+2}쪽으로 이동</a>',"summarytable"),p(f'<a href="#{anchor(file)}" color="#245EA5">부록 {i+1} 열기</a>',"summarytable")])
    tab=Table(rows,colWidths=[CONTENT*.49,CONTENT*.24,CONTENT*.27])
    tab.setStyle(TableStyle([("LINEBELOW",(0,0),(-1,-1),.35,LINE),("VALIGN",(0,0),(-1,-1),"TOP"),("TOPPADDING",(0,0),(-1,-1),9),("BOTTOMPADDING",(0,0),(-1,-1),9)]))
    out.extend([tab,Spacer(1,20),p('<a href="https://github.com/b100to/devops-configs-portfolio" color="#245EA5">현재 공개 코드</a> · <a href="https://github.com/b100to/manifest-k8s-cluster-portfolio" color="#245EA5">이전 배포 저장소</a>',"summary"),p("설계 근거와 공개 코드 링크는 문서 안에서 클릭할 수 있습니다.","small")])
    return out


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output",type=Path,default=ROOT/"output/pdf/devops-portfolio.pdf")
    parser.add_argument("--font-path",type=Path,default=Path(os.environ.get("PORTFOLIO_KOREAN_FONT",str(Path.home()/"Library/Fonts/NotoSansKR-VariableFont_wght.ttf"))))
    args=parser.parse_args();font_setup(args.font_path);setup_styles();load_anchors()
    summary=(DOCS/"devops-portfolio.md").read_text()
    sections=re.split(r"(?=^## )",summary,flags=re.M)[1:]
    if len(sections)!=4: raise ValueError("Expected exactly four portfolio cases")
    story=cover(summary)
    for i,section in enumerate(sections):
        story += [PageBreak(),SectionMark(f"summary-{i}",f"요약 {i+1}. {LABELS[i]}")]
        story += markdown_flow(section,"devops-portfolio.md",summary=True)
    for i,file in enumerate(FILES):
        story += [PageBreak(),SectionMark(anchor(file),f"부록 {i+1}. {LABELS[i]}"),p(f"APPENDIX {i+1:02}","small")]
        story += markdown_flow((DOCS/file).read_text(),file)
    args.output.parent.mkdir(parents=True,exist_ok=True)
    PortfolioDoc(args.output).build(story)
    from pypdf import PdfReader
    reader=PdfReader(args.output)
    for i,label in enumerate(["1. GitOps","2. EKS","3. Authentik","4. Istio"],start=1):
        if label not in reader.pages[i].extract_text():
            raise ValueError(f"Summary case {label} must begin on page {i+1}; adjust spacing")
    if "APPENDIX 01" not in reader.pages[5].extract_text():raise ValueError("Appendix must begin on page 6")
    if any(not page.extract_text().strip() for page in reader.pages):raise ValueError("Blank page found")
    counts={"internal":0,"external":0}
    for page in reader.pages:
        for ref in page.get("/Annots",[]):
            ann=ref.get_object()
            if ann.get("/Subtype")=="/Link":counts["external" if ann.get("/A",{}).get("/URI") else "internal"]+=1
    print(f"Created {args.output}: {len(reader.pages)} pages; links={counts}; summary pages=2-5, appendix starts=6")


if __name__=="__main__":main()
