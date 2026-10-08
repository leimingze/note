# 脚本功能：使用本项目原 PDF 与转换产物，验证页码、标题、图片及输出保护行为。
# 启动命令：python3 -m unittest test_pdf_to_md.py

import json
import re
import unittest
from pathlib import Path
from types import SimpleNamespace
from urllib.parse import unquote

import pymupdf

from pdf_content import text_lines
from pdf_headings import locate_headings
from pdf_to_md import chapter_ranges, convert, source_digest

ROOT = Path(__file__).resolve().parent
SOURCE = ROOT / "亿级流量系统架构设计与实战 (--) (manongshu.com).pdf"
OUTPUT = ROOT / "亿级流量系统架构设计与实战-md"


class ConversionTests(unittest.TestCase):
    def test_chapter_coverage_and_headings(self):
        """
        输入：本项目原 PDF 及生成的 Markdown。
        输出：所有页码与书签标题是否完整的测试结果。
        功能：确认章节范围连续，标题未遗漏或重复。
        """
        with pymupdf.open(SOURCE) as document:
            toc, sections = chapter_ranges(document)
            covered, headings = [], []
            for section in sections:
                text = (OUTPUT / section["file"]).read_text(encoding="utf-8")
                pages = [int(number) for number in re.findall(r"\[原 PDF 第 (\d+) 页\]", text)]
                self.assertEqual(pages, list(range(section["start"], section["end"] + 1)))
                covered.extend(pages)
                headings.extend(re.findall(r"^#{1,3} (.+)$", text, re.MULTILINE))
                self.assertEqual(text.count("```") % 2, 0)
            self.assertEqual(covered, list(range(1, len(document) + 1)))
            expected = [entry[1] for entry in toc if entry[0] > 1 or entry[1].startswith("第")]
            self.assertEqual(headings, expected)

    def test_assets_and_source_links(self):
        """
        输入：转换目录中的 Markdown、PNG 和 JSON 报告。
        输出：图片有效性、来源链接及输入指纹的测试结果。
        功能：检查全部引用可用，确保无文字层页面以原图保留。
        """
        report = json.loads((OUTPUT / "conversion-report.json").read_text(encoding="utf-8"))
        images = []
        for section in report["sections"]:
            text = (OUTPUT / section["file"]).read_text(encoding="utf-8")
            images.extend(re.findall(r"!\[[^\]]*\]\((assets/[^)]+)\)", text))
            links = re.findall(r"\[原 PDF 第 \d+ 页\]\(([^#]+)#page=\d+\)", text)
            for link in links:
                self.assertEqual((OUTPUT / unquote(link)).resolve(), SOURCE)
        self.assertEqual(len(images), report["images"])
        self.assertEqual(set(images), {path.relative_to(OUTPUT).as_posix() for path in (OUTPUT / "assets").glob("*.png")})
        for relative in images:
            pixmap = pymupdf.Pixmap(OUTPUT / relative)
            self.assertGreater(pixmap.width, 0)
            self.assertGreater(pixmap.height, 0)
        for number in report["image_only_pages"]:
            self.assertTrue(any(f"page-{number:04d}-" in name for name in images))
        self.assertEqual(report["source_sha256"], source_digest(SOURCE))

    def test_real_ocr_title_and_list_continuation(self):
        """
        输入：原书标题分为多个文字段的页面及已生成的列表。
        输出：标题合并及列表续行的测试结果。
        功能：覆盖已确认的 PDF 文字顺序和段落连接问题。
        """
        with pymupdf.open(SOURCE) as document:
            rows = text_lines(document[125])
            entries = [entry for entry in document.get_toc() if entry[2] == 126]
            titles = [entry[1] for _, entry in locate_headings(rows, entries).values()]
            self.assertEqual(titles, [entry[1] for entry in entries])
            rows = text_lines(document[284])
            entries = [entry for entry in document.get_toc() if entry[2] == 285]
            found = locate_headings(rows, entries)
            self.assertTrue(all(not rows[start]["text"].startswith("◎") for start in found))
        text = (OUTPUT / "第1章 大型互联网公司的基础架构.md").read_text(encoding="utf-8")
        self.assertIn("目的是让读者了解客户端用户请求是如何进入机房", text)
        self.assertIn("则将是一件非常困难的事情。因为它要求公司：", text)

    def test_existing_output_protection(self):
        """
        输入：原 PDF 及已经生成的目录，不指定覆盖选项。
        输出：确认转换会抛出 FileExistsError。
        功能：防止重复执行命令时覆盖已有文件。
        """
        options = SimpleNamespace(pdf=SOURCE, output=OUTPUT, overwrite=False)
        with self.assertRaises(FileExistsError):
            convert(options)


if __name__ == "__main__":
    unittest.main()
