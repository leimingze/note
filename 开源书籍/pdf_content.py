# 脚本功能：提取 PDF 正文和插图，生成带标题与段落的 Markdown。
# 启动命令：由 python3 pdf_to_md.py 调用。

import re
from pathlib import Path
from urllib.parse import quote

import pymupdf

from pdf_headings import locate_headings

HEADER_RATIO = 0.085
FOOTER_RATIO = 0.94
BACKGROUND_RATIO = 0.85
ROW_TOLERANCE = 3
PARAGRAPH_INDENT = 12
PARAGRAPH_GAP = 7
IMAGE_PADDING = 2
IMAGE_DPI = 144
INDENT_POINT_WIDTH = 5
CJK_PATTERN = re.compile(r"[\u3400-\u9fff]")
CODE_PATTERN = re.compile(r"[{};=]|^[()\[\],]+$|^(?://|#|func |def |return |import |package |SELECT |CREATE |INSERT )")


def raw_lines(page):
    """
    输入：page，PyMuPDF 页面对象。
    输出：过滤页眉和页脚的文字行。
    功能：保留 PDF 坐标作为阅读顺序和段落依据。
    """
    flags = pymupdf.TEXTFLAGS_DICT & ~pymupdf.TEXT_PRESERVE_IMAGES
    rows = []
    for block in page.get_text("dict", flags=flags)["blocks"]:
        if block["type"] != 0:
            continue
        for line in block["lines"]:
            rect = pymupdf.Rect(line["bbox"])
            text = "".join(span["text"] for span in line["spans"]).strip()
            if text and rect.y0 >= page.rect.height * HEADER_RATIO and rect.y0 < page.rect.height * FOOTER_RATIO:
                rows.append({"text": text, "rect": rect, "columns": False})
    return rows


def text_lines(page):
    """
    输入：page，PyMuPDF 页面对象。
    输出：按阅读顺序合并同一水平行后的文字列表。
    功能：将标题和表格中独立保存的文字按横坐标连接。
    """
    rows = raw_lines(page)
    rows.sort(key=lambda row: (round(row["rect"].y0 / ROW_TOLERANCE), row["rect"].x0))
    merged = []
    for row in rows:
        if merged and abs(merged[-1]["rect"].y0 - row["rect"].y0) < ROW_TOLERANCE:
            previous = merged[-1]
            if row["rect"].x0 < previous["rect"].x0:
                previous["text"] = row["text"] + "    " + previous["text"]
            else:
                previous["text"] += "    " + row["text"]
            previous["rect"] |= row["rect"]
            previous["columns"] = True
        else:
            merged.append(row)
    return merged


def extract_images(page, context):
    """
    输入：page，PDF 页面；context，包含输出目录和页码的配置。
    输出：插图事件列表；将原图区域保存为 PNG 文件。
    功能：保留插图；有文字层时排除扫描背景，无文字层时保留原页。
    """
    events = []
    for index, info in enumerate(page.get_image_info(), 1):
        rect = pymupdf.Rect(info["bbox"]) & page.rect
        background = rect.get_area() / page.rect.get_area() >= BACKGROUND_RATIO
        if rect.is_empty or (background and context["has_text"]):
            continue
        relative = Path("assets") / f"page-{context['page']:04d}-image-{index:02d}.png"
        clip = (rect + (-IMAGE_PADDING, -IMAGE_PADDING, IMAGE_PADDING, IMAGE_PADDING)) & page.rect
        page.get_pixmap(clip=clip, dpi=IMAGE_DPI).save(context["output"] / relative)
        events.append({"y": rect.y0, "rect": rect, "markdown": f"![PDF 第 {context['page']} 页插图]({relative.as_posix()})"})
    return events


def code_line(text):
    """
    输入：text，提取的一行文字。
    输出：是否具有代码或配置语句特征。
    功能：将代码行保留为等宽文本，避免连接成正文。
    """
    if text.startswith(("//", "#")):
        return True
    return not CJK_PATTERN.search(text) and bool(CODE_PATTERN.search(text))


def paragraph_layout(row, context):
    """
    输入：row，当前文字行；context，段落、行距和缩进状态。
    输出：本行显示文字、段落起点判断及列表左边界。
    功能：区分列表续行和新段落，保留原文的列表结构。
    """
    text, rect = row["text"], row["rect"]
    bullet = text.startswith(("◎", "©", "•"))
    in_list = context["paragraph"] and context["paragraph"][0].startswith("- ") and context["item_left"] is not None
    starts = bullet or (rect.x0 < context["item_left"] + PARAGRAPH_INDENT if in_list else rect.x0 > context["margin"] + PARAGRAPH_INDENT)
    previous = context["previous"]
    gap = previous is not None and rect.y0 - previous.y1 > PARAGRAPH_GAP
    item_left = rect.x0 if bullet else context["item_left"]
    return {"text": "- " + text[1:].strip() if bullet else text, "starts": starts or gap, "item_left": item_left}


def format_groups(paragraph, literal):
    """
    输入：paragraph，正文行列表；literal，等宽文本行列表。
    输出：对应的 Markdown 段落列表。
    功能：统一生成正文和等宽代码块的格式。
    """
    groups = []
    if paragraph:
        groups.append("".join(paragraph))
    if literal:
        groups.append("```text\n" + "\n".join(literal) + "\n```")
    return groups


def format_rows(rows):
    """
    输入：rows，一段连续正文的文字行。
    输出：正文、列表、等宽代码与表格行组成的 Markdown。
    功能：按缩进和行距连接换行，同时保留代码和多列内容。
    """
    if not rows:
        return ""
    margin = min(row["rect"].x0 for row in rows)
    output, paragraph, literal = [], [], []
    previous, item_left = None, None
    for row in rows:
        text, rect = row["text"], row["rect"]
        is_literal = code_line(text) or row["columns"]
        if is_literal:
            if paragraph:
                output.append("".join(paragraph))
                paragraph = []
            indent = max(0, round((rect.x0 - margin) / INDENT_POINT_WIDTH))
            literal.append(" " * indent + text)
        else:
            if literal:
                output.append("```text\n" + "\n".join(literal) + "\n```")
                literal = []
            layout = paragraph_layout(row, {"paragraph": paragraph, "item_left": item_left, "margin": margin, "previous": previous})
            if paragraph and layout["starts"]:
                output.append("".join(paragraph))
                paragraph = []
            paragraph.append(layout["text"])
            item_left = layout["item_left"]
        previous = rect
    output.extend(format_groups(paragraph, literal))
    return "\n\n".join(output)


def page_events(rows, headings, images):
    """
    输入：rows，文字行；headings，标题位置；images，插图事件。
    输出：按纵坐标排列的正文、标题和插图事件列表。
    功能：去除插图区域的重复文字，保留正文标题顺序。
    """
    events = list(images)
    index = 0
    while index < len(rows):
        if index in headings:
            end, (level, title, _) = headings[index]
            events.append({"y": rows[index]["rect"].y0, "markdown": "#" * level + " " + title})
            index = end
            continue
        row = rows[index]
        inside = any(image["rect"].contains(row["rect"]) for image in images)
        if not inside:
            events.append({"y": row["rect"].y0, "row": row})
        index += 1
    events.sort(key=lambda event: event["y"])
    return events


def render_page(page, context):
    """
    输入：page，PDF 页面；context，包含书签、输出路径、原文链接的配置。
    输出：本页 Markdown 和图片数量；保存本页插图；文字缺失时抛出异常。
    功能：组织标题、正文和插图，并保留可核对的原 PDF 页码。
    """
    rows = context["rows"]
    images = extract_images(page, context)
    if not rows and not images and page.get_text().strip():
        raise ValueError(f"PDF 第 {context['page']} 页的正文全部被页边距过滤")
    headings = locate_headings(rows, context["bookmarks"])
    events = page_events(rows, headings, images)
    link = quote(context["source"], safe="/.")
    chunks = [f"[原 PDF 第 {context['page']} 页]({link}#page={context['page']})"]
    pending = []
    for event in events:
        if "row" in event:
            pending.append(event["row"])
            continue
        if pending:
            chunks.append(format_rows(pending))
            pending = []
        chunks.append(event["markdown"])
    if pending:
        chunks.append(format_rows(pending))
    return "\n\n".join(chunks), len(images)
