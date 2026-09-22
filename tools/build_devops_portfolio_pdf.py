#!/usr/bin/env python3
"""Build the submission PDF solely from docs/portfolio.md.

Dependencies: reportlab, markdown-it-py, fonttools, Pillow, pypdf.
Optional: --output PATH --name NAME --email EMAIL --github URL --period PERIOD.
Unfilled placeholders are omitted; source Markdown remains unchanged.
Detailed documents link to GitHub and are never appended to the PDF.
Apple SD Gothic Neo has CFF outlines unsupported by ReportLab's TTF embedding,
so Korean text uses Noto Sans KR 400/600; English labels use Helvetica Neue.
Set --font-path or PORTFOLIO_KOREAN_FONT to a Noto Sans KR variable TTF.
"""
from __future__ import annotations

import argparse
import html
import os
from pathlib import Path
import re
from urllib.parse import urljoin

from fontTools.ttLib import TTFont as FontToolsFont
from fontTools.varLib.instancer import instantiateVariableFont
from markdown_it import MarkdownIt
from PIL import Image as PILImage
from pypdf import PdfReader
from reportlab.lib import colors
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import BaseDocTemplate, Flowable, Frame, Image, PageBreak, PageTemplate, Paragraph, Spacer, Table, TableStyle

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "docs/portfolio.md"
WEB_SOURCE = "https://github.com/b100to/platform-lab/blob/main/docs/portfolio.md"
FONT_CACHE = ROOT / "tmp/pdfs/fonts"
MD = MarkdownIt("commonmark").enable("table")
INK, NAVY, GRAY, LINE = [colors.HexColor(c) for c in ("#111827", "#1F3A5F", "#6B7280", "#D1D5DB")]
W, H = A4
MARGIN = 15 * mm
CONTENT = W - 2 * MARGIN
ST = {}


def font_setup(path):
    FONT_CACHE.mkdir(parents=True, exist_ok=True)
    for weight, style, name in [(400, "Regular", "KR"), (600, "SemiBold", "KR-Semibold")]:
        target = FONT_CACHE / f"CanonicalPortfolioNotoSansKR-{style}.ttf"
        if not target.exists() or target.stat().st_mtime < path.stat().st_mtime:
            font = FontToolsFont(path)
            if "fvar" in font:
                font = instantiateVariableFont(font, {"wght": weight}, inplace=True)
            names = {1: "Canonical Portfolio Noto Sans KR", 2: style,
                     3: f"CanonicalPortfolioNotoSansKR-{style}",
                     4: f"Canonical Portfolio Noto Sans KR {style}",
                     6: f"CanonicalPortfolioNotoSansKR-{style}",
                     16: "Canonical Portfolio Noto Sans KR", 17: style}
            for record in list(font["name"].names):
                if record.nameID in names:
                    font["name"].setName(names[record.nameID], record.nameID,
                                         record.platformID, record.platEncID, record.langID)
            font.save(target)
        pdfmetrics.registerFont(TTFont(name, str(target)))
    pdfmetrics.registerFontFamily("KR", normal="KR", bold="KR-Semibold", italic="KR", boldItalic="KR-Semibold")
    helvetica = Path("/System/Library/Fonts/HelveticaNeue.ttc")
    for name, index, fallback in [("Latin", 0, "Regular"), ("Latin-Medium", 10, "SemiBold")]:
        if helvetica.exists():
            pdfmetrics.registerFont(TTFont(name, str(helvetica), subfontIndex=index))
        else:
            pdfmetrics.registerFont(TTFont(name, str(FONT_CACHE / f"CanonicalPortfolioNotoSansKR-{fallback}.ttf")))


def setup_styles():
    specs = {
        "body": dict(fontSize=10.3, leading=16.3, spaceAfter=9),
        "title": dict(fontName="KR-Semibold", fontSize=23, leading=32, spaceAfter=17),
        "case": dict(fontName="KR-Semibold", fontSize=21, leading=29, spaceAfter=12),
        "continuation": dict(fontName="KR-Semibold", fontSize=16, leading=24, spaceAfter=18),
        "meta": dict(fontSize=10, leading=15, textColor=GRAY, spaceAfter=18),
        "table": dict(fontSize=10, leading=15),
        "cover_table": dict(fontSize=10, leading=15.5),
        "eyebrow": dict(fontName="Latin-Medium", fontSize=10, leading=15, textColor=NAVY, spaceAfter=12),
    }
    for name, opts in specs.items():
        ST[name] = ParagraphStyle(name, **{"fontName": "KR", "textColor": INK, "wordWrap": "CJK", **opts})


def para(markup, style="body"):
    return Paragraph(markup, ST[style])


def inline(children):
    out = []
    for token in children or []:
        kind = token.type
        if kind in ("text", "code_inline"):
            value = html.escape(token.content)
            out.append(f'<font color="#1F3A5F">{value}</font>' if kind == "code_inline" else value)
        elif kind == "strong_open": out.append("<b>")
        elif kind == "strong_close": out.append("</b>")
        elif kind == "em_open": out.append("<i>")
        elif kind == "em_close": out.append("</i>")
        elif kind in ("softbreak", "hardbreak"):
            out.append("<br/>" if kind == "hardbreak" else " ")
        elif kind == "html_inline" and re.fullmatch(r"<br\s*/?>", token.content): out.append("<br/>")
        elif kind == "link_open":
            target = html.escape(urljoin(WEB_SOURCE, token.attrGet("href")), quote=True)
            out.append(f'<a href="{target}" color="#1F3A5F"><u>')
        elif kind == "link_close": out.append("</u></a>")
    return "".join(out)


class SectionMark(Flowable):
    def __init__(self, key, title):
        super().__init__()
        self.key, self.title = key, title
        self.width = self.height = 0
        self.keepWithNext = True

    def draw(self):
        self.canv.bookmarkPage(self.key)
        self.canv.addOutlineEntry(self.title, self.key, 0, closed=False)
        self.canv._portfolio_chapter = self.title


def table_flow(tokens, start, cover):
    rows, row, i = [], [], start + 1
    while tokens[i].type != "table_close":
        token = tokens[i]
        if token.type == "tr_open": row = []
        elif token.type == "inline":
            markup = inline(token.children)
            if not rows: markup = "<b>" + markup + "</b>"
            row.append(para(markup, "cover_table" if cover else "table"))
        elif token.type == "tr_close": rows.append(row)
        i += 1
    if cover:
        widths = [CONTENT * v for v in (.04, .16, .135, .335, .33)]
        for index in range(1, len(rows)):
            label = rows[index][1].text
            rows[index][1] = para(f'<a href="#case-{index}" color="#1F3A5F">{label}</a>', "cover_table")
    elif len(rows[0]) == 3: widths = [CONTENT * v for v in (.52, .24, .24)]
    else: widths = [CONTENT * v for v in (.2, .8)]
    table = Table(rows, colWidths=widths, repeatRows=1, hAlign="LEFT")
    table.setStyle(TableStyle([
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("LEFTPADDING", (0, 0), (-1, -1), 6), ("RIGHTPADDING", (0, 0), (-1, -1), 6),
        ("TOPPADDING", (0, 0), (-1, -1), 10 if cover else 7),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 10 if cover else 7),
        ("LINEABOVE", (0, 0), (-1, 0), 1, NAVY), ("LINEBELOW", (0, 0), (-1, 0), .7, NAVY),
        ("LINEBELOW", (0, 1), (-1, -1), .4, LINE),
    ]))
    return [table, Spacer(1, 15)], i


def markdown_flow(text, cover=False):
    tokens = MD.parse(text)
    out, i, after_heading = [], 0, False
    while i < len(tokens):
        token = tokens[i]
        if token.type == "heading_open":
            out.append(para(inline(tokens[i + 1].children), "title" if cover else "case"))
            after_heading = True
            i += 2
        elif token.type == "table_open":
            flow, i = table_flow(tokens, i, cover)
            out.extend(flow)
        elif token.type == "paragraph_open":
            content = tokens[i + 1]
            image = next((c for c in content.children or [] if c.type == "image"), None)
            if image:
                path = (SOURCE.parent / image.attrGet("src")).resolve()
                with PILImage.open(path) as bitmap: width, height = bitmap.size
                out.extend([Spacer(1, 3), Image(str(path), width=CONTENT,
                            height=CONTENT * height / width, hAlign="CENTER"), Spacer(1, 15)])
            else:
                if content.content.startswith("**핵심 발견."):
                    out.extend([PageBreak(), SectionMark("case-1-finding", "1. 워크로드 분산 · 핵심 발견"),
                                para("1. 워크로드 분산 · 핵심 발견", "continuation")])
                out.append(para(inline(content.children), "meta" if after_heading else "body"))
            after_heading = False
            i += 2
        i += 1
    return out


class PortfolioDoc(BaseDocTemplate):
    def __init__(self, path, author):
        super().__init__(str(path), pagesize=A4, leftMargin=MARGIN, rightMargin=MARGIN,
                         topMargin=17 * mm, bottomMargin=18 * mm,
                         title="플랫폼 엔지니어링 포트폴리오", author=author or "", allowSplitting=1)
        frame = Frame(MARGIN, 18 * mm, CONTENT, H - 35 * mm,
                      leftPadding=0, rightPadding=0, topPadding=0, bottomPadding=0)
        self.addPageTemplates(PageTemplate(id="portfolio", frames=[frame], onPageEnd=self.footer))

    def footer(self, canvas, doc):
        canvas.saveState()
        canvas.setStrokeColor(LINE)
        canvas.setLineWidth(.4)
        canvas.line(MARGIN, 13 * mm, W - MARGIN, 13 * mm)
        canvas.setFont("KR", 8)
        canvas.setFillColor(GRAY)
        canvas.drawString(MARGIN, 8 * mm, getattr(canvas, "_portfolio_chapter", "플랫폼 엔지니어링 포트폴리오"))
        canvas.setFont("Latin", 8)
        canvas.drawRightString(W - MARGIN, 8 * mm, str(doc.page))
        canvas.restoreState()


def personalize(text, args):
    for key, placeholder in [("name", "이름"), ("email", "이메일"), ("github", "GitHub"), ("period", "기간")]:
        value = getattr(args, key) or ""
        if key == "email" and value: value = f"[{value}](mailto:{value})"
        if key == "github" and value:
            url = value if value.startswith("https://") else "https://github.com/" + value.removeprefix("@")
            value = f"[{value}]({url})"
        text = text.replace(f"`[{placeholder}]`", value).replace(f"[{placeholder}]", value)
    lines = []
    for line in text.splitlines():
        if "·" in line and not line.startswith("|"):
            line = " · ".join(part.strip() for part in line.split("·") if part.strip())
        lines.append(line)
    return "\n".join(lines)


def verify(path):
    reader = PdfReader(path)
    texts = [page.extract_text() for page in reader.pages]
    if len(texts) != 6:
        raise ValueError(f"Expected six submission pages, got {len(texts)}; inspect layout")
    starts = ["플랫폼 엔지니어링 포트폴리오", "1. 워크로드 분산", "핵심 발견", "2. Istio", "3. Authentik SSO", "4. GitOps 재설계"]
    for index, expected in enumerate(starts):
        if expected not in texts[index]: raise ValueError(f"Expected {expected} on page {index + 1}")
    combined = "\n".join(texts)
    if any(value in combined for value in ("[이름]", "[이메일]", "[GitHub]", "[기간]", "\ufffd")):
        raise ValueError("Unfilled placeholder or replacement glyph in PDF")
    image_count = sum(len(page.images) for page in reader.pages)
    if image_count != 5: raise ValueError(f"Expected five source images, got {image_count}")
    links = {"internal": 0, "external": 0}
    for page in reader.pages:
        for ref in page.get("/Annots", []):
            ann = ref.get_object()
            if ann.get("/Subtype") == "/Link":
                links["external" if ann.get("/A", {}).get("/URI") else "internal"] += 1
    print(f"Created {path}: {len(texts)} pages, {image_count} source images, links={links}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=ROOT / "output/pdf/devops-portfolio.pdf")
    parser.add_argument("--font-path", type=Path, default=Path(os.environ.get(
        "PORTFOLIO_KOREAN_FONT", str(Path.home() / "Library/Fonts/NotoSansKR-VariableFont_wght.ttf"))))
    for key in ("name", "email", "github", "period"): parser.add_argument(f"--{key}")
    args = parser.parse_args()
    font_setup(args.font_path)
    setup_styles()
    parts = re.split(r"(?=^## )", personalize(SOURCE.read_text(), args), flags=re.M)
    if len(parts) != 5: raise ValueError("Expected introduction and four cases in docs/portfolio.md")
    story = [SectionMark("overview", "전체 작업"), Spacer(1, 12), para("DEVOPS / PLATFORM ENGINEERING", "eyebrow")]
    story.extend(markdown_flow(parts[0], cover=True))
    for index, section in enumerate(parts[1:], 1):
        title = section.splitlines()[0].removeprefix("## ")
        story.extend([PageBreak(), SectionMark(f"case-{index}", title)])
        story.extend(markdown_flow(section))
    args.output.parent.mkdir(parents=True, exist_ok=True)
    PortfolioDoc(args.output, args.name).build(story)
    verify(args.output)


if __name__ == "__main__": main()
