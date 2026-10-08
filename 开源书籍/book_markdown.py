# 脚本功能：用 Markdown 解析器提取书籍标题、小节范围、页码和图片引用。
# 启动命令：由 python3 book_access.py index 调用。

import hashlib
import re

from markdown_it import MarkdownIt

# 标题与来源标记
CHAPTER_ID = re.compile(r"^第\s*(\d+)\s*章")
SECTION_ID = re.compile(r"^(\d+(?:\.\d+)+)\s")
PAGE_LABEL = re.compile(r"原 PDF 第 (\d+) 页")


def digest_bytes(data):
    """
    输入：data，文件的原始字节。
    输出：SHA-256 指纹字符串。
    功能：记录 Markdown 版本，供索引有效性检查使用。
    """
    return hashlib.sha256(data).hexdigest()


def page_number(token):
    """
    输入：token，Markdown inline 节点。
    输出：独立 PDF 来源链接的页码，其他节点返回 None。
    功能：从解析器节点识别来源标记，保留代码和普通链接。
    """
    children = token.children or []
    if [child.type for child in children] != ["link_open", "text", "link_close"]:
        return None
    match = PAGE_LABEL.fullmatch(children[1].content)
    if match and children[0].attrGet("href").endswith(f"#page={match[1]}"):
        return int(match[1])
    return None


def image_nodes(token):
    """
    输入：token，Markdown inline 节点。
    输出：本节点中的图片行号和路径列表。
    功能：通过解析器收集图片引用，保持正文与媒体入口独立。
    """
    return [{"line": token.map[0], "path": child.attrGet("src")} for child in (token.children or []) if child.type == "image"]


def parse_document(data, first_page):
    """
    输入：data，Markdown 字节；first_page，该章节的起始 PDF 页码。
    输出：包含原文行、解析节点、来源页码和过滤范围的字典。
    功能：一次解析章节，为索引及定向读取提供相同的正文规则。
    """
    text = data.decode("utf-8")
    lines = text.splitlines(keepends=True)
    tokens = MarkdownIt("commonmark").parse(text)
    markers, excluded, images = {}, set(), []
    for token in tokens:
        if token.type != "inline":
            continue
        number = page_number(token)
        if number is not None:
            markers[token.map[0]] = number
            excluded.update(range(*token.map))
        children = token.children or []
        images.extend(image_nodes(token))
        if children and all(child.type in {"image", "softbreak"} for child in children):
            excluded.update(range(*token.map))
    pages, current = [], first_page
    for index in range(len(lines)):
        current = markers.get(index, current)
        pages.append(current)
    return {"lines": lines, "tokens": tokens, "excluded": excluded, "pages": pages, "images": images}


def clean_region(document, start, end):
    """
    输入：document，已解析的章节；start、end，零起点的半开行范围。
    输出：移除来源 URL 和独立图片节点后的 Markdown 正文。
    功能：控制 agent 输入长度，同时保留标题、正文、表格和代码。
    """
    return "".join(document["lines"][index] for index in range(start, end) if index not in document["excluded"]).strip()


def region_pages(document, start, end):
    """
    输入：document，章节解析结果；start、end，零起点的半开行范围。
    输出：该范围内正文及图片对应的 PDF 起止页码。
    功能：排除仅有来源链接的页面，同时保留纯图片页面的来源。
    """
    image_lines = {image["line"] for image in document["images"]}
    substantive = [index for index in range(start, end) if document["lines"][index].strip() and (index not in document["excluded"] or index in image_lines)]
    pages = [document["pages"][index] for index in substantive]
    return [min(pages), max(pages)]


def headings(document, chapter):
    """
    输入：document，已解析的章节；chapter，来自转换报告的章节信息。
    输出：带编号、层级、父编号和起始行的标题列表。
    功能：使用解析器识别标题，避免将代码块中的井号当作目录。
    """
    result, parents = [], {}
    tokens = document["tokens"]
    for index, token in enumerate(tokens):
        if token.type != "heading_open":
            continue
        level, title = int(token.tag[1:]), tokens[index + 1].content
        match = CHAPTER_ID.match(title) if level == 1 else SECTION_ID.match(title)
        if match is None:
            raise ValueError(f"无法识别标题编号：{chapter['file']}，{title}")
        identifier = match[1]
        result.append({"id": identifier, "title": title, "level": level, "start": token.map[0], "parent": parents.get(level - 1)})
        parents[level] = identifier
    if not result:
        result.append({"id": "front", "title": chapter["title"], "level": 1, "start": 0, "parent": None})
    return result


def section_record(document, context):
    """
    输入：document，章节解析结果；context，当前标题、结束范围和章节信息。
    输出：带行范围、页码、长度和正文的小节记录。
    功能：分别记录本节正文与包含下级小节的阅读范围。
    """
    heading, chapter = context["heading"], context["chapter"]
    start, end, own_end = heading["start"], context["end"], context["own_end"]
    pages = region_pages(document, start, end)
    body = clean_region(document, start, own_end)
    return {"id": heading["id"], "title": heading["title"], "level": heading["level"], "parent": heading["parent"], "chapter": context["chapter_id"], "file": chapter["file"], "start_line": start + 1, "end_line": end, "own_end_line": own_end, "page_start": min(pages), "page_end": max(pages), "chars": len(body), "body": body}


def chapter_records(data, chapter):
    """
    输入：data，章节 Markdown 字节；chapter，转换报告的章节信息。
    输出：本章节所有阅读入口及原文字符数。
    功能：建立不重复保存正文的范围索引，并保留父子关系。
    """
    document = parse_document(data, chapter["start"])
    titles = headings(document, chapter)
    records, total_lines = [], len(document["lines"])
    for index, heading in enumerate(titles):
        following = titles[index + 1:]
        own_end = following[0]["start"] if following else total_lines
        end = next((item["start"] for item in following if item["level"] <= heading["level"]), total_lines)
        context = {"heading": heading, "chapter": chapter, "chapter_id": titles[0]["id"], "own_end": own_end, "end": end}
        records.append(section_record(document, context))
    return records, len(data.decode("utf-8"))
