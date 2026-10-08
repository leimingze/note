# 脚本功能：使用原书转换产物验证小节检索、阅读预算、来源范围和索引版本检查。
# 启动命令：python3 -m unittest -v test_book_access.py

import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace

from book_index import build_index, open_index
from book_query import list_sections, read_section, search

# 原书与测试过程目录
ROOT = Path(__file__).resolve().parent
BOOK = ROOT / "亿级流量系统架构设计与实战-md"
WORK = ROOT / ".agent/work/20261008-book-access"
WHOLE_SECTION_CHARS = 100000


def reading_options(**changes):
    """
    输入：changes，本次阅读选项的覆盖值。
    输出：完整的阅读参数对象。
    功能：为原文阅读测试统一提供范围和预算。
    """
    values = {"scope": "own", "cursor": 0, "max_chars": WHOLE_SECTION_CHARS, "images": False}
    return SimpleNamespace(**{**values, **changes})


class BookAccessTests(unittest.TestCase):
    def test_catalog_matches_original_outline(self):
        """
        输入：原书章节 Markdown 和当前检索数据库。
        输出：目录数量、父子关系及章节顺序的验证结果。
        功能：确认索引保留原书的全部阅读入口。
        """
        chapters = list_sections(BOOK)
        self.assertEqual([row["id"] for row in chapters], ["front", *map(str, range(1, 14))])
        with open_index(BOOK) as connection:
            count = connection.execute("SELECT count(*) FROM sections WHERE level>1").fetchone()[0]
            row = connection.execute("SELECT * FROM sections WHERE id=?", ("4.3.5",)).fetchone()
        self.assertEqual(count, 314)
        self.assertEqual(row["parent"], "4.3")
        self.assertEqual(row["page_start"], 196)
        self.assertEqual(row["page_end"], 196)
        self.assertLess(len((BOOK / "agent-index.md").read_text(encoding="utf-8")), 4000)

    def test_search_chinese_and_english(self):
        """
        输入：原书中存在的中文及中英文组合问题。
        输出：检索能找到相关原文并遵守结果数量和章号范围。
        功能：验证中文分词和 FTS5 全文检索的可观察行为。
        """
        options = SimpleNamespace(limit=5, chapter=None)
        clocks = search(BOOK, "Snowflake 时钟回拨", options)
        self.assertIn("4.3.5", [row["id"] for row in clocks["results"]])
        cache = search(BOOK, "缓存击穿", SimpleNamespace(limit=3, chapter="2"))
        self.assertTrue(cache["results"])
        self.assertTrue(all(row["chapter"] == "2" for row in cache["results"]))
        self.assertLessEqual(len(cache["results"]), 3)
        self.assertTrue(any("缓存击穿" in row["title"] for row in cache["results"]))
        self.assertTrue(all(len(row["excerpt"]) <= 160 for row in clocks["results"]))
        locks = search(BOOK, "锁", SimpleNamespace(limit=3, chapter=None))
        self.assertTrue(locks["results"])

    def test_parent_scope_and_page_ranges(self):
        """
        输入：第 4 章引言和具有下级标题的小节。
        输出：本节与下级范围互相区分，PDF 页码对应所选范围。
        功能：避免默认读取父标题时连带加载所有下级正文。
        """
        own = read_section(BOOK, "4.3", reading_options())
        subtree = read_section(BOOK, "4.3", reading_options(scope="subtree"))
        self.assertNotIn("### 4.3.5", own["content"])
        self.assertIn("### 4.3.5", subtree["content"])
        self.assertGreater(subtree["total_chars"], own["total_chars"])
        introduction = read_section(BOOK, "4", reading_options())
        self.assertEqual(introduction["pdf_pages"], [181, 181])

    def test_pagination_reconstructs_full_content(self):
        """
        输入：第 4.3.5 节及每次 137 个字符的预算。
        输出：续读可完整重建原文，且每次返回长度符合预算。
        功能：验证字符边界、剩余长度和完成标记。
        """
        whole = read_section(BOOK, "4.3.5", reading_options())
        cursor, pieces = 0, []
        while cursor is not None:
            result = read_section(BOOK, "4.3.5", reading_options(cursor=cursor, max_chars=137))
            self.assertLessEqual(len(result["content"]), 137)
            pieces.append(result["content"])
            cursor = result["next_cursor"]
        self.assertEqual("".join(pieces), whole["content"])
        self.assertEqual(result["remaining_chars"], 0)

    def test_images_and_long_links_are_not_loaded_by_default(self):
        """
        输入：包含扫描原页的第 3 章，以及含配置代码的第 1.4.1 节。
        输出：默认不返回图片；显式选项能获取有效路径；代码继续保留。
        功能：确认阅读节省输入，同时保留原始内容的核对入口。
        """
        options = reading_options(scope="subtree")
        result = read_section(BOOK, "3", options)
        self.assertNotIn("images", result)
        self.assertNotIn("#page=", result["content"])
        self.assertNotIn("![PDF", result["content"])
        images = read_section(BOOK, "3", reading_options(scope="subtree", images=True))["images"]
        self.assertTrue(any("page-0179-" in image for image in images))
        self.assertTrue(all((BOOK / image).is_file() for image in images))
        nginx = read_section(BOOK, "1.4.1", reading_options())
        self.assertIn("worker_processes", nginx["content"])
        self.assertIn("```text", nginx["content"])

    def test_invalid_inputs_are_explicit_errors(self):
        """
        输入：不存在的编号、超出范围的续读位置和非正预算。
        输出：每个无效请求均抛出 ValueError。
        功能：确认阅读失败不会被当成空的成功结果。
        """
        with self.assertRaises(ValueError):
            read_section(BOOK, "99.99", reading_options())
        with self.assertRaises(ValueError):
            read_section(BOOK, "4.3.5", reading_options(cursor=WHOLE_SECTION_CHARS))
        with self.assertRaises(ValueError):
            read_section(BOOK, "4.3.5", reading_options(max_chars=0))
        with self.assertRaises(ValueError):
            search(BOOK, "时钟回拨", SimpleNamespace(limit=0, chapter=None))

    def test_modified_source_requires_reindex(self):
        """
        输入：原书报告与全部章节的独立副本，随后修改副本的 Markdown。
        输出：旧索引明确拒绝读取，重新生成后恢复正常。
        功能：验证文件指纹检查及索引更新，不修改正式原文。
        """
        WORK.mkdir(parents=True, exist_ok=True)
        copied = Path(tempfile.mkdtemp(prefix="fingerprint-", dir=WORK))
        report = json.loads((BOOK / "conversion-report.json").read_text(encoding="utf-8"))
        names = ["conversion-report.json", *[section["file"] for section in report["sections"]]]
        for name in names:
            shutil.copy2(BOOK / name, copied / name)
        build_index(copied)
        path = copied / "第4章 唯一ID生成器.md"
        path.write_text(path.read_text(encoding="utf-8") + "\n", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "索引需要重新生成"):
            read_section(copied, "4.3.5", reading_options())
        build_index(copied)
        self.assertEqual(read_section(copied, "4.3.5", reading_options())["pdf_pages"], [196, 196])

    def test_cli_returns_json_with_bounded_content(self):
        """
        输入：命令行阅读真实小节，每次最多 200 个字符。
        输出：可解析 JSON、正文预算和可用的续读位置。
        功能：验证 agent 调用的完整命令入口。
        """
        process = subprocess.run([sys.executable, str(ROOT / "book_access.py"), "read", "4.3.5", "--max-chars", "200"], capture_output=True, text=True, check=True, timeout=60)
        result = json.loads(process.stdout)
        self.assertEqual(len(result["content"]), 200)
        self.assertEqual(result["next_cursor"], 200)


if __name__ == "__main__":
    unittest.main()
