"""
脚本功能：
根据 sources/ 下的 Markdown 源文件生成 Redis 阅读型 Notebook，每份只包含八股问答和
面试话术/自查清单两个 Markdown 单元格，不生成练习或执行输出。

启动命令：
cd redis && python3 scripts/build_notebooks.py
"""

from pathlib import Path

import nbformat as nbf


ROOT = Path(__file__).resolve().parent.parent
SOURCE_DIR = ROOT / "sources"
NOTEBOOK_DIR = ROOT / "notebooks"

NOTEBOOKS = [
    "01-基础",
    "02-数据结构",
    "03-持久化",
    "04-功能与淘汰",
    "05-高可用",
    "06-缓存",
]


def heading_matches(line: str, heading: str) -> bool:
    """
    输入：line，单行 Markdown；heading，目标标题。
    输出：目标标题或带冒号、括号后缀时返回 True。
    功能：兼容带说明文字的章节标题。
    """
    normalized = line.strip()
    return normalized == heading or normalized.startswith(
        (f"{heading}：", f"{heading}:", f"{heading}（")
    )


def extract_section(text: str, start: str, end: str | None) -> str:
    """
    输入：text，完整 Markdown；start，起始标题；end，可选结束标题。
    输出：包含起始标题、不包含结束标题的 Markdown 片段。
    功能：按二级标题切分八股问答、面试话术和自查清单。
    """
    lines = text.splitlines()
    start_index = next(i for i, line in enumerate(lines) if heading_matches(line, start))
    end_index = len(lines)
    if end:
        end_index = next(
            (i for i, line in enumerate(lines) if heading_matches(line, end)),
            len(lines),
        )
    return "\n".join(lines[start_index:end_index]).strip() + "\n"


def build_notebook(doc_file: str) -> nbf.NotebookNode:
    """
    输入：doc_file，不含扩展名的主题名称。
    输出：包含两个 Markdown 单元格的 NotebookNode。
    功能：读取 Markdown 源文件并组装阅读型 notebook。
    """
    source = (SOURCE_DIR / f"{doc_file}.md").read_text(encoding="utf-8")
    title = source.splitlines()[0].lstrip("# ").strip()
    intro = extract_section(source, "## 八股问答", "## 面试话术")
    talk = extract_section(source, "## 面试话术", "## 自查清单")
    checklist = extract_section(source, "## 自查清单", None)
    return nbf.v4.new_notebook(
        cells=[
            nbf.v4.new_markdown_cell(f"# {title}\n\n{intro}"),
            nbf.v4.new_markdown_cell(f"{talk}\n\n{checklist}"),
        ]
    )


def main() -> None:
    """
    输入：命令行参数，当前脚本不支持参数。
    输出：生成 notebooks/ 下六份阅读型 .ipynb 文件并打印路径。
    功能：遍历全部 Redis 主题，生成无执行输出的 Markdown notebook。
    """
    NOTEBOOK_DIR.mkdir(parents=True, exist_ok=True)
    for doc_file in NOTEBOOKS:
        notebook = build_notebook(doc_file)
        target = NOTEBOOK_DIR / f"{doc_file}.ipynb"
        nbf.write(notebook, target)
        print(f"生成 {target.name}（{len(notebook.cells)} 个 Markdown 单元格）")


if __name__ == "__main__":
    main()
