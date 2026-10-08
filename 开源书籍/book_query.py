# 脚本功能：提供书籍的小节搜索、目录查询和带续读位置的正文读取。
# 启动命令：python3 book_access.py search 'Snowflake 时钟回拨'
# python3 book_access.py read 4.3.5 --max-chars 3000

import re

from book_index import open_index, terms, tokenizer
from book_markdown import clean_region, parse_document, region_pages

# 输出预算
DEFAULT_LIMIT = 5
DEFAULT_MAX_CHARS = 3000
EXCERPT_CHARS = 160


def list_sections(book, chapter=None):
    """
    输入：book，书籍目录；chapter，可选章号。
    输出：章级目录或指定章的全部小节元数据。
    功能：仅返回目录信息，避免加载完整正文。
    """
    with open_index(book) as connection:
        if chapter is None:
            rows = connection.execute("SELECT id,title,chars,page_start,page_end FROM sections WHERE level=1 ORDER BY CAST(chapter AS INTEGER)").fetchall()
        else:
            rows = connection.execute("SELECT id,title,parent,chars,page_start,page_end FROM sections WHERE chapter=? ORDER BY start_line", (chapter,)).fetchall()
            if not rows:
                raise ValueError(f"章号不存在：{chapter}")
    return [dict(row) for row in rows]


def read_region(book, record, scope):
    """
    输入：book，书籍目录；record，小节元数据；scope，本节或含下级范围。
    输出：过滤后的正文、原文图片路径列表和范围对应的 PDF 页码。
    功能：按索引范围读取来源，保持正文与检索索引一致。
    """
    data = (book / record["file"]).read_bytes()
    document = parse_document(data, record["page_start"])
    start = record["start_line"] - 1
    end = record["end_line"] if scope == "subtree" else record["own_end_line"]
    content = clean_region(document, start, end)
    images = [image["path"] for image in document["images"] if start <= image["line"] < end]
    return content, images, region_pages(document, start, end)


def read_section(book, identifier, options):
    """
    输入：book，书籍目录；identifier，小节编号；options，范围、字符预算、续读位置和图片选项。
    输出：正文片段、总字符数、续读位置与来源；无效输入抛出异常。
    功能：按预算读取原文，明确显示剩余内容，图片仅返回路径。
    """
    with open_index(book) as connection:
        record = connection.execute("SELECT * FROM sections WHERE id=?", (identifier,)).fetchone()
    if record is None:
        raise ValueError(f"小节编号不存在：{identifier}")
    if options.scope not in {"own", "subtree"}:
        raise ValueError("scope 必须是 own 或 subtree")
    content, images, pages = read_region(book, record, options.scope)
    if options.cursor < 0 or options.cursor > len(content) or options.max_chars <= 0:
        raise ValueError("cursor 必须位于正文范围内，max-chars 必须大于零")
    end = min(options.cursor + options.max_chars, len(content))
    result = {"id": identifier, "title": record["title"], "scope": options.scope, "pdf_pages": pages, "file": record["file"], "total_chars": len(content), "cursor": options.cursor, "next_cursor": end if end < len(content) else None, "remaining_chars": len(content) - end, "content": content[options.cursor:end]}
    if options.images:
        result["images"] = images
    return result


def excerpt(content, query_terms):
    """
    输入：content，本节原文；query_terms，查询词列表。
    输出：查询词附近、具有字符上限的原文摘录。
    功能：帮助选择阅读入口，保留摘录的原始文字。
    """
    compact = re.sub(r"\s+", " ", content)
    positions = [compact.lower().find(term) for term in query_terms]
    position = min((value for value in positions if value >= 0), default=0)
    start = max(0, position - EXCERPT_CHARS // 4)
    return compact[start:start + EXCERPT_CHARS]


def search(book, query, options):
    """
    输入：book，书籍目录；query，查询文字；options，结果数量和可选章号。
    输出：按 BM25 排序的小节标题、页码与短摘录。
    功能：执行参数化的本地中文检索，只返回阅读候选。
    """
    query_terms = list(dict.fromkeys(terms(query, tokenizer())))
    if not query_terms or options.limit <= 0:
        raise ValueError("查询需要包含有效关键词，limit 必须大于零")
    expression = " OR ".join('"' + term.replace('"', '""') + '"' for term in query_terms)
    clause, values = (" AND s.chapter=?", [options.chapter]) if options.chapter else ("", [])
    sql = "SELECT s.*,bm25(sections_fts,0,8,2,1) AS rank FROM sections_fts JOIN sections s ON s.id=sections_fts.id WHERE sections_fts MATCH ?" + clause + " ORDER BY rank,s.rowid LIMIT ?"
    with open_index(book) as connection:
        rows = connection.execute(sql, [expression, *values, options.limit]).fetchall()
    results = []
    for row in rows:
        body, _, pages = read_region(book, row, "own")
        results.append({"id": row["id"], "title": row["title"], "chapter": row["chapter"], "pdf_pages": pages, "chars": row["chars"], "excerpt": excerpt(body, query_terms)})
    return {"query": query, "terms": query_terms, "results": results}
