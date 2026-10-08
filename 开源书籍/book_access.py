# 脚本功能：提供 agent 使用的书籍索引、目录、搜索及定向阅读命令。
# 启动命令：
# python3 -m pip install -r requirements.txt
# python3 book_access.py index
# python3 book_access.py sections 4
# python3 book_access.py search 'Snowflake 时钟回拨' --limit 5
# python3 book_access.py read 4.3.5 --max-chars 3000 --cursor 0
# python3 book_access.py --book book-md read 4.3 --scope subtree --images

import argparse
import json
from pathlib import Path

from book_index import build_index
from book_query import DEFAULT_LIMIT, DEFAULT_MAX_CHARS, list_sections, read_section, search

DEFAULT_BOOK = Path(__file__).resolve().parent / "亿级流量系统架构设计与实战-md"


def argument_parser():
    """
    输入：无。
    输出：配置完成的命令行参数解析器。
    功能：提供独立的索引、目录、搜索和阅读入口及明确预算选项。
    """
    parser = argparse.ArgumentParser(description="本地书籍检索与按需阅读")
    parser.add_argument("--book", type=Path, default=DEFAULT_BOOK, help="转换后的书籍目录")
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("index", help="生成或更新索引")
    commands.add_parser("chapters", help="列出全书章节")
    sections = commands.add_parser("sections", help="列出一章的小节")
    sections.add_argument("chapter", help="章号，如 4")
    searching = commands.add_parser("search", help="搜索标题与正文")
    searching.add_argument("query")
    searching.add_argument("--limit", type=int, default=DEFAULT_LIMIT)
    searching.add_argument("--chapter", help="仅搜索指定章")
    reading = commands.add_parser("read", help="读取指定编号的正文")
    reading.add_argument("identifier", help="如 4.3.5；章号读取本章引言，front 读取书前资料")
    reading.add_argument("--scope", choices=["own", "subtree"], default="own", help="本节正文或包含下级小节")
    reading.add_argument("--max-chars", type=int, default=DEFAULT_MAX_CHARS, help="本次正文字符上限，不等同于 token 数")
    reading.add_argument("--cursor", type=int, default=0, help="上一次返回的 next_cursor")
    reading.add_argument("--images", action="store_true", help="同时返回本阅读范围的图片路径")
    return parser


def main():
    """
    输入：命令行子命令、书籍目录及检索或阅读参数。
    输出：打印紧凑的 JSON 结果；错误原样暴露并返回非零状态。
    功能：让 agent 通过本地工具逐步获取需要的原文。
    """
    options = argument_parser().parse_args()
    book = options.book.resolve()
    if options.command == "index":
        result = build_index(book)
    elif options.command == "chapters":
        result = list_sections(book)
    elif options.command == "sections":
        result = list_sections(book, options.chapter)
    elif options.command == "search":
        result = search(book, options.query, options)
    else:
        result = read_section(book, options.identifier, options)
    print(json.dumps(result, ensure_ascii=False, separators=(",", ":")))


if __name__ == "__main__":
    main()
