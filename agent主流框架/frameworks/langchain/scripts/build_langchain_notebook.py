"""
脚本功能：
生成 LangChain 核心用法 Jupyter Notebook（带看懂版）：组装入门、写法对比、
核心能力三部分内容，写入 demo/LangChain使用方法.ipynb。

启动命令：
.venv/bin/python scripts/build_langchain_notebook.py
"""

from pathlib import Path

import nbformat
from nbformat.v4 import new_code_cell, new_markdown_cell, new_notebook

from cells_capabilities import build_capability_cells
from cells_examples import build_example_cells
from cells_intro import build_intro_cells


NOTEBOOK_PATH = Path(__file__).resolve().parent.parent / "demo" / "LangChain使用方法.ipynb"

# 内核约定：每个框架的 notebook 使用该框架 demo/.venv 注册的内核，
# 统一命名为 <framework>-demo。
KERNEL_NAME = "langchain-demo"
KERNEL_DISPLAY_NAME = "Python 3 (langchain)"


def to_cells(descriptions: list[tuple]) -> list:
    """
    输入：descriptions，("md"|"code", source[, tags]) 描述的列表。
    输出：nbformat cell 列表。
    功能：把内容模块的简单描述转换为 notebook cell。
    """
    cells = []
    for item in descriptions:
        kind, source = item[0], item[1]
        tags = list(item[2]) if len(item) > 2 else None
        if kind == "md":
            cells.append(new_markdown_cell(source))
        else:
            cell = new_code_cell(source)
            if tags:
                cell.metadata["tags"] = tags
            cells.append(cell)
    return cells


def main() -> None:
    """
    输入：无。
    输出：生成 demo/LangChain使用方法.ipynb 文件。
    功能：组装三部分内容并写入 notebook，内核固定为 langchain-demo。
    """
    descriptions = (
        build_intro_cells() + build_example_cells() + build_capability_cells()
    )
    notebook = new_notebook(
        cells=to_cells(descriptions),
        metadata={
            "kernelspec": {
                "display_name": KERNEL_DISPLAY_NAME,
                "language": "python",
                "name": KERNEL_NAME,
            },
            "language_info": {"name": "python", "version": "3.10"},
        },
    )
    nbformat.write(notebook, NOTEBOOK_PATH)
    print(f"notebook written: {NOTEBOOK_PATH}")


if __name__ == "__main__":
    main()
