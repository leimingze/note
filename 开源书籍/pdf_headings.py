# 脚本功能：根据 PDF 书签定位文字层中的章节标题。
# 启动命令：由 python3 pdf_to_md.py 调用。

import re
import unicodedata
from difflib import SequenceMatcher

MATCH_THRESHOLD = 0.64
MAX_TITLE_LINES = 3


def normalized(text):
    """
    输入：text，书签标题或提取的文字。
    输出：去除间隔和标点的比较用字符串。
    功能：容纳 OCR 标点差异，定位原书标题。
    """
    text = unicodedata.normalize("NFKC", text).lower()
    return "".join(char for char in text if char.isalnum())


def locate_headings(lines, bookmarks):
    """
    输入：lines，按阅读顺序排列的文字行；bookmarks，本页的书签。
    输出：标题起点到结束位置及书签的映射；无法定位时抛出异常。
    功能：通过相似度匹配恢复标题，并防止目录文字混入正文。
    """
    found = {}
    cursor = 0
    for bookmark in bookmarks:
        target = normalized(bookmark[1])
        best = (0.0, -1, -1)
        for start in range(cursor, len(lines)):
            if lines[start]["text"].startswith(("◎", "©", "•")):
                continue
            for count in range(1, MAX_TITLE_LINES + 1):
                end = start + count
                candidate = normalized("".join(row["text"] for row in lines[start:end]))
                score = SequenceMatcher(None, target, candidate).ratio()
                if score > best[0]:
                    best = (score, start, end)
        score, start, end = best
        if score < MATCH_THRESHOLD:
            raise ValueError(f"PDF 第 {bookmark[2]} 页无法定位标题：{bookmark[1]}，相似度 {score:.2f}")
        found[start] = (end, bookmark)
        cursor = end
    return found


def filename(title):
    """
    输入：title，章节名称。
    输出：适合本地保存的 Markdown 文件名。
    功能：转换文件系统不允许的字符，并保留中文标题。
    """
    return re.sub(r'[<>:"/\\|?*\x00-\x1f]', "_", title).strip(" .") + ".md"
