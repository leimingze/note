# 脚本功能：将 PDF 按章转换为 Markdown、导出插图并生成 agent 检索索引。
# 启动命令：
# python3 -m pip install -r requirements.txt
# python3 pdf_to_md.py '亿级流量系统架构设计与实战 (--) (manongshu.com).pdf'
# python3 pdf_to_md.py book.pdf --output book-md
# python3 pdf_to_md.py book.pdf --output book-md --overwrite

import argparse
import hashlib
import json
import os
import re
from collections import defaultdict
from pathlib import Path
from urllib.parse import quote

import pymupdf

from book_index import build_index
from pdf_content import render_page, text_lines
from pdf_headings import filename, locate_headings

CHAPTER_PATTERN = re.compile(r"^第\s*\d+\s*章")
PROGRESS_INTERVAL = 25
HASH_CHUNK_BYTES = 1024 * 1024


def validate_chapters(chapters, total_pages):
    """
    输入：chapters，章节书签列表；total_pages，PDF 总页数。
    输出：已验证的章节起点页码；缺少章节或页码错误时抛出异常。
    功能：确保章节页码能组成连续且无重叠的范围。
    """
    if not chapters:
        raise ValueError("PDF 缺少中文章节书签，无法确定章节范围")
    starts = [entry[2] for entry in chapters]
    if starts != sorted(set(starts)) or starts[0] < 1 or starts[-1] > total_pages:
        raise ValueError("章节书签页码重复、顺序错误或超出 PDF 范围")
    return starts


def chapter_ranges(document):
    """
    输入：document，含中文章节书签的 PDF。
    输出：每章标题及起止 PDF 页码；书签无效时抛出异常。
    功能：按书签确定章节范围，额外保留书前资料。
    """
    toc = document.get_toc()
    chapters = [entry for entry in toc if entry[0] == 1 and CHAPTER_PATTERN.match(entry[1])]
    starts = validate_chapters(chapters, len(document))
    sections = []
    if starts[0] > 1:
        sections.append({"title": "书前资料", "start": 1, "end": starts[0] - 1, "file": "00-书前资料.md"})
    for index, (_, title, start) in enumerate(chapters):
        end = chapters[index + 1][2] - 1 if index + 1 < len(chapters) else len(document)
        sections.append({"title": title, "start": start, "end": end, "file": filename(title)})
    return toc, sections


def check_text_layer(document):
    """
    输入：document，打开的 PDF。
    输出：无文字且含图片的页码列表；整本没有文字层时抛出异常。
    功能：记录需保留原页图片的页面，并拒绝将整本扫描件当作文字转换。
    """
    missing = [index + 1 for index, page in enumerate(document) if not page.get_text().strip() and page.get_images()]
    if len(missing) == len(document):
        raise ValueError("整本 PDF 没有文字层，需要先执行本地 OCR")
    return missing


def write_index(context):
    """
    输入：context，转换报告与输出路径。
    输出：写入 Markdown 目录和 JSON 转换报告。
    功能：提供章节入口、来源指纹和转换统计。
    """
    report, output = context["report"], context["output"]
    lines = [f"# {report['title']}", "", "本地文字层提取，未调用大模型。保留原文字层中的 OCR 识别结果，代码中的符号和复杂表格需要结合原 PDF 核对。", "", "## 目录", ""]
    for section in report["sections"]:
        lines.append(f"- [{section['title']}]({quote(section['file'])})：PDF 第 {section['start']}–{section['end']} 页")
    lines += ["", "## Agent 阅读", "", "[全书入口](agent-index.md)包含各章标题和关键词。查询命令及续读规则见[目录规则](../AGENTS.md)。转换完成后自动生成小节索引；仅更新 Markdown 时运行 `python3 book_access.py index`。", ""]
    lines += ["", "## 转换信息", "", f"- 页数：{report['pages']}", f"- 章节：{report['chapters']}", f"- 已定位正文标题：{report['headings']}", f"- 图片：{report['images']}", f"- 无文字层、保留为原页图片的页码：{report['image_only_pages']}", "- 多列文字使用等宽文本保留，未推断表格结构。", "- 每页提供原 PDF 链接；文件移动后需要一同保留原 PDF 的相对位置。", ""]
    (output / "README.md").write_text("\n".join(lines), encoding="utf-8")
    (output / "conversion-report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def validate_output(output, overwrite, digest):
    """
    输入：output，输出目录；overwrite，是否更新已有结果；digest，输入文件指纹。
    输出：目录符合要求时返回；目录已存在或来源不同则抛出异常。
    功能：限制更新操作只作用于同一输入文件的转换目录。
    """
    if not output.exists():
        return
    if not overwrite:
        raise FileExistsError(f"输出目录已存在，不覆盖已有文件：{output}")
    old_report = json.loads((output / "conversion-report.json").read_text(encoding="utf-8"))
    if old_report["source_sha256"] != digest:
        raise ValueError("已有结果的来源 PDF 与本次输入不同，请指定新的输出目录")


def prepare_bookmarks(document, toc):
    """
    输入：document，PDF；toc，原始书签。
    输出：每页书签与正文行的字典；任何标题无法定位时抛出异常。
    功能：在生成文件前检查全部标题，防止产生内容不完整的结果。
    """
    bookmarks = defaultdict(list)
    for entry in toc:
        if entry[0] > 1 or CHAPTER_PATTERN.match(entry[1]):
            bookmarks[entry[2]].append(entry)
    rows = {number: text_lines(document[number - 1]) for number in range(1, len(document) + 1)}
    for number, entries in bookmarks.items():
        locate_headings(rows[number], entries)
    return bookmarks, rows


def build_report(document, context):
    """
    输入：document，PDF；context，来源、指纹、章节、书签和图片数量。
    输出：可保存为 JSON 的转换报告。
    功能：记录来源版本和生成统计，供目录生成和验证使用。
    """
    return {"title": re.sub(r"\s*\(.*", "", context["source"].stem), "source": str(context["source"]), "source_sha256": context["digest"], "pymupdf_version": pymupdf.VersionBind, "pages": len(document), "chapters": sum(CHAPTER_PATTERN.match(section["title"]) is not None for section in context["sections"]), "headings": sum(len(entries) for entries in context["bookmarks"].values()), "images": context["images"], "image_only_pages": context["missing"], "sections": context["sections"]}


def write_section(document, section, context):
    """
    输入：document，PDF；section，章节范围；context，输出目录、原文及每页数据。
    输出：写入一个章节 Markdown 与插图，返回本章图片数量。
    功能：按 PDF 页码依次生成正文，并显示整本转换进度。
    """
    image_count = 0
    path = context["output"] / section["file"]
    with path.open("w", encoding="utf-8") as stream:
        for number in range(section["start"], section["end"] + 1):
            page_context = {"page": number, "output": context["output"], "source": context["source"], "bookmarks": context["bookmarks"][number], "rows": context["rows"][number], "has_text": number not in context["missing"]}
            markdown, images = render_page(document[number - 1], page_context)
            stream.write(markdown + ("\n" if number == section["end"] else "\n\n"))
            image_count += images
            if number % PROGRESS_INTERVAL == 0:
                print(f"已转换 {number}/{len(document)} 页", flush=True)
    print(f"已生成：{section['file']}", flush=True)
    return image_count


def convert(options):
    """
    输入：options，包含输入 PDF 和新输出目录的命令行配置。
    输出：创建章节 Markdown、插图、报告和检索索引；指定 --overwrite 可更新同一来源的输出。
    功能：执行不调用网络服务的整本书转换，并明确检查文字层。
    """
    source = options.pdf.resolve()
    output = options.output.resolve() if options.output else source.with_suffix("")
    with pymupdf.open(source) as document:
        toc, sections = chapter_ranges(document)
        missing = check_text_layer(document)
        digest = source_digest(source)
        validate_output(output, options.overwrite, digest)
        bookmarks, rows = prepare_bookmarks(document, toc)
        output.mkdir(parents=True, exist_ok=options.overwrite)
        (output / "assets").mkdir(exist_ok=options.overwrite)
        image_count = 0
        context = {"output": output, "source": os.path.relpath(source, output), "bookmarks": bookmarks, "rows": rows, "missing": missing}
        for section in sections:
            image_count += write_section(document, section, context)
        report = build_report(document, {"source": source, "digest": digest, "sections": sections, "bookmarks": bookmarks, "images": image_count, "missing": missing})
        write_index({"report": report, "output": output})
    build_index(output)
    print(f"转换完成：{output}")


def source_digest(source):
    """
    输入：source，原 PDF 路径。
    输出：文件内容的 SHA-256 指纹。
    功能：以分块读取控制内存占用并记录输入版本。
    """
    digest = hashlib.sha256()
    with source.open("rb") as stream:
        for chunk in iter(lambda: stream.read(HASH_CHUNK_BYTES), b""):
            digest.update(chunk)
    return digest.hexdigest()


def main():
    """
    输入：命令行 PDF 路径、可选 --output 目录和 --overwrite 更新选项。
    输出：执行转换；失败时显示原始异常并返回非零状态。
    功能：提供可以重复使用的本地命令行入口。
    """
    parser = argparse.ArgumentParser(description="使用文字层和章节书签在本地转换 PDF，不调用大模型")
    parser.add_argument("pdf", type=Path, help="输入 PDF")
    parser.add_argument("--output", type=Path, help="输出目录，更新已有结果时需指定 --overwrite")
    parser.add_argument("--overwrite", action="store_true", help="更新同一来源 PDF 的已有转换结果")
    convert(parser.parse_args())


if __name__ == "__main__":
    main()
