# 脚本功能：建立书籍的小节索引、文件指纹和中文全文搜索数据库。
# 启动命令：python3 book_access.py index

import json
import logging
import re
import sqlite3
from contextlib import closing, contextmanager
from pathlib import Path

from book_markdown import chapter_records, digest_bytes

# 配置与索引结构
INDEX_NAME = "sections.sqlite"
SCHEMA_VERSION = "1"
MAX_OVERVIEW_KEYWORDS = 12
CACHE_DIR = Path(__file__).resolve().parent / ".agent/work/20261008-book-access/jieba"
SECTION_COLUMNS = "id,title,level,parent,chapter,file,start_line,end_line,own_end_line,page_start,page_end,chars"


def tokenizer():
    """
    输入：无；使用项目内的分词缓存目录。
    输出：已初始化的 jieba 分词器；创建缓存并调整 jieba 日志等级。
    功能：使用现成中文分词，确保缓存保存在本项目目录内。
    """
    import jieba

    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    logging.getLogger("jieba").setLevel(logging.WARNING)
    engine = jieba.Tokenizer()
    engine.tmp_dir = str(CACHE_DIR)
    engine.initialize()
    return engine


def terms(text, engine):
    """
    输入：text，正文或查询；engine，jieba 分词器。
    输出：去除常见停用词后的搜索词列表。
    功能：统一中文和英文的索引与查询规则。
    """
    from jieba.analyse import TFIDF

    return [word.lower() for word in engine.cut_for_search(text) if word.lower() not in TFIDF.STOP_WORDS and any(char.isalnum() for char in word)]


def load_sources(book):
    """
    输入：book，已生成章节 Markdown 的书籍目录。
    输出：转换报告、文件指纹、小节记录及原文字符数。
    功能：从有效转换产物生成可追溯的检索输入。
    """
    report_data = (book / "conversion-report.json").read_bytes()
    report = json.loads(report_data)
    files = {"conversion-report.json": digest_bytes(report_data)}
    records, raw_chars = [], 0
    for chapter in report["sections"]:
        data = (book / chapter["file"]).read_bytes()
        entries, count = chapter_records(data, chapter)
        files[chapter["file"]] = digest_bytes(data)
        records.extend(entries)
        raw_chars += count
    if len({record["id"] for record in records}) != len(records):
        raise ValueError("小节编号重复，无法建立唯一的阅读入口")
    return report, files, records, raw_chars


def write_database(book, context):
    """
    输入：book，输出目录；context，报告、指纹、小节与分词器。
    输出：在一个事务中生成或更新 SQLite 数据库，异常时撤销该事务。
    功能：保存范围索引并建立标题、目录和正文的 FTS5 检索。
    """
    with closing(sqlite3.connect(book / INDEX_NAME)) as connection, connection:
        connection.execute("BEGIN IMMEDIATE")
        connection.execute("DROP TABLE IF EXISTS sections_fts")
        connection.execute("DROP TABLE IF EXISTS sections")
        connection.execute("DROP TABLE IF EXISTS files")
        connection.execute("DROP TABLE IF EXISTS metadata")
        connection.execute("CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL)")
        connection.execute("CREATE TABLE files (path TEXT PRIMARY KEY, digest TEXT NOT NULL)")
        connection.execute("CREATE TABLE sections (id TEXT PRIMARY KEY,title TEXT,level INTEGER,parent TEXT,chapter TEXT,file TEXT,start_line INTEGER,end_line INTEGER,own_end_line INTEGER,page_start INTEGER,page_end INTEGER,chars INTEGER)")
        connection.execute("CREATE VIRTUAL TABLE sections_fts USING fts5(id UNINDEXED,title,outline,body)")
        metadata = {"schema_version": SCHEMA_VERSION, "source_sha256": context["report"]["source_sha256"]}
        connection.executemany("INSERT INTO metadata VALUES (?,?)", metadata.items())
        connection.executemany("INSERT INTO files VALUES (?,?)", context["files"].items())
        insert_records(connection, context)


def insert_records(connection, context):
    """
    输入：connection，数据库事务；context，小节记录与分词器。
    输出：写入所有小节元数据与全文搜索词。
    功能：为目录关系和本节正文建立独立的搜索字段。
    """
    columns = SECTION_COLUMNS.split(",")
    by_id = {record["id"]: record for record in context["records"]}
    placeholders = ",".join("?" for _ in columns)
    for record in context["records"]:
        values = [record[column] for column in columns]
        connection.execute(f"INSERT INTO sections ({SECTION_COLUMNS}) VALUES ({placeholders})", values)
        parent = by_id.get(record["parent"], {})
        outline = by_id[record["chapter"]]["title"] + " " + parent.get("title", "")
        fields = [" ".join(terms(text, context["engine"])) for text in [record["title"], outline, record["body"]]]
        connection.execute("INSERT INTO sections_fts VALUES (?,?,?,?)", [record["id"], *fields])


def write_overview(book, context):
    """
    输入：book，书籍目录；context，转换报告、小节记录和长度统计。
    输出：写入紧凑的全书入口 agent-index.md。
    功能：提供各章标题和原书关键词，供 agent 选择检索范围。
    """
    lines = [f"# {context['report']['title']}", "", "| 章号 | 标题 | 原书关键词 |", "| --- | --- | --- |"]
    for record in context["records"]:
        if record["level"] != 1 or record["id"] == "front":
            continue
        match = re.search(r"本章关键词[：:]([\s\S]+)$", record["body"])
        keyword_text = re.sub(r"\s+", "", match[1]) if match else ""
        keywords = [word for word in re.split(r"[、，,；;。]", keyword_text) if word][:MAX_OVERVIEW_KEYWORDS]
        lines.append(f"| {record['id']} | {record['title']} | {'、'.join(keywords).strip()} |")
    lines += ["", f"正文小节：{sum(record['level'] > 1 for record in context['records'])}；完整来源入口：{len(context['records'])}。", "", "原文字层含 OCR 错误；代码符号和复杂表格需要结合 PDF 核对。", ""]
    (book / "agent-index.md").write_text("\n".join(lines), encoding="utf-8")


def build_index(book):
    """
    输入：book，包含转换报告和章节 Markdown 的目录。
    输出：生成 SQLite 索引与全书入口，返回数量和字符统计。
    功能：在本地建立无需大模型或网络服务的书籍检索入口。
    """
    book = Path(book).resolve()
    report, files, records, raw_chars = load_sources(book)
    context = {"report": report, "files": files, "records": records, "raw_chars": raw_chars, "engine": tokenizer()}
    write_database(book, context)
    write_overview(book, context)
    return {"entries": len(records), "sections": sum(record["level"] > 1 for record in records), "raw_chars": raw_chars, "indexed_chars": sum(record["chars"] for record in records), "index": str(book / INDEX_NAME)}


@contextmanager
def open_index(book):
    """
    输入：book，书籍目录。
    输出：管理只读 SQLite 连接的上下文；指纹不符或索引缺失时抛出异常。
    功能：检查索引与 Markdown 一致，并在成功或异常时关闭连接。
    """
    uri = (book / INDEX_NAME).resolve().as_uri() + "?mode=ro"
    with closing(sqlite3.connect(uri, uri=True)) as connection:
        connection.row_factory = sqlite3.Row
        metadata = dict(connection.execute("SELECT key,value FROM metadata"))
        if metadata["schema_version"] != SCHEMA_VERSION:
            raise ValueError("索引版本不符，请运行 book_access.py index")
        for row in connection.execute("SELECT path,digest FROM files"):
            if digest_bytes((book / row["path"]).read_bytes()) != row["digest"]:
                raise ValueError(f"文件已更新，索引需要重新生成：{row['path']}；运行 book_access.py index")
        yield connection
